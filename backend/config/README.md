# Brain2 Configuration

This directory contains configuration files for the Brain2 backend application.

## 📁 File Structure

```
config/
├── base.yaml           # Base configuration (all environments inherit from this)
├── development.yaml    # Development environment overrides
├── staging.yaml        # Staging environment overrides
├── production.yaml     # Production environment overrides
├── example.yaml        # Fully documented example with all available options
├── local.yaml.example  # Template for local development overrides
└── local.yaml          # Your personal local overrides (gitignored)
```

## 🚀 Quick Start

### For Development

1. The application will automatically use `development.yaml` configuration
2. To override settings locally, copy `local.yaml.example` to `local.yaml`:
   ```bash
   cp config/local.yaml.example config/local.yaml
   ```
3. Edit `local.yaml` with your personal preferences (this file is gitignored)

### For Production

1. Set the environment variable:
   ```bash
   export ENVIRONMENT=production
   ```
2. Provide required secrets via environment variables:
   ```bash
   export JWT_SECRET=your-secret-key
   export REDIS_PASSWORD=your-redis-password
   ```

## 📊 Configuration Hierarchy

Configuration is loaded in this order (later sources override earlier ones):

1. **Default values** (hardcoded in application)
2. **base.yaml** (common settings)
3. **{environment}.yaml** (environment-specific)
4. **local.yaml** (personal overrides, development only)
5. **Environment variables** (highest priority)

## 🔧 Environment Variables

Any configuration value can be overridden using environment variables:

| Config Path | Environment Variable | Description |
|------------|---------------------|-------------|
| `server.port` | `SERVER_PORT` | HTTP server port |
| `database.table_name` | `DATABASE_TABLE_NAME` | DynamoDB table name |
| `features.enable_metrics` | `FEATURES_ENABLE_METRICS` | Enable metrics collection |
| `security.jwt_secret` | `SECURITY_JWT_SECRET` | JWT signing secret |
| `infrastructure.idempotency_ttl` | `IDEMPOTENCY_TTL` | Idempotency record retention (e.g., "24h", "7d") |

### Naming Convention
- Replace dots (.) with underscores (_)
- Convert to UPPERCASE
- For nested values, join with underscores

## 🌍 Environments

### Development
- Debug logging enabled
- Authentication disabled for easier testing
- All experimental features enabled
- Connects to local services

### Staging
- Production-like configuration
- Moderate resource allocation
- Full monitoring enabled
- Test experimental features

### Production
- Strict security settings
- High resource allocation
- Required monitoring and metrics
- No debug features

## 🔐 Security

### Secrets Management

**Never commit secrets to version control!**

Use placeholders in YAML files:
```yaml
security:
  jwt_secret: "${JWT_SECRET}"
```

Provide actual values via:
- Environment variables
- AWS Secrets Manager
- HashiCorp Vault
- Kubernetes Secrets

### Required Production Settings
- `security.enable_auth: true`
- `security.secure_headers: true`
- `features.enable_metrics: true`
- `features.enable_circuit_breaker: true`

## ⏰ Idempotency TTL Configuration

The idempotency TTL controls how long duplicate request prevention records are retained in DynamoDB.

### Configuration Options

1. **Via Configuration File** (base.yaml, development.yaml, etc.):
   ```yaml
   infrastructure:
     idempotency_ttl: 24h  # Default: 24 hours
   ```

2. **Via Environment Variable**:
   ```bash
   export IDEMPOTENCY_TTL=7d    # 7 days
   export IDEMPOTENCY_TTL=1h    # 1 hour
   export IDEMPOTENCY_TTL=48h   # 48 hours
   ```

### TTL Guidelines

| Duration | Use Case | Storage Impact |
|----------|----------|----------------|
| **1-6 hours** | High-volume transient operations, rapid request processing | Minimal storage, lower costs |
| **24 hours** (default) | Standard APIs, balanced duplicate prevention | Moderate storage |
| **2-7 days** | Critical operations, payment processing, important state changes | Higher storage, better protection |

### Important Notes

- **Valid Range**: 1h (minimum) to 168h/7d (maximum)
- **DynamoDB TTL**: Records expire at the specified time but may persist up to 48 hours before deletion
- **Query Behavior**: Expired records are filtered from queries immediately after TTL passes
- **Cost Consideration**: Longer TTLs increase storage costs but provide better duplicate prevention

### Example Configurations

```bash
# Short TTL for high-volume API
IDEMPOTENCY_TTL=2h

# Standard daily operations
IDEMPOTENCY_TTL=24h

# Critical financial transactions
IDEMPOTENCY_TTL=7d
```

## 📝 Configuration Examples

### Minimal Development Setup
```yaml
# local.yaml
server:
  port: 3000

logging:
  level: debug
  
features:
  enable_debug_endpoints: true
```

### Production with Redis Cache
```yaml
# Via environment variables
export CACHE_PROVIDER=redis
export CACHE_REDIS_HOST=redis.example.com
export CACHE_REDIS_PASSWORD=secret
```

### Enable All Features
```yaml
features:
  enable_caching: true
  enable_auto_connect: true
  enable_ai_processing: true
  enable_metrics: true
  enable_tracing: true
  enable_event_bus: true
```

## 🧪 Testing Configuration

To test your configuration:

```bash
# Validate configuration
go run cmd/main/main.go --validate-config

# Print loaded configuration (excludes secrets)
go run cmd/main/main.go --print-config

# Test with specific environment
ENVIRONMENT=staging go run cmd/main/main.go --validate-config
```

## 📚 Complete Configuration Reference

See [`example.yaml`](./example.yaml) for a complete list of all available configuration options with detailed documentation.

## 🔄 Hot Reload (Development Only)

The application supports configuration hot reload in development:

1. Edit any configuration file
2. The application detects changes automatically
3. New configuration is validated and applied
4. Check logs for reload confirmation

**Note:** Hot reload is disabled in staging/production for stability.

## 🐛 Troubleshooting

### Configuration not loading?
1. Check file exists: `ls config/`
2. Verify YAML syntax: `yamllint config/*.yaml`
3. Check application logs for errors

### Environment variable not working?
1. Verify variable is exported: `echo $VARIABLE_NAME`
2. Check naming convention (SECTION_KEY format)
3. Ensure variable is set before starting application

### Validation errors?
1. Read the specific error message
2. Check required fields are present
3. Verify value constraints (min/max, patterns)
4. Compare with example.yaml

## 📖 Additional Resources

- [Configuration Package Documentation](../internal/config/doc.go)
- [Environment Variables Guide](../docs/environment-variables.md)
- [Security Best Practices](../docs/security.md)
- [Deployment Guide](../docs/deployment.md)