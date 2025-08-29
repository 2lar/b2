# Environment Configuration Setup Guide

This guide explains how to configure environment variables for the Brain2 project, which uses a unified root-level `.env` file to manage configuration for all components (frontend, backend, and infrastructure).

## Overview

Brain2 uses a **single root-level `.env` file** instead of multiple component-specific `.env` files. This approach:

- ✅ Eliminates duplication of shared variables (like Supabase keys)
- ✅ Provides centralized configuration management
- ✅ Simplifies environment switching between development/staging/production
- ✅ Reduces configuration drift between components

## Quick Start

### 1. Create Environment File

Copy the example file and configure your values:

```bash
# From project root directory
cp .env.example .env
```

### 2. Configure Required Variables

Edit the `.env` file and set the following **required** variables:

```bash
# Project Configuration
PROJECT_ENV=development
PROJECT_NAME=brain2
AWS_REGION=us-west-2

# Supabase Configuration (get from your Supabase project)
SUPABASE_URL=https://your-project-id.supabase.co
SUPABASE_ANON_KEY=your-supabase-anon-key
SUPABASE_SERVICE_ROLE_KEY=your-supabase-service-role-key

# Frontend API URLs
VITE_API_BASE_URL=https://your-api-id.execute-api.us-west-2.amazonaws.com
VITE_API_BASE_URL_LOCAL=http://localhost:8080

# Infrastructure
CDK_DEFAULT_ACCOUNT=123456789012
```

### 3. Build and Run

The build system automatically loads the root `.env` file:

```bash
# Build all components (automatically uses root .env)
./build.sh

# Or build individual components with explicit environment loading
cd frontend && npm run build:with-env
cd infra && npm run deploy:with-env
```

## Environment File Structure

The `.env` file is organized into logical sections:

```bash
# ============================================================================
# PROJECT CONFIGURATION
# ============================================================================
PROJECT_ENV=development              # development, staging, production
PROJECT_NAME=brain2                  # Used for resource naming
AWS_REGION=us-west-2                # AWS region for all resources

# ============================================================================
# SUPABASE AUTHENTICATION (Shared across all components)
# ============================================================================
SUPABASE_URL=https://your-project-id.supabase.co
SUPABASE_ANON_KEY=your-supabase-anon-key          # Safe for frontend
SUPABASE_SERVICE_ROLE_KEY=your-service-role-key   # Backend only - keep secret!

# ============================================================================
# FRONTEND CONFIGURATION (Vite/React - VITE_ prefix required)
# ============================================================================
VITE_SUPABASE_URL=${SUPABASE_URL}                 # Auto-populated from above
VITE_SUPABASE_ANON_KEY=${SUPABASE_ANON_KEY}
VITE_API_BASE_URL=https://your-api-gateway-url
VITE_API_BASE_URL_LOCAL=http://localhost:8080     # For local development

# ============================================================================
# BACKEND CONFIGURATION (Go/Lambda)
# ============================================================================
TABLE_NAME=brain2-dev                             # DynamoDB table name
INDEX_NAME=GSI1                                   # DynamoDB GSI name
LOG_LEVEL=info                                     # debug, info, warn, error

# ============================================================================
# INFRASTRUCTURE/CDK CONFIGURATION
# ============================================================================
CDK_DEFAULT_ACCOUNT=123456789012                  # Your AWS account ID
CDK_DEFAULT_REGION=${AWS_REGION}                  # Inherits from AWS_REGION
```

## Component-Specific Environment Loading

The environment loading system supports loading variables for specific components:

### Frontend Environment

```bash
# Load only frontend-relevant variables (VITE_* and some general vars)
source ./scripts/load-env.sh frontend
```

Variables loaded:
- All `VITE_*` prefixed variables
- General variables: `PROJECT_ENV`, `PROJECT_NAME`, `AWS_REGION`
- Supabase URL and anonymous key

### Backend Environment

```bash
# Load backend variables (excludes VITE_* prefixed variables)
source ./scripts/load-env.sh backend
```

Variables loaded:
- All non-`VITE_*` variables
- Derived variables: `ENV`, `TABLE_NAME`, `INDEX_NAME`

### Infrastructure Environment

```bash
# Load all variables (CDK needs access to everything)
source ./scripts/load-env.sh infra
```

Variables loaded:
- All environment variables
- CDK-specific derived variables: `CDK_DEFAULT_REGION`, `STACK_NAME`

## Environment-Specific Overrides

Use environment-specific loading to apply overrides for different deployment environments:

### Development Environment

```bash
source ./scripts/load-env.sh development
```

Automatically sets:
- `PROJECT_ENV=development`
- `DEBUG=true`
- `LOG_LEVEL=debug`
- `VITE_DEBUG=true`
- `TABLE_NAME=${PROJECT_NAME}-dev`
- `MONITORING_ENABLED=false`

### Staging Environment

```bash
source ./scripts/load-env.sh staging
```

Automatically sets:
- `PROJECT_ENV=staging`
- `DEBUG=false`
- `LOG_LEVEL=info`
- `TABLE_NAME=${PROJECT_NAME}-staging`
- `MONITORING_ENABLED=true`

### Production Environment

```bash
source ./scripts/load-env.sh production
```

Automatically sets:
- `PROJECT_ENV=production`
- `DEBUG=false`
- `LOG_LEVEL=warn`
- `VITE_DEBUG=false`
- `TABLE_NAME=${PROJECT_NAME}-prod`
- `MONITORING_ENABLED=true`
- `WAF_ENABLED=true`

## Build Integration

### Main Build Script

The main `./build.sh` script automatically loads the root `.env` file:

```bash
./build.sh  # Automatically sources ./scripts/load-env.sh all
```

### Component Build Scripts

Each component now has environment-aware build scripts:

**Frontend:**
```bash
cd frontend
npm run build:with-env     # Loads frontend-specific environment
npm run dev:with-env       # Runs dev server with environment
```

**Infrastructure:**
```bash
cd infra
npm run deploy:with-env    # Deploys with environment loaded
npm run synth:with-env     # Synthesizes CDK with environment
npm run destroy:with-env   # Destroys resources with environment
```

## Environment Variable Reference

### Required Variables

These variables **must** be set for the application to function:

| Variable | Component | Description |
|----------|-----------|-------------|
| `SUPABASE_URL` | All | Supabase project URL |
| `SUPABASE_ANON_KEY` | Frontend | Supabase anonymous key (safe for frontend) |
| `SUPABASE_SERVICE_ROLE_KEY` | Backend | Supabase service role key (backend only) |
| `AWS_REGION` | All | AWS region for resources |
| `TABLE_NAME` | Backend | DynamoDB table name |
| `VITE_API_BASE_URL` | Frontend | API Gateway URL for production |
| `CDK_DEFAULT_ACCOUNT` | Infrastructure | AWS account ID for deployment |

### Optional Variables

These variables have sensible defaults but can be overridden:

| Variable | Default | Description |
|----------|---------|-------------|
| `PROJECT_ENV` | `development` | Project environment |
| `PROJECT_NAME` | `brain2` | Project name for resource naming |
| `LOG_LEVEL` | `info` | Backend logging level |
| `INDEX_NAME` | `GSI1` | DynamoDB GSI name |
| `VITE_DEBUG` | `false` | Enable frontend debug mode |
| `MONITORING_ENABLED` | `true` | Enable CloudWatch monitoring |

## Variable Substitution

The environment loading system supports variable substitution using `${VARIABLE_NAME}` syntax:

```bash
# Base configuration
SUPABASE_URL=https://abc123.supabase.co
PROJECT_NAME=brain2
PROJECT_ENV=development

# Variables that reference other variables
VITE_SUPABASE_URL=${SUPABASE_URL}              # Resolves to https://abc123.supabase.co
TABLE_NAME=${PROJECT_NAME}-${PROJECT_ENV}     # Resolves to brain2-development
```

## Security Best Practices

### 1. Never Commit Actual Environment Files

```bash
# .gitignore already includes:
.env
.env.*
!.env.example
```

### 2. Variable Security Levels

- **Public (safe for frontend):** `SUPABASE_ANON_KEY`, `VITE_*` variables
- **Private (backend/infra only):** `SUPABASE_SERVICE_ROLE_KEY`, AWS credentials

### 3. Production Environment

For production deployments:

- Use AWS Parameter Store or Secrets Manager for sensitive values
- Set environment variables in CI/CD pipeline
- Rotate keys regularly
- Use separate Supabase projects for different environments

### 4. Local Development

For local development:

- Copy `.env.example` to `.env` and fill in your values
- Use development-specific Supabase project
- Never commit your local `.env` file

## Troubleshooting

### Missing Environment Variables

If you see errors like:

```
Error: VITE_SUPABASE_URL is not defined. Please check your .env file.
```

**Solution:**
1. Ensure `.env` file exists in project root
2. Check that the variable is defined in `.env`
3. For `VITE_*` variables, ensure they're prefixed correctly
4. Restart your development server after changing `.env`

### Environment Not Loading

If environment variables aren't being loaded:

**For Build Scripts:**
```bash
# Verify the environment loading script works
source ./scripts/load-env.sh frontend
echo $VITE_SUPABASE_URL  # Should print your Supabase URL
```

**For Frontend (Vite):**
```bash
# Check vite.config.ts envDir setting
cd frontend
npm run dev  # Check console output for environment loading logs
```

**For Infrastructure (CDK):**
```bash
# Check that infra/bin/infra.ts loads from correct path
cd infra
npm run synth  # Should show loaded environment info
```

### Variable Substitution Not Working

If variables like `${SUPABASE_URL}` aren't being resolved:

1. Ensure the referenced variable is defined before the substitution
2. Check for circular references
3. Use the environment loading script instead of manually sourcing

## Migration from Multiple .env Files

If you're migrating from the old system with multiple `.env` files:

1. **Backup existing files:**
   ```bash
   cp frontend/.env frontend/.env.backup
   cp infra/.env infra/.env.backup
   ```

2. **Copy values to root .env:**
   - Copy `VITE_*` variables from `frontend/.env`
   - Copy infrastructure variables from `infra/.env`
   - Remove duplicated Supabase variables

3. **Test the migration:**
   ```bash
   ./build.sh  # Should build successfully with new environment system
   ```

4. **Remove old files** (after confirming everything works):
   ```bash
   rm frontend/.env
   rm infra/.env
   ```

## Examples

### Development Environment Setup

```bash
# 1. Copy example file
cp .env.example .env

# 2. Edit .env with your development values
PROJECT_ENV=development
SUPABASE_URL=https://dev-project.supabase.co
SUPABASE_ANON_KEY=your-dev-anon-key
SUPABASE_SERVICE_ROLE_KEY=your-dev-service-key
VITE_API_BASE_URL=http://localhost:8080
TABLE_NAME=brain2-dev
CDK_DEFAULT_ACCOUNT=123456789012

# 3. Build and run
./build.sh
cd frontend && npm run dev
```

### Production Deployment

```bash
# 1. Set production environment variables
PROJECT_ENV=production
SUPABASE_URL=https://prod-project.supabase.co
SUPABASE_SERVICE_ROLE_KEY=your-prod-service-key
VITE_API_BASE_URL=https://api.yourdomain.com
TABLE_NAME=brain2-prod
MONITORING_ENABLED=true
WAF_ENABLED=true

# 2. Deploy infrastructure
cd infra && npm run deploy:with-env

# 3. Build and deploy frontend
./build.sh
```

## Getting Help

- Check the [build documentation](../backend/README.md#environment-configuration) for build-specific issues
- See [troubleshooting guide](../infra/docs/troubleshooting.md) for deployment issues
- Review the [environment loading script](../scripts/load-env.sh) source code for advanced usage