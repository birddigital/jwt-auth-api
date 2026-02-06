package main

import (
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

// middlewareRequestID adds a unique request ID to each request
func middlewareRequestID(next http.Handler) http.Handler {
	return middleware.RequestID(next)
}

// middlewareLogger logs HTTP requests
func middlewareLogger(next http.Handler) http.Handler {
	return middleware.Logger(next)
}

// middlewareRecover recovers from panics
func middlewareRecover(next http.Handler) http.Handler {
	return middleware.Recoverer(next)
}

// middlewareSecure adds security headers
func middlewareSecure(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Prevent content type sniffing
		w.Header().Set("X-Content-Type-Options", "nosniff")

		// Prevent clickjacking
		w.Header().Set("X-Frame-Options", "DENY")

		// Enable XSS filter
		w.Header().Set("X-XSS-Protection", "1; mode=block")

		// Strict transport security (only on HTTPS)
		if r.URL.Scheme == "https" {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		// Content security policy (adjust for your needs)
		w.Header().Set("Content-Security-Policy", "default-src 'self'")

		next.ServeHTTP(w, r)
	})
}

// middlewareCORS handles CORS headers
func middlewareCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Allow requests from any origin (customize in production)
		w.Header().Set("Access-Control-Allow-Origin", "*")

		// Allow common methods
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")

		// Allow common headers
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Allow credentials
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		// Max age for preflight requests
		w.Header().Set("Access-Control-Max-Age", "86400")

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// middlewareTimeout adds a timeout to requests
func middlewareTimeout(timeout time.Duration) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Wrap the ResponseWriter to track if timeout occurred
			wrapped := &responseWriter{ResponseWriter: w, timeout: timeout}

			// Create channel to monitor request completion
			done := make(chan bool, 1)

			// Run request in goroutine
			go func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("Panic in request handler: %v", r)
						http.Error(w, "Internal Server Error", http.StatusInternalServerError)
						done <- true
					}
				}()

				next.ServeHTTP(wrapped, r)
				done <- true
			}()

			// Wait for request completion or timeout
			select {
			case <-done:
				// Request completed normally
				if !wrapped.wroteHeader {
					wrapped.WriteHeader(http.StatusGatewayTimeout)
				}
			case <-time.After(timeout):
				// Timeout occurred
				wroteHeader := wrapped.wroteHeader
				if !wroteHeader {
					wrapped.WriteHeader(http.StatusGatewayTimeout)
				}
				log.Printf("Request timeout: %s %s", r.Method, r.URL.Path)
			}
		})
	}
}

// responseWriter wraps http.ResponseWriter to track header writes
type responseWriter struct {
	http.ResponseWriter
	wroteHeader bool
	timeout     time.Duration
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.wroteHeader {
		rw.wroteHeader = true
		rw.ResponseWriter.WriteHeader(code)
	}
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.ResponseWriter.Write(b)
}
