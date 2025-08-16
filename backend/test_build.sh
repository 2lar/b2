#\!/bin/bash
set -e
echo 'Testing build without tests...'
echo 'Installing dependencies...'
go mod tidy
echo 'Building Lambda functions...'
apps=$(ls -d cmd/*/ | xargs -n 1 basename)
for app in $apps
do
    echo "--- Building $app ---"
    SRC_PATH="./cmd/$app"
    OUTPUT_PATH="./build/$app"
    mkdir -p "$OUTPUT_PATH"
    GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o "$OUTPUT_PATH/bootstrap" "$SRC_PATH"
    chmod +x "$OUTPUT_PATH/bootstrap"
    echo "✅ Successfully built $app"
done
echo '🎉 All Lambda functions built successfully\!'
