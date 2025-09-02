#!/bin/bash
# Quick development script for rapid iteration

set -e

echo "🚀 Quick development build..."

# Quick build with unit tests only
./build.sh --quick --test-level unit

echo "✅ Quick build complete!"
echo "💡 Run './deploy.sh --quick' for quick deployment"