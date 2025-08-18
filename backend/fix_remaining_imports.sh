#!/bin/bash

# Script to fix remaining import issues

echo "Fixing remaining NodeID type issues..."

# Fix NodeID references that should be shared.NodeID
sed -i 's/node\.NodeID/shared.NodeID/g' internal/repository/read_write_separation.go

# Fix any remaining domain references that got missed
find internal/ -name "*.go" -exec sed -i 's/\bdomain\./shared./g' {} \;

echo "Fixed remaining import issues"