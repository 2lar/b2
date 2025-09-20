# Frontend

Brain2's frontend is a Vite-powered React 19 single-page application written in TypeScript. It provides authenticated graph exploration, memory management, and category tooling backed by Supabase auth, the backend REST API, and a real-time WebSocket channel. This document explains how the project is organised, how data flows through the app, and which commands you need for development, testing, and production builds.

## Quick Start

1. **Prerequisites**
   - Node.js 20.x (LTS) or newer and npm 10+
   - Access to the shared `openapi.yaml` in the repository root
   - Supabase credentials and backend/API URLs exposed as environment variables

2. **Install dependencies**
   ```bash
   cd frontend
   npm install
   ```

3. **Configure environment**
   - Create an `.env.local` (or `.env.development`) in the repository root and prefix frontend variables with `VITE_` (see [Environment Variables](#environment-variables)).
   - Optionally load environment files via `../scripts/load-env.sh`:
     ```bash
     source ../scripts/load-env.sh frontend
     ```

4. **Generate API types**
   ```bash
   npm run generate-api-types   # runs openapi-typescript on ../openapi.yaml
   ```

5. **Run the development server**
   ```bash
   npm run dev                  # serves src/ through Vite (default mode)
   npm run dev:with-env         # loads env vars via scripts/load-env.sh first
   ```

6. **Type-check / tests**
   ```bash
   npm run test                 # runs TypeScript (tsc --noEmit)
   ```

7. **Build for production**
   ```bash
   npm run build                # clean, reinstall deps, generate types, tsc, vite build
   npm run build:with-env       # same, but loads env vars beforehand
   npm run preview              # serve the built dist/ bundle locally
   ```

The compiled assets are emitted to `frontend/dist` and can be hosted on any static CDN (e.g. S3 + CloudFront) as long as SPA routing is supported.

## Tech Stack & Architecture

- **React 19** with Suspense/`lazy` for route-level code splitting
- **TypeScript 5** with strict mode enforced during builds (`tsc --noEmit`)
- **Vite 5** (rooted at `src/`) for lightning-fast dev server and Rollup-based builds
- **React Router 7** for client-side routing
- **TanStack Query 5** for server-state caching, mutations, and optimistic updates
- **Zustand 5** (+ `persist` and `devtools`) for long-lived graph state and UI flags
- **Supabase JS 2** for authentication/session handling
- **Cytoscape.js + cola layout** for graph visualisation
- **Framer Motion** for micro-interactions
- **web-vitals** instrumentation hooks for runtime performance measurement

## Project Structure

```
frontend/
├── public/                           # Static assets served as-is (currently empty)
├── src/                              # Vite root
│   ├── app/                          # Application shell, routing, error boundaries
│   ├── common/                       # Shared UI primitives, constants, hooks
│   ├── components/                   # Cross-feature presentation components
│   ├── features/                     # Feature-specific modules
│   │   ├── auth/                     # Supabase session hooks & AuthSection UI
│   │   ├── dashboard/                # Graph dashboard views + API hooks
│   │   ├── memories/                 # Memory CRUD flows and TanStack Query logic
│   │   └── categories/               # Category list/detail UI and data hooks
│   ├── hooks/                        # Reusable React hooks (non-feature specific)
│   ├── services/                     # API/auth/WebSocket clients and helpers
│   ├── stores/                       # Zustand stores (e.g. graphStore.ts)
│   ├── styles/                       # Feature-level stylesheets
│   ├── types/                        # Domain typings + generated OpenAPI types
│   └── utils/                        # Utility helpers (formatting, guards, etc.)
├── dist/                             # Build output (gitignored)
├── node_modules/
├── package.json                      # Scripts, dependencies, and build pipeline
├── package-lock.json
├── tsconfig.json                     # TypeScript configuration shared across scripts
└── vite.config.ts                    # Vite/Rollup configuration (manual chunking, aliases)
```

### Routing & App Shell (`src/app`)
`main.tsx` instantiates a `QueryClient` and renders `<App />` within `QueryClientProvider`, attaching `ReactQueryDevtools` in development. `App.tsx` manages authentication-aware routing, lazy-loads feature bundles, and coordinates WebSocket lifecycle events through `webSocketClient` and the persisted Zustand graph store.

### State & Data Flow
- **Server state** lives in TanStack Query caches. Hooks inside `features/**/hooks` wrap queries/mutations for specific resources (nodes, categories, memories) and handle optimistic updates.
- **Client state** (selected graph nodes, UI toggles, caches) is stored in `useGraphStore` (Zustand + `persist`). Actions inside the store call the typed API client and reconcile results with optimistic updates and caches.
- **Events**: `services/webSocketClient.ts` dispatches `CustomEvent('graph-update-event', …)` on `document`, enabling listeners anywhere in the app to react to real-time updates.

## API Integration & Generated Types

`npm run generate-api-types` executes `openapi-typescript` on `../openapi.yaml`, emitting `src/types/generated/generated-types.ts`. All service modules import types from this file to ensure request/response payloads stay aligned with backend contracts.

`services/apiClient.ts` centralises authenticated fetch calls. It:
- Resolves the base URL from `VITE_API_BASE_URL` (or `VITE_API_BASE_URL_LOCAL` if toggled)
- Retrieves JWTs from Supabase (`authClient.getJwtToken()`)
- Implements retry/backoff logic and meaningful error messages for auth, rate limits, and cold starts
- Exposes typed helpers for graph/memory/category endpoints consumed by features and the Zustand store

## Authentication & Session Management

`services/authClient.ts` wraps Supabase auth:
- Validates `VITE_SUPABASE_URL` / `VITE_SUPABASE_ANON_KEY` at startup
- Exposes `auth.getSession()`, `auth.signIn`, `auth.signOut`, and token refresh with exponential backoff
- Provides `auth.getJwtToken()` used by the API and WebSocket clients
- `features/auth` surfaces hooks/components (`useAuth`, `AuthSection`) for the UI

## Real-Time Updates

`services/webSocketClient.ts` handles connection logic against `VITE_WEBSOCKET_URL`:
- Attaches the Supabase JWT as a query parameter for authentication
- Reconnects automatically with exponential backoff on failures
- Emits DOM events for node/edge updates that the graph store can consume
- Cleans up connections when a user signs out or their session expires

For more advanced batching and throttling, `services/optimizedWebSocketClient.ts` provides an alternate implementation ready to be wired in when needed.

## Styling & UI Conventions

- Global styles reside in `src/style.css` and `src/new-layout.css`
- Feature-scoped styles (e.g. document editor) live under `src/styles`
- Components prefer CSS Modules or scoped class names; ensure new global styles do not conflict across features
- `common` contains shared layout components and error boundaries reused throughout the app

## Build & Performance Configuration

`vite.config.ts` customises the build to keep interactive load times low:
- **Root** is set to `src/` while `envDir` points to the repo root so Vite loads `.env*` files alongside backend/infra configs
- **Manual Rollup chunks** create dedicated bundles for React, state/query libraries, graph libs, utilities, and Supabase to improve caching
- **Custom chunk naming** (`assets/[name]-<facade>-<[hash]>.js`) simplifies debugging and cache inspection
- `chunkSizeWarningLimit` raises the threshold to account for Cytoscape bundles
- `optimizeDeps.include` pre-bundles heavy packages during dev for faster HMR
- Source maps (`sourcemap: true`) and esbuild minification are enabled by default for production diagnostics

## Environment Variables

Define the following (prefix with `VITE_`) in the repository root `.env` files:

| Variable | Required | Description |
|----------|----------|-------------|
| `VITE_SUPABASE_URL` | ✅ | Supabase project URL for auth requests |
| `VITE_SUPABASE_ANON_KEY` | ✅ | Supabase anonymous key used by the browser client |
| `VITE_API_BASE_URL` | ✅ | HTTPS endpoint for the backend API (used in production builds and by default in dev) |
| `VITE_API_BASE_URL_LOCAL` | ⭕️ | Optional local/dev API endpoint; enable in `apiClient` when running the Go API locally |
| `VITE_WEBSOCKET_URL` | ⭕️ | WebSocket endpoint for real-time graph updates |
| `VITE_FORCE_PRODUCTION_API` | ⭕️ | When `true`, forces production API usage even if local endpoints are configured |

Remember that Vite only exposes variables beginning with `VITE_` to the browser bundle.

## Development Workflow

1. Source environment variables (`source ../scripts/load-env.sh frontend`) if you keep secrets outside `.env.local`.
2. Start Supabase / backend services (local or remote) as needed.
3. Run `npm run dev`. Vite serves from `src/index.html`; HMR keeps component updates instant.
4. Inspect TanStack Query cache with React Query Devtools (press > button in dev tools dock) and check the console for any token refresh logs.
5. For WebSocket testing, ensure `VITE_WEBSOCKET_URL` points to a reachable endpoint—the app will automatically connect after login.
6. Use `npm run test` before committing to guarantee the TypeScript compiler remains happy.

## Testing & Quality Gates

- `npm run test` (default script) type-checks the entire project via `tsc --noEmit`.
- When integrating with backend changes, re-run `npm run generate-api-types` to catch contract regressions early.
- Consider adding Vitest or Cypress tests under `src/__tests__` / `cypress/` respectively; existing scripts act as placeholders for now.

## Troubleshooting

- **Environment variables missing**: Ensure `.env` lives in the repo root (because `envDir` is `../`). Variables must be prefixed with `VITE_`.
- **Manual chunk warnings**: Review `vite.config.ts` if any vendor chunk exceeds the 600 KB warning, and split further if necessary.
- **WebSocket auth errors**: Confirm the Supabase session is valid and that the backend accepts the JWT in query params. Re-check `VITE_WEBSOCKET_URL`.
- **Slow builds**: The default `build` script reinstalls dependencies for reproducibility. For local faster builds you can temporarily run `vite build` directly (but keep the scripted build for CI).
- **Type generation failures**: Verify `openapi-typescript` is installed (`npm install`) and that `../openapi.yaml` is accessible.

## Contributing Guidelines

- Follow feature-based organisation—new functionality belongs under `src/features/<feature-name>` with colocated hooks/components/types.
- Use TypeScript everywhere; prefer explicit types for public APIs and exported hooks.
- Keep state mutations inside zustand actions or TanStack Query mutations to centralise side effects.
- Reuse `common` components and utilities before introducing new primitives.
- Document significant UI or architectural changes in this README (and/or `docs/`) and include screenshots or screen recordings when altering user-facing flows.
- Run `npm run generate-api-types` after backend contract updates and commit the resulting file when appropriate.

This frontend is intentionally modular. Extending it with new surfaces (e.g. GraphQL clients, alternative visualisations, or collaborative features) should plug into the existing service/adaptor layer and reuse the domain-specific hooks provided in `features/`.
