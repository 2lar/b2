#!/bin/bash

# Tagger Service Startup Script
# This script sets up and runs the LLM tagger service

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

print_status() {
    echo -e "${BLUE}[TAGGER]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if Python is available
if ! command -v python3 &> /dev/null; then
    print_error "Python 3 is required but not installed"
    exit 1
fi

print_status "Starting LLM Tagger Service..."
print_status "======================================"

# Check if virtual environment exists
if [ ! -d "venv" ]; then
    print_status "Creating Python virtual environment..."
    python3 -m venv venv
fi

# Activate virtual environment
print_status "Activating virtual environment..."
source venv/bin/activate

# Install dependencies
print_status "Installing dependencies..."
pip install -r requirements.txt

# Create models directory if it doesn't exist
mkdir -p models

# Set default environment variables
export PORT=${PORT:-8000}
export HF_HOME=${HF_HOME:-"./models"}

print_status "Configuration:"
print_status "  Port: $PORT"
print_status "  Models cache: $HF_HOME"

print_success "Starting tagger service..."
print_warning "First run will download the model (~4GB) - this may take several minutes"

# Start the service
uvicorn app.main:app --host 0.0.0.0 --port $PORT --reload