# B2 Memory Application - Deployment Guide

## Issues Fixed

### 1. Authentication Bug
**Problem**: Users had to enter credentials twice to log in.
**Root Cause**: Frontend wasn't handling successful login response directly.
**Fix**: Modified `frontend/src/ts/auth.ts:83-89` to immediately call `window.showApp()` on successful login.

### 2. API 500 Errors
**Problem**: All API requests (GET /api/nodes, POST /api/nodes) returning 500 errors.
**Root Causes**:
- Lambda authorizer missing required environment variables
- Lambda authorizer TypeScript not compiled to JavaScript
- Backend Go Lambda not properly built
- Wrong Supabase URL format in environment config

## Environment Configuration

### Required Environment Variables

#### Frontend (`.env` in `/frontend/`)
```bash
VITE_SUPABASE_URL=https://id.supabase.co
VITE_SUPABASE_ANON_KEY=key...
VITE_API_BASE_URL=https://id.execute-api.us-west-2.amazonaws.com
```

#### Infrastructure (`.env` in `/infra/`)
```bash
SUPABASE_URL=https://id.supabase.co
SUPABASE_JWT_SECRET=secret+key...
SUPABASE_SERVICE_ROLE_KEY=key...
AWS_REGION=us-west-2
```

## Build Process (From Scratch)

### 1. Frontend Setup
```bash
cd frontend
npm install
npm run dev    # For local development
npm run build  # For production deployment
```

### 2. Lambda Authorizer Setup
```bash
cd infra/lambda/authorizer
npm install
npx tsc index.ts --target es2020 --module commonjs --esModuleInterop --allowSyntheticDefaultImports --skipLibCheck
```

**Note**: The authorizer `package.json` was updated to include:
- `@supabase/supabase-js` dependency
- `@types/aws-lambda` for TypeScript types
- Build script for compilation

### 3. Backend Go Lambda Setup
```bash
cd backend
chmod +x build.sh  # Make build script executable
./build.sh         # Build Go Lambda function
```

**Note**: Fixed `build.sh` formatting issues - the Go build command was split across multiple lines.

### 4. Infrastructure Deployment
```bash
cd infra
npm install
npx cdk deploy
```

## Architecture Overview

### Components
1. **Frontend**: Vite + TypeScript SPA with Supabase auth
2. **API Gateway**: HTTP API with Lambda authorizer
3. **Lambda Authorizer**: Validates Supabase JWT tokens
4. **Backend Lambda**: Go function handling CRUD operations
5. **DynamoDB**: Single table design for memory storage
6. **CloudFront + S3**: Frontend hosting

### Authentication Flow
1. User enters credentials in frontend
2. Frontend calls Supabase `signInWithPassword()`
3. On success, frontend immediately shows app (fixed)
4. API requests include `Authorization: Bearer <jwt_token>`
5. Lambda authorizer validates token with Supabase
6. Backend Lambda processes authorized requests

## Key Files Modified

### `frontend/src/ts/auth.ts`
- Added immediate `window.showApp()` call on successful login
- Captures and uses session data from login response

### `infra/lib/b2-stack.ts`
- Added `SUPABASE_SERVICE_ROLE_KEY` environment variable
- Added proper validation for all required env vars
- Fixed Lambda authorizer environment configuration

### `infra/lambda/authorizer/package.json`
- Added missing dependencies (`@supabase/supabase-js`, `@types/aws-lambda`)
- Added build script for TypeScript compilation

### `backend/build.sh`
- Fixed formatting issues in Go build command
- Made script executable with proper permissions

### `infra/.env`
- Fixed `SUPABASE_URL` format (removed `/auth/v1` suffix)
- Added missing `SUPABASE_SERVICE_ROLE_KEY`

## Deployment Checklist

### Before Deployment
- [ ] All environment variables set correctly
- [ ] Frontend built (`npm run build`)
- [ ] Lambda authorizer compiled (TypeScript → JavaScript)
- [ ] Backend Go Lambda built (`./build.sh`)
- [ ] Infrastructure code updated with latest changes

### Deployment Commands
```bash
# 1. Build frontend
cd frontend && npm run build

# 2. Build authorizer
cd ../infra/lambda/authorizer && npx tsc index.ts --target es2020 --module commonjs --esModuleInterop --allowSyntheticDefaultImports --skipLibCheck

# 3. Build backend
cd ../../../backend && ./build.sh

# 4. Deploy infrastructure
cd ../infra && npx cdk deploy
```

### After Deployment
- [ ] Test login functionality (should work on first attempt)
- [ ] Test memory loading (GET /api/nodes)
- [ ] Test memory creation (POST /api/nodes)
- [ ] Check CloudWatch logs for any remaining errors

## Troubleshooting

### Common Issues
1. **"Cannot find module 'index'" error**: Lambda authorizer TypeScript not compiled
2. **Permission denied on build.sh**: Run `chmod +x backend/build.sh`
3. **500 errors on API calls**: Check environment variables in deployed Lambda functions
4. **Authentication issues**: Verify Supabase URL format and JWT secret

### Log Locations
- Lambda Authorizer: CloudWatch `/aws/lambda/b2Stack-jwt-authorizer`
- Backend Lambda: CloudWatch `/aws/lambda/b2Stack-BackendLambda`
- Frontend: Browser developer console

## Security Notes
- Service role key has elevated permissions - keep secure
- JWT secret used for token validation - never expose in frontend
- CORS configured for all origins (`*`) - restrict in production
- Lambda authorizer caches results for 5 minutes for performance