# Frontend Services Layer

This directory contains all service modules that handle external communication and business logic, separated from the React UI layer.

## Structure

- `apiClient.ts` - REST API communication with type-safe methods
- `authClient.ts` - Supabase authentication wrapper
- `webSocketClient.ts` - Real-time WebSocket connection management
- `generated-types.ts` - Auto-generated TypeScript types from OpenAPI spec
- `index.ts` - Barrel export for convenient importing

## Usage

Import services in React components:
```typescript
import { api, auth, webSocketClient } from '../services';
```

## Type Safety

All API methods are fully typed using generated types from the OpenAPI specification.