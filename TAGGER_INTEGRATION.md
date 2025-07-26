# 🏷️ LLM Tagging System Integration Guide

This guide shows how to use the new intelligent tagging system in your Brain2 application.

## 🚀 Quick Start

### 1. Choose Your Tagger

You now have three options for generating tags:

```bash
# Option A: Keyword-based (default, always available)
export TAGGER_TYPE=keyword

# Option B: OpenAI API (requires API key)
export TAGGER_TYPE=openai
export OPENAI_API_KEY=sk-your-key-here
export OPENAI_MODEL=gpt-3.5-turbo

# Option C: Local LLM (best for privacy and cost)
export TAGGER_TYPE=local_llm
export TAGGER_SERVICE_URL=http://localhost:8000
```

### 2. Start the Tagger Service (if using local LLM)

```bash
cd tagger-service
./start.sh
```

Wait for the model to download and load (~5-10 minutes on first run).

### 3. Run Your Backend

```bash
cd backend
# The backend will automatically use your configured tagger
go run ./cmd/main
```

## 🔧 Configuration Options

### Environment Variables

| Variable | Description | Default | Example |
|----------|-------------|---------|---------|
| `TAGGER_TYPE` | Which tagger to use | `keyword` | `local_llm` |
| `TAGGER_SERVICE_URL` | Local LLM service URL | - | `http://localhost:8000` |
| `OPENAI_API_KEY` | OpenAI API key | - | `sk-...` |
| `OPENAI_MODEL` | OpenAI model to use | `gpt-3.5-turbo` | `gpt-4` |
| `TAGGER_MAX_TAGS` | Maximum tags per node | `5` | `10` |
| `TAGGER_FALLBACK` | Enable fallback to keyword tagger | `true` | `false` |

### Tagger Comparison

| Tagger | Pros | Cons | Best For |
|--------|------|------|----------|
| **Keyword** | Fast, reliable, no dependencies | Basic semantic understanding | Development, fallback |
| **OpenAI** | High quality, latest models | Costs money, requires internet | Production with budget |
| **Local LLM** | Private, no costs, good quality | Requires GPU/RAM, slower startup | Production, privacy-focused |

## 📋 API Changes

### Node Responses Now Include Tags

```json
{
  "nodeId": "abc-123",
  "content": "Learning about machine learning algorithms",
  "tags": ["technology", "learning", "algorithms"],  // ← NEW!
  "timestamp": "2025-01-25T10:30:00Z",
  "version": 1
}
```

### Tags vs Keywords

- **Keywords**: Used for finding connections between nodes (unchanged)
- **Tags**: Used for categorization and organization (new feature)

## 🔍 How It Works

### 1. Node Creation Flow

```
User creates node → Extract keywords → Generate tags → Store node → Find connections
                           ↓
                   [Keyword Tagger]
                   [OpenAI API] or  
                   [Local LLM]
```

### 2. Fallback Mechanism

```
Primary Tagger Fails → Fallback to Keyword Tagger → Continue normally
```

Your application never breaks - it gracefully degrades to keyword-based tagging.

### 3. Tag Generation Examples

**Input**: "I'm learning React for building user interfaces"

- **Keyword Tagger**: `["technology", "learning", "react"]`
- **OpenAI/LLM**: `["technology", "programming", "frontend", "learning", "react"]`

## 🐳 Docker Deployment

### Development Mode

```bash
# Just run the tagger service
docker-compose -f docker-compose.dev.yml up tagger-service

# Run Go backend locally
TAGGER_TYPE=local_llm TAGGER_SERVICE_URL=http://localhost:8000 go run ./cmd/main
```

### Full Stack

```bash
# Run everything together
docker-compose up
```

## 🧪 Testing

### Test Tagger Service Directly

```bash
# Health check
curl http://localhost:8000/health

# Generate tags
curl -X POST http://localhost:8000/generate-tags \
  -H "Content-Type: application/json" \
  -d '{"content": "Learning about serverless architecture patterns"}'
```

### Test Through Go Backend

```bash
# Create a node (tags will be generated automatically)
curl -X POST http://localhost:3000/api/nodes \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-jwt-token" \
  -d '{"content": "Exploring GraphQL for better API design"}'
```

## 🚨 Troubleshooting

### Tagger Service Won't Start

```bash
# Check system requirements
python3 --version  # Needs 3.8+
free -h            # Needs 6GB+ RAM

# Clear model cache and retry
rm -rf tagger-service/models/*
cd tagger-service && ./start.sh
```

### Go Backend Can't Connect

```bash
# Check if service is running
curl http://localhost:8000/health

# Check environment variables
echo $TAGGER_TYPE
echo $TAGGER_SERVICE_URL

# Check logs for connection errors
go run ./cmd/main 2>&1 | grep -i tagger
```

### Tags Not Appearing

1. **Check tagger type**: Ensure `TAGGER_TYPE` is set correctly
2. **Check fallback**: If primary tagger fails, it falls back to keyword tagger
3. **Check logs**: Look for warning messages about tag generation failures
4. **Verify API responses**: Use browser dev tools to check if tags are in responses

### Performance Issues

```bash
# Check if GPU is being used
nvidia-smi

# Monitor memory usage
htop

# Reduce model size (if needed)
# Edit tagger-service/app/main.py and use a smaller model
```

## 📊 Monitoring

### Health Checks

The system includes built-in health monitoring:

```bash
# Tagger service health
curl http://localhost:8000/health

# Go backend logs
# Watch for: "Warning: failed to generate tags"
```

### Performance Metrics

- **Model Loading**: 30-60 seconds (one-time per restart)
- **Tag Generation**: 1-3 seconds per request
- **Fallback Rate**: Should be <5% in production

## 🔒 Security & Privacy

### Local LLM Benefits

- No data sent to external APIs
- Complete control over the model
- No per-request costs
- Offline operation

### Best Practices

1. **Content Validation**: Already implemented - prevents XSS and oversized content
2. **Rate Limiting**: Consider adding for production
3. **Authentication**: Tagger service should be internal-only
4. **Model Updates**: Pin model versions for consistency

## 🎯 Next Steps

1. **Test with your content**: Try different types of notes to see tag quality
2. **Tune parameters**: Adjust `TAGGER_MAX_TAGS` based on your preferences  
3. **Monitor performance**: Watch resource usage and response times
4. **Scale if needed**: Add multiple tagger service instances for high load
5. **Custom models**: Consider fine-tuning models on your specific domain

---

🎉 **You're all set!** Your Brain2 application now has intelligent, context-aware tagging that makes your knowledge graph more organized and discoverable.