#!/bin/bash
# Force deployment script that ensures CDK picks up Lambda changes

set -e

echo "=========================================="
echo "🚀 FORCE DEPLOYMENT SCRIPT"
echo "=========================================="

# Step 1: Force rebuild backend
echo ""
echo "📦 Step 1/4: Force rebuilding backend..."
cd /home/wsl/b2/backend2
chmod +x build.sh
./build.sh

# Verify Lambda binaries were created
if [ -d "build/lambda" ] || [ -d "build/api" ]; then
    echo "✅ Lambda binaries built successfully"
else
    echo "❌ Lambda build directories not found!"
    exit 1
fi

# Step 2: Clear CDK caches
echo ""
echo "🧹 Step 2/4: Clearing CDK caches..."
cd /home/wsl/b2/infra
rm -rf cdk.out
rm -rf node_modules/.cache
echo "✅ CDK caches cleared"

# Step 3: Re-synthesize with fresh assets
echo ""
echo "🔄 Step 3/4: Re-synthesizing CDK app..."
npx cdk synth

# Verify the asset was created
if ls cdk.out/asset.*/bootstrap >/dev/null 2>&1; then
    echo "✅ Lambda assets created in cdk.out"
    
    # Check if any asset has our debug strings
    for asset in cdk.out/asset.*/bootstrap; do
        if [ -f "$asset" ]; then
            size=$(stat -c%s "$asset" 2>/dev/null || echo "0")
            # Main lambda is around 33-34MB
            if [ "$size" -gt 30000000 ] && [ "$size" -lt 40000000 ]; then
                echo "🔍 Checking asset: $asset (size: $(numfmt --to=iec $size))"
                if strings "$asset" | grep -q "DEBUG HANDLER"; then
                    echo "✅ Found debug logging in CDK asset!"
                else
                    echo "⚠️  Debug logging NOT in CDK asset - may need manual fix"
                fi
            fi
        fi
    done
else
    echo "❌ No Lambda assets found in cdk.out"
    exit 1
fi

# Step 4: Deploy with hotswap for faster Lambda updates
echo ""
echo "🚀 Step 4/4: Deploying to AWS..."
echo "Using hotswap for faster Lambda-only deployment..."

# Try hotswap first (faster)
if npx cdk deploy --all --hotswap --require-approval never; then
    echo "✅ Hotswap deployment successful!"
else
    echo "⚠️  Hotswap failed, trying regular deployment..."
    npx cdk deploy --all --require-approval never
fi

echo ""
echo "=========================================="
echo "✅ DEPLOYMENT COMPLETE!"
echo "=========================================="
echo ""
echo "Next steps:"
echo "1. Check CloudWatch logs for 'DEBUG HANDLER' messages"
echo "2. Create a new memory with title to test"
echo "3. Look for these log lines:"
echo "   - 'DEBUG: CreateNode handler called'"
echo "   - 'DEBUG HANDLER: Decoded request'"
echo "   - 'DEBUG HANDLER: Created command'"
echo ""
echo "If debug logs don't appear, try:"
echo "  aws lambda update-function-code --function-name <your-function> --zip-file fileb://backend2/build/lambda/bootstrap.zip"
echo "=========================================="