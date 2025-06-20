#!/bin/bash
set -e

echo "🧹 Cleaning Lambda Authorizer build artifacts..."

# Remove build artifacts
rm -rf dist/
rm -rf node_modules/
rm -f *.js
rm -f *.js.map

echo "✅ Lambda Authorizer cleaned successfully!"