#!/bin/bash

# Run Backend2 Locally for Development
# This script starts the backend API server on port 8080

echo "🚀 Starting Backend2 API Server..."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# Set environment variables for local development
export ENVIRONMENT=development
export SERVER_ADDRESS=:8080
export AWS_REGION=us-east-1
export DYNAMODB_TABLE=brain2-backend
export EVENT_BUS_NAME=brain2-events
export LOG_LEVEL=debug
export JWT_SECRET=local-development-secret-key-change-in-production
export JWT_ISSUER=brain2-backend
export ENABLE_CORS=true
export ENABLE_METRICS=false
export ENABLE_TRACING=false
export IS_LAMBDA=false

# AWS credentials should be set via AWS CLI or environment
if [ -z "$AWS_ACCESS_KEY_ID" ]; then
    echo "⚠️  Warning: AWS credentials not set. DynamoDB operations will fail."
    echo "   Run 'aws configure' or set AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY"
fi

# Build the API server
echo "📦 Building backend..."
go build -o bin/api ./cmd/api

if [ $? -ne 0 ]; then
    echo "❌ Build failed!"
    exit 1
fi

echo "✅ Build successful!"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "🌐 Starting server on http://localhost:8080"
echo "📝 API endpoints available at http://localhost:8080/api/v2"
echo "❤️  Health check: http://localhost:8080/health"
echo ""
echo "Press Ctrl+C to stop the server"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# Run the server
./bin/api