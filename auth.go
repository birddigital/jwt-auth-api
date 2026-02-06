package main

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const (
	tokenExpiration    = 24 * time.Hour
	refreshExpiration  = 7 * 24 * time.Hour
	bearerTokenPrefix  = "Bearer "
	contentTypeJSON    = "application/json"
	contentTypeForm    = "application/x-www-form-urlencoded"
)

var (
	jwtSecret     []byte
	refreshSecret []byte
)

// User represents a user in the system
type User struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Password string `json:"-"` // Never expose in JSON
	Email    string `json:"email,omitempty"`
	Active   bool   `json:"active"`
}

// MockUserStore implements a simple in-memory user store
// In production, replace with database-backed implementation
type MockUserStore struct {
	users map[string]*User
}

// NewMockUserStore creates a new mock user store with sample users
func NewMockUserStore() *MockUserStore {
	store := &MockUserStore{
		users: make(map[string]*User),
	}

	// Hash sample passwords
	password1, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	password2, _ := bcrypt.GenerateFromPassword([]byte("admin456"), bcrypt.DefaultCost)

	// Add sample users
	store.users["user1"] = &User{
		ID:       "usr_1a2b3c4d",
		Username: "user1",
		Password: string(password1),
		Email:    "user1@example.com",
		Active:   true,
	}

	store.users["admin"] = &User{
		ID:       "usr_admin001",
		Username: "admin",
		Password: string(password2),
		Email:    "admin@example.com",
		Active:   true,
	}

	return store
}

// FindByUsername retrieves a user by username
func (s *MockUserStore) FindByUsername(username string) (*User, error) {
	user, exists := s.users[username]
	if !exists {
		return nil, errors.New("user not found")
	}
	return user, nil
}

// ValidateCredentials checks if the provided password matches the stored hash
func (s *MockUserStore) ValidateCredentials(user *User, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	return err == nil
}

// Request/Response DTOs

// LoginRequest represents the login request payload
type LoginRequest struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
}

// LoginResponse represents the successful login response
type LoginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	User         UserInfo `json:"user"`
}

// UserInfo represents safe user information to return
type UserInfo struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email,omitempty"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
	Code    string `json:"code,omitempty"`
}

// RefreshTokenRequest represents a refresh token request
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// Claims represents JWT claims
type Claims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// JWTManager handles JWT token generation and validation
type JWTManager struct {
	secretKey     []byte
	refreshSecret []byte
	issuer        string
}

// NewJWTManager creates a new JWT manager
func NewJWTManager(secretKey, refreshSecret, issuer string) *JWTManager {
	return &JWTManager{
		secretKey:     []byte(secretKey),
		refreshSecret: []byte(refreshSecret),
		issuer:        issuer,
	}
}

// GenerateTokens generates both access and refresh tokens
func (m *JWTManager) GenerateTokens(user *User) (string, string, error) {
	// Generate access token
	accessToken, err := m.GenerateAccessToken(user)
	if err != nil {
		return "", "", err
	}

	// Generate refresh token
	refreshToken, err := m.GenerateRefreshToken(user)
	if err != nil {
		return "", "", err
	}

	return accessToken, refreshToken, nil
}

// GenerateAccessToken generates a new JWT access token
func (m *JWTManager) GenerateAccessToken(user *User) (string, error) {
	now := time.Now()
	expiresAt := now.Add(tokenExpiration)

	claims := &Claims{
		UserID:   user.ID,
		Username: user.Username,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   user.ID,
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secretKey)
}

// GenerateRefreshToken generates a refresh token with longer expiration
func (m *JWTManager) GenerateRefreshToken(user *User) (string, error) {
	now := time.Now()
	expiresAt := now.Add(refreshExpiration)

	claims := &Claims{
		UserID:   user.ID,
		Username: user.Username,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer + "/refresh",
			Subject:   user.ID,
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.refreshSecret)
}

// ValidateAccessToken validates an access token and returns the claims
func (m *JWTManager) ValidateAccessToken(tokenString string) (*Claims, error) {
	return m.validateToken(tokenString, m.secretKey)
}

// ValidateRefreshToken validates a refresh token and returns the claims
func (m *JWTManager) ValidateRefreshToken(tokenString string) (*Claims, error) {
	return m.validateToken(tokenString, m.refreshSecret)
}

func (m *JWTManager) validateToken(tokenString string, secret []byte) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return secret, nil
	})

	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}

// AuthHandler handles authentication requests
type AuthHandler struct {
	userStore   *MockUserStore
	jwtManager  *JWTManager
}

// NewAuthHandler creates a new authentication handler
func NewAuthHandler(userStore *MockUserStore, jwtManager *JWTManager) *AuthHandler {
	return &AuthHandler{
		userStore:  userStore,
		jwtManager: jwtManager,
	}
}

// Login handles user authentication
// @Summary Login
// @Description Authenticate user with username and password
// @Tags auth
// @Accept json
// @Produce json
// @Param request body LoginRequest true "Login credentials"
// @Success 200 {object} LoginResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Router /api/v1/auth/login [post]
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	// Validate content type
	if !isValidContentType(r.Header.Get("Content-Type")) {
		respondWithError(w, http.StatusUnsupportedMediaType, "unsupported media type", "invalid_content_type")
		return
	}

	// Parse request
	var req LoginRequest
	if err := parseJSON(r, &req); err != nil {
		respondWithError(w, http.StatusBadRequest, "invalid request body: "+err.Error(), "invalid_request")
		return
	}

	// Validate required fields
	if err := validateLoginRequest(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error(), "validation_error")
		return
	}

	// Find user
	user, err := h.userStore.FindByUsername(req.Username)
	if err != nil {
		// Use constant-time comparison to prevent timing attacks
		subtle.ConstantTimeCompare([]byte(req.Password), []byte("dummy_password_for_timing"))
		respondWithError(w, http.StatusUnauthorized, "invalid credentials", "invalid_credentials")
		return
	}

	// Check if user is active
	if !user.Active {
		respondWithError(w, http.StatusForbidden, "user account is disabled", "account_disabled")
		return
	}

	// Validate credentials
	if !h.userStore.ValidateCredentials(user, req.Password) {
		respondWithError(w, http.StatusUnauthorized, "invalid credentials", "invalid_credentials")
		return
	}

	// Generate tokens
	accessToken, refreshToken, err := h.jwtManager.GenerateTokens(user)
	if err != nil {
		log.Printf("Error generating tokens: %v", err)
		respondWithError(w, http.StatusInternalServerError, "failed to generate tokens", "token_generation_failed")
		return
	}

	// Log successful login (in production, use proper logging)
	log.Printf("User logged in: %s (ID: %s)", user.Username, user.ID)

	// Respond with tokens
	respondWithJSON(w, http.StatusOK, LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int64(tokenExpiration.Seconds()),
		User: UserInfo{
			ID:       user.ID,
			Username: user.Username,
			Email:    user.Email,
		},
	})
}

// RefreshToken handles token refresh
// @Summary Refresh Token
// @Description Refresh access token using refresh token
// @Tags auth
// @Accept json
// @Produce json
// @Param request body RefreshTokenRequest true "Refresh token"
// @Success 200 {object} LoginResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Router /api/v1/auth/refresh [post]
func (h *AuthHandler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	// Validate content type
	if !isValidContentType(r.Header.Get("Content-Type")) {
		respondWithError(w, http.StatusUnsupportedMediaType, "unsupported media type", "invalid_content_type")
		return
	}

	// Parse request
	var req RefreshTokenRequest
	if err := parseJSON(r, &req); err != nil {
		respondWithError(w, http.StatusBadRequest, "invalid request body: "+err.Error(), "invalid_request")
		return
	}

	// Validate refresh token
	if req.RefreshToken == "" {
		respondWithError(w, http.StatusBadRequest, "refresh_token is required", "missing_refresh_token")
		return
	}

	// Validate and parse refresh token
	claims, err := h.jwtManager.ValidateRefreshToken(req.RefreshToken)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "invalid or expired refresh token", "invalid_refresh_token")
		return
	}

	// Get user from claims
	user, err := h.userStore.FindByUsername(claims.Username)
	if err != nil || !user.Active {
		respondWithError(w, http.StatusUnauthorized, "user not found or inactive", "user_not_found")
		return
	}

	// Generate new tokens
	accessToken, refreshToken, err := h.jwtManager.GenerateTokens(user)
	if err != nil {
		log.Printf("Error generating tokens: %v", err)
		respondWithError(w, http.StatusInternalServerError, "failed to generate tokens", "token_generation_failed")
		return
	}

	// Respond with new tokens
	respondWithJSON(w, http.StatusOK, LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int64(tokenExpiration.Seconds()),
		User: UserInfo{
			ID:       user.ID,
			Username: user.Username,
			Email:    user.Email,
		},
	})
}

// ProtectedEndpoint is an example of a protected endpoint
func (h *AuthHandler) ProtectedEndpoint(w http.ResponseWriter, r *http.Request) {
	// Get user from context (set by auth middleware)
	user, ok := r.Context().Value("user").(*Claims)
	if !ok {
		respondWithError(w, http.StatusInternalServerError, "failed to get user context", "context_error")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Access granted to protected resource",
		"user": map[string]string{
			"id":       user.UserID,
			"username": user.Username,
		},
	})
}

// AuthMiddleware validates JWT tokens and adds user info to request context
func (h *AuthHandler) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract token from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			respondWithError(w, http.StatusUnauthorized, "missing authorization header", "missing_auth_header")
			return
		}

		// Check Bearer prefix
		if !strings.HasPrefix(authHeader, bearerTokenPrefix) {
			respondWithError(w, http.StatusUnauthorized, "invalid authorization header format", "invalid_auth_format")
			return
		}

		// Extract token
		tokenString := strings.TrimPrefix(authHeader, bearerTokenPrefix)

		// Validate token
		claims, err := h.jwtManager.ValidateAccessToken(tokenString)
		if err != nil {
			respondWithError(w, http.StatusUnauthorized, "invalid or expired token", "invalid_token")
			return
		}

		// Add user info to context
		ctx := r.Context()
		ctx = contextWithUser(ctx, claims)
		r = r.WithContext(ctx)

		// Call next handler
		next.ServeHTTP(w, r)
	})
}

// Helper functions

type contextKey string

const userKey contextKey = "user"

func contextWithUser(ctx context.Context, user *Claims) context.Context {
	return context.WithValue(ctx, userKey, user)
}

func isValidContentType(contentType string) bool {
	return strings.Contains(contentType, contentTypeJSON) ||
		strings.Contains(contentType, contentTypeForm)
}

func validateLoginRequest(req *LoginRequest) error {
	if req.Username == "" {
		return errors.New("username is required")
	}
	if req.Password == "" {
		return errors.New("password is required")
	}
	if len(req.Username) < 3 {
		return errors.New("username must be at least 3 characters")
	}
	if len(req.Password) < 6 {
		return errors.New("password must be at least 6 characters")
	}
	return nil
}

func respondWithJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", contentTypeJSON)
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Error encoding JSON response: %v", err)
	}
}

func respondWithError(w http.ResponseWriter, statusCode int, message string, code string) {
	respondWithJSON(w, statusCode, ErrorResponse{
		Error:   http.StatusText(statusCode),
		Message: message,
		Code:    code,
	})
}

func parseJSON(r *http.Request, v interface{}) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(v)
}

// generateSecretKey generates a cryptographically secure random key
func generateSecretKey(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// init initializes secrets from environment or generates new ones
func init() {
	// Try to get from environment first
	if secret := os.Getenv("JWT_SECRET"); secret != "" {
		jwtSecret = []byte(secret)
	} else {
		// Generate a random secret (in production, ALWAYS set via environment)
		generated, err := generateSecretKey(32)
		if err != nil {
			log.Fatal("Failed to generate JWT secret")
		}
		jwtSecret = []byte(generated)
		log.Printf("WARNING: Using auto-generated JWT secret. Set JWT_SECRET environment variable in production!")
	}

	if secret := os.Getenv("REFRESH_SECRET"); secret != "" {
		refreshSecret = []byte(secret)
	} else {
		// Generate a random secret for refresh tokens
		generated, err := generateSecretKey(32)
		if err != nil {
			log.Fatal("Failed to generate refresh secret")
		}
		refreshSecret = []byte(generated)
		log.Printf("WARNING: Using auto-generated refresh secret. Set REFRESH_SECRET environment variable in production!")
	}
}

func main() {
	// Initialize components
	userStore := NewMockUserStore()
	jwtManager := NewJWTManager(string(jwtSecret), string(refreshSecret), "jwt-auth-api")
	authHandler := NewAuthHandler(userStore, jwtManager)

	// Setup router
	r := chi.NewRouter()

	// Middleware
	r.Use(middlewareRequestID)
	r.Use(middlewareLogger)
	r.Use(middlewareRecover)
	r.Use(middlewareSecure)
	r.Use(middlewareCORS)

	// Public routes
	r.Post("/api/v1/auth/login", authHandler.Login)
	r.Post("/api/v1/auth/refresh", authHandler.RefreshToken)

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(authHandler.AuthMiddleware)
		r.Get("/api/v1/protected", authHandler.ProtectedEndpoint)
	})

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		respondWithJSON(w, http.StatusOK, map[string]string{
			"status":  "healthy",
			"service": "jwt-auth-api",
			"version": "1.0.0",
		})
	})

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	addr := ":" + port
	log.Printf("Starting JWT Auth API on %s", addr)
	log.Printf("Login endpoint: http://localhost%s/api/v1/auth/login", addr)
	log.Printf("Health check: http://localhost%s/health", addr)

	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
