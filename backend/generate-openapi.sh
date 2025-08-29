#!/bin/bash
# This script generates OpenAPI specification from code annotations using swaggo/swag.
#
# USAGE EXAMPLES:
# ===============
# Generate OpenAPI spec:
#   ./generate-openapi.sh
#
# Generate and validate spec:
#   ./generate-openapi.sh --validate
#
# Generate spec with custom output directory:
#   ./generate-openapi.sh --output ./custom/path

# Exit immediately if a command exits with a non-zero status.
set -e

# Parse command line arguments
VALIDATE=false
OUTPUT_DIR="pkg/api"
while [[ $# -gt 0 ]]; do
    case $1 in
        --validate)
            VALIDATE=true
            shift
            ;;
        --output)
            OUTPUT_DIR="$2"
            shift 2
            ;;
        --help|-h)
            echo "Usage: $0 [--validate] [--output <directory>]"
            echo ""
            echo "Options:"
            echo "  --validate     Validate the generated OpenAPI spec"
            echo "  --output <dir> Output directory for generated files (default: pkg/api)"
            echo "  --help, -h     Show this help message"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

echo "🔄 Generating OpenAPI specification from code annotations..."

# Ensure swag command is available
if ! command -v swag &> /dev/null; then
    echo "❌ swag command not found. Installing swaggo/swag..."
    go install github.com/swaggo/swag/cmd/swag@latest
    
    # Add GOBIN to PATH if it's not already there
    if [[ ":$PATH:" != *":$(go env GOPATH)/bin:"* ]]; then
        export PATH="$(go env GOPATH)/bin:$PATH"
    fi
fi

# Create output directory if it doesn't exist
mkdir -p "$OUTPUT_DIR"

# Generate OpenAPI specification
echo "📝 Running swag init to generate OpenAPI spec..."
swag init \
    --generalInfo "./cmd/main/main.go" \
    --dir "./" \
    --output "$OUTPUT_DIR" \
    --outputTypes "go,json,yaml" \
    --parseDependency \
    --parseInternal

if [ $? -ne 0 ]; then
    echo "❌ Failed to generate OpenAPI specification"
    exit 1
fi

echo "✅ OpenAPI specification generated successfully!"
echo "   • Go docs: $OUTPUT_DIR/docs.go"
echo "   • JSON spec: $OUTPUT_DIR/swagger.json"
echo "   • YAML spec: $OUTPUT_DIR/swagger.yaml"

# Validate the generated specification if requested
if [ "$VALIDATE" = true ]; then
    echo ""
    echo "🔍 Validating generated OpenAPI specification..."
    
    # Check if spectral is available for validation
    if command -v spectral &> /dev/null; then
        echo "Using Spectral for OpenAPI validation..."
        spectral lint "$OUTPUT_DIR/swagger.yaml"
    elif command -v swagger-codegen &> /dev/null; then
        echo "Using swagger-codegen for validation..."
        swagger-codegen validate -i "$OUTPUT_DIR/swagger.yaml"
    elif command -v openapi-generator &> /dev/null; then
        echo "Using openapi-generator for validation..."
        openapi-generator validate -i "$OUTPUT_DIR/swagger.yaml"
    else
        echo "⚠️  No OpenAPI validation tool found (spectral, swagger-codegen, or openapi-generator)"
        echo "   Performing basic validation checks..."
        
        # Basic validation - check if files exist and are not empty
        if [ ! -s "$OUTPUT_DIR/swagger.yaml" ]; then
            echo "❌ Generated YAML spec is empty or missing"
            exit 1
        fi
        
        if [ ! -s "$OUTPUT_DIR/swagger.json" ]; then
            echo "❌ Generated JSON spec is empty or missing"
            exit 1
        fi
        
        # Check if YAML is valid by attempting to parse it
        if command -v python3 &> /dev/null; then
            python3 -c "import yaml; yaml.safe_load(open('$OUTPUT_DIR/swagger.yaml'))" 2>/dev/null
            if [ $? -eq 0 ]; then
                echo "✅ YAML specification is valid"
            else
                echo "❌ Generated YAML specification is invalid"
                exit 1
            fi
        fi
        
        echo "✅ Basic validation passed"
    fi
fi

# Show statistics about the generated spec
echo ""
echo "📊 OpenAPI Specification Statistics:"
if [ -f "$OUTPUT_DIR/swagger.yaml" ]; then
    # Count paths
    paths_count=$(grep -c "^  /.*:" "$OUTPUT_DIR/swagger.yaml" 2>/dev/null || echo "0")
    echo "   • API Paths: $paths_count"
    
    # Count operations (methods)
    operations_count=$(grep -c "^    get:\|^    post:\|^    put:\|^    delete:\|^    patch:" "$OUTPUT_DIR/swagger.yaml" 2>/dev/null || echo "0")
    echo "   • Operations: $operations_count"
    
    # Count schemas/models
    schemas_count=$(grep -c "^    [A-Z][A-Za-z]*:" "$OUTPUT_DIR/swagger.yaml" 2>/dev/null | head -1 || echo "0")
    echo "   • Models: $schemas_count"
    
    # File sizes
    yaml_size=$(stat -c%s "$OUTPUT_DIR/swagger.yaml" 2>/dev/null | numfmt --to=iec 2>/dev/null || echo "unknown")
    json_size=$(stat -c%s "$OUTPUT_DIR/swagger.json" 2>/dev/null | numfmt --to=iec 2>/dev/null || echo "unknown")
    echo "   • YAML size: $yaml_size"
    echo "   • JSON size: $json_size"
fi

echo ""
echo "🎉 OpenAPI generation completed successfully!"
echo "💡 Tip: Use './generate-openapi.sh --validate' to validate the specification"
echo "💡 Tip: The generated spec is embedded in your Go application via //go:embed"