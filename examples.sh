#!/bin/bash

# JWT Authentication API - Example Usage
# This script demonstrates how to use the JWT Auth API

set -e

API_URL="http://localhost:8080"

echo "=========================================="
echo "JWT Authentication API - Example Usage"
echo "=========================================="
echo ""

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 1. Health Check
echo -e "${BLUE}1. Health Check${NC}"
curl -s "${API_URL}/health" | jq .
echo ""
echo ""

# 2. Login with valid credentials
echo -e "${BLUE}2. Login (Valid Credentials)${NC}"
echo "Username: user1"
echo "Password: password123"
LOGIN_RESPONSE=$(curl -s -X POST "${API_URL}/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"username":"user1","password":"password123"}')
echo "$LOGIN_RESPONSE" | jq .

# Extract tokens
ACCESS_TOKEN=$(echo "$LOGIN_RESPONSE" | jq -r '.access_token')
REFRESH_TOKEN=$(echo "$LOGIN_RESPONSE" | jq -r '.refresh_token')

echo ""
echo -e "${GREEN}✓ Login successful!${NC}"
echo "Access Token: ${ACCESS_TOKEN:0:50}..."
echo "Refresh Token: ${REFRESH_TOKEN:0:50}..."
echo ""
echo ""

# 3. Access protected endpoint with valid token
echo -e "${BLUE}3. Access Protected Endpoint (Valid Token)${NC}"
curl -s -X GET "${API_URL}/api/v1/protected" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" | jq .
echo ""
echo ""

# 4. Test with invalid token
echo -e "${BLUE}4. Access Protected Endpoint (Invalid Token)${NC}"
curl -s -X GET "${API_URL}/api/v1/protected" \
  -H "Authorization: Bearer invalid.token.here" | jq .
echo ""
echo ""

# 5. Login with invalid credentials
echo -e "${BLUE}5. Login (Invalid Credentials)${NC}"
echo "Username: user1"
echo "Password: wrongpassword"
curl -s -X POST "${API_URL}/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"username":"user1","password":"wrongpassword"}' | jq .
echo ""
echo ""

# 6. Refresh token
echo -e "${BLUE}6. Refresh Token${NC}"
REFRESH_RESPONSE=$(curl -s -X POST "${API_URL}/api/v1/auth/refresh" \
  -H "Content-Type: application/json" \
  -d "{\"refresh_token\":\"${REFRESH_TOKEN}\"}")
echo "$REFRESH_RESPONSE" | jq .

NEW_ACCESS_TOKEN=$(echo "$REFRESH_RESPONSE" | jq -r '.access_token')
echo ""
echo -e "${GREEN}✓ Token refreshed!${NC}"
echo "New Access Token: ${NEW_ACCESS_TOKEN:0:50}..."
echo ""
echo ""

# 7. Validation errors
echo -e "${BLUE}7. Login Validation Errors${NC}"

echo "Missing username:"
curl -s -X POST "${API_URL}/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"password":"password123"}' | jq .
echo ""

echo "Short password (less than 6 chars):"
curl -s -X POST "${API_URL}/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"username":"user1","password":"12345"}' | jq .
echo ""
echo ""

# 8. Test with admin user
echo -e "${BLUE}8. Login as Admin${NC}"
echo "Username: admin"
echo "Password: admin456"
curl -s -X POST "${API_URL}/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin456"}' | jq .
echo ""
echo ""

echo "=========================================="
echo -e "${GREEN}All examples completed!${NC}"
echo "=========================================="
