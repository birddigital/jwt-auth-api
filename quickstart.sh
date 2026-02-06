#!/bin/bash

# JWT Authentication API - Quick Start Script
# This script sets up and runs the JWT Auth API

set -e

echo "=========================================="
echo "JWT Authentication API - Quick Start"
echo "=========================================="
echo ""

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "Error: Go is not installed"
    echo "Please install Go from https://golang.org/dl/"
    exit 1
fi

echo "✓ Go version: $(go version)"
echo ""

# Check if jq is installed for examples
if ! command -v jq &> /dev/null; then
    echo "Warning: jq is not installed (required for examples.sh)"
    echo "Install with: brew install jq (macOS) or apt install jq (Linux)"
    echo ""
fi

# Install dependencies
echo "Installing dependencies..."
go mod download
echo "✓ Dependencies installed"
echo ""

# Run tests
echo "Running tests..."
if go test -v ./... > /dev/null 2>&1; then
    echo "✓ All tests passed"
else
    echo "⚠ Some tests failed (but continuing...)"
fi
echo ""

# Check if server is already running
if lsof -Pi :8080 -sTCP:LISTEN -t >/dev/null 2>&1 ; then
    echo "⚠ Port 8080 is already in use"
    echo "Please stop the existing server or change the port"
    echo ""
    read -p "Press Enter to exit..."
    exit 1
fi

# Set environment variables
export JWT_SECRET="dev-secret-key-change-in-production-32chars"
export REFRESH_SECRET="dev-refresh-key-change-in-production-32chars"
export PORT="8080"

echo "Starting server on http://localhost:8080"
echo ""
echo "Press Ctrl+C to stop the server"
echo ""
echo "=========================================="
echo "API Endpoints:"
echo "=========================================="
echo "POST   /api/v1/auth/login      - Login and get tokens"
echo "POST   /api/v1/auth/refresh    - Refresh access token"
echo "GET    /api/v1/protected       - Protected endpoint (requires auth)"
echo "GET    /health                 - Health check"
echo ""
echo "Test Users:"
echo "  user1  / password123"
echo "  admin  / admin456"
echo ""
echo "=========================================="
echo ""

# Start server
go run .
