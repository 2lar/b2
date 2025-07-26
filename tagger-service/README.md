# B2 LLM Tagger Service

An intelligent tagging service using Llama 3.1 8B for generating semantic tags from content.

## Features

- **Llama 3.1 8B Model**: Uses AWQ quantized model for efficient inference
- **Semantic Understanding**: Generates contextually relevant tags
- **Tag Memory**: Maintains and reuses existing tags for consistency
- **Health Monitoring**: Built-in health check endpoint
- **Docker Support**: Containerized for easy deployment

## Quick Start

### Option 1: Direct Python (Recommended for Development)

```bash
cd tagger-service
./start.sh
```

This will:
1. Create a virtual environment
2. Install dependencies
3. Download the model (~4GB on first run)
4. Start the service on port 8000

### Option 2: Docker

```bash
# Build and run with docker-compose
docker-compose -f docker-compose.dev.yml up tagger-service

# Or build manually
cd tagger-service
docker build -t b2-tagger .
docker run -p 8000:8000 -v $(pwd)/models:/app/models b2-tagger
```

### Option 3: Full Stack Development

```bash
# Run everything (backend + tagger + frontend)
docker-compose up
```

## Configuration

### Environment Variables

- `PORT`: Service port (default: 8000)
- `HF_HOME`: Hugging Face cache directory for models
- `MODEL_NAME`: Override the default model (advanced users)

### Go Backend Integration

Configure your Go backend to use the tagger service:

```bash
export TAGGER_TYPE=local_llm
export TAGGER_SERVICE_URL=http://localhost:8000
export TAGGER_FALLBACK=true
export TAGGER_MAX_TAGS=5
```

## API Endpoints

### Generate Tags

```bash
POST /generate-tags
Content-Type: application/json

{
  "content": "Your text content here"
}
```

Response:
```json
{
  "tags": ["technology", "learning", "productivity"]
}
```

### Health Check

```bash
GET /health
```

Response:
```json
{
  "status": "healthy",
  "model_loaded": true
}
```

## Model Information

- **Model**: `hugging-quants/Meta-Llama-3.1-8B-Instruct-AWQ-INT4`
- **Size**: ~4GB download
- **Quantization**: AWQ INT4 for efficiency
- **GPU Support**: Automatically detects and uses GPU if available

## Performance Notes

- **First Run**: Model download takes 5-10 minutes depending on connection
- **Cold Start**: ~30-60 seconds for model loading
- **Inference**: ~1-3 seconds per request (depends on hardware)
- **Memory**: ~6-8GB RAM recommended, 4GB minimum

## Troubleshooting

### Model Download Issues
```bash
# Clear cache and retry
rm -rf models/*
./start.sh
```

### Memory Issues
- Reduce batch size in config
- Ensure sufficient RAM (6GB+)
- Use CPU-only mode if GPU memory is limited

### Connection Issues
```bash
# Test health endpoint
curl http://localhost:8000/health

# Test tagging
curl -X POST http://localhost:8000/generate-tags \
  -H "Content-Type: application/json" \
  -d '{"content": "test content"}'
```

## Development

### Local Development Setup

1. Clone and setup:
   ```bash
   cd tagger-service
   python3 -m venv venv
   source venv/bin/activate
   pip install -r requirements.txt
   ```

2. Run with auto-reload:
   ```bash
   uvicorn app.main:app --reload --port 8000
   ```

3. Test integration:
   ```bash
   # In another terminal, run Go backend
   cd ../backend
   TAGGER_TYPE=local_llm TAGGER_SERVICE_URL=http://localhost:8000 go run ./cmd/main
   ```

### Testing with Go Backend

The service is designed to work seamlessly with the Go backend. The request/response format matches exactly:

- Go sends: `{"content": "..."}`
- Python responds: `{"tags": ["tag1", "tag2"]}`

## Production Deployment

For production, consider:

1. **Resource Allocation**: 8GB+ RAM, GPU recommended
2. **Load Balancing**: Multiple service instances behind a load balancer
3. **Model Caching**: Persistent volume for model cache
4. **Monitoring**: Health checks and metrics collection
5. **Security**: API authentication and rate limiting