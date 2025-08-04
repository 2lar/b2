# Frontend Code Organization Plan: Feature-Based Architecture

## Goal
To reorganize the frontend codebase into a feature-based structure, enhancing discoverability, maintainability, scalability, and team collaboration, while retaining a layered approach within each feature.

## Current State Assessment
The frontend currently uses a layered architecture (`components/`, `hooks/`, `services/`, `types/`). While effective for smaller applications, this can lead to large, flat directories as the application grows, making it harder to navigate and maintain.

## Proposed New Structure

```
frontend/src/
├── app/                  # Root application setup (App.tsx, routing, global layout)
├── common/               # Truly shared, generic components, hooks, utilities
├── features/             # All application features, organized by domain/feature
│   ├── auth/             # Authentication feature
│   │   ├── components/   # Auth-related UI components (Login, Register, AuthSection)
│   │   ├── hooks/        # Auth-related custom hooks (useAuth)
│   │   ├── api/          # Auth-related API calls (authClient.ts, or specific auth endpoints)
│   │   ├── types/        # Auth-specific types (if any)
│   │   └── index.ts      # Barrel export for the auth feature
│   ├── memories/         # Memory management feature
│   │   ├── components/   # Memory-related UI (MemoryInput, MemoryList, GraphVisualization)
│   │   ├── hooks/        # Memory-related hooks (e.g., useMemories)
│   │   ├── api/          # Memory-related API calls (apiClient methods for nodes)
│   │   ├── types/        # Memory-specific types (if any)
│   │   └── index.ts
│   ├── categories/       # Category management feature
│   │   ├── components/   # Category-related UI (CategoriesList, CategoryDetail)
│   │   ├── hooks/        # Category-related hooks (e.g., useCategories)
│   │   ├── api/          # Category-related API calls (apiClient methods for categories)
│   │   ├── types/        # Category-specific types (if any)
│   │   └── index.ts
│   └── ... (other features as they emerge)
├── services/             # Global API clients (if not feature-specific, e.g., apiClient, webSocketClient)
├── types/                # Centralized global type definitions (generated-types.ts, global.d.ts, cytoscape-cola.d.ts, errors.ts)
├── main.tsx              # Application entry point
├── index.html            # Main HTML file
└── style.css             # Global styles
```

## Implementation Plan

### Phase 1: Preparation & Setup

1.  **Create Top-Level Directories:**
    *   `mkdir -p frontend/src/app`
    *   `mkdir -p frontend/src/common`
    *   `mkdir -p frontend/src/features`
    *   `mkdir -p frontend/src/services` (if it doesn't exist, or ensure it's for global services)

2.  **Review Existing Codebase:**
    *   Identify distinct features/domains (e.g., Authentication, Memory Management, Category Management).
    *   List all components, hooks, and service calls associated with each feature.

### Phase 2: Migrate `app/` and `common/`

1.  **Move `App.tsx` to `frontend/src/app/`:**
    *   `mv frontend/src/components/App.tsx frontend/src/app/App.tsx`
    *   Update import path in `frontend/src/main.tsx`:
        ```typescript
        // Before:
        // import App from './components/App';
        // After:
        import App from './app/App';
        ```

2.  **Create `frontend/src/app/index.ts`:**
    *   `touch frontend/src/app/index.ts`
    *   Add `export { default as App } from './App';`

3.  **Populate `frontend/src/common/`:**
    *   Identify any truly generic components (e.g., a generic `Button`, `Modal`, `LoadingSpinner`), hooks (e.g., `useLocalStorage`, `useDebounce`), or utility functions that are not specific to any single feature.
    *   Move them from `frontend/src/components/` and `frontend/src/hooks/` into `frontend/src/common/`.
    *   Create `index.ts` barrel files within `common/` subdirectories (e.g., `common/components/index.ts`, `common/hooks/index.ts`) for organized exports.
    *   Update import paths in files that use these common elements.

### Phase 3: Feature Migration (Iterative Process)

Perform these steps for each identified feature (e.g., `auth`, `memories`, `categories`).

1.  **Create Feature Directory Structure:**
    *   `mkdir -p frontend/src/features/<feature-name>/components`
    *   `mkdir -p frontend/src/features/<feature-name>/hooks`
    *   `mkdir -p frontend/src/features/<feature-name>/api`
    *   `mkdir -p frontend/src/features/<feature-name>/types` (for feature-specific types)

2.  **Move Feature-Specific Components:**
    *   Move components from `frontend/src/components/` that belong to this feature into `frontend/src/features/<feature-name>/components/`.
    *   **Example (Auth Feature):**
        *   `mv frontend/src/components/AuthSection.tsx frontend/src/features/auth/components/AuthSection.tsx`
        *   Update imports in `App.tsx` (now `app/App.tsx`) and any other files that use `AuthSection`.

3.  **Move Feature-Specific Hooks:**
    *   Move hooks from `frontend/src/hooks/` that belong to this feature into `frontend/src/features/<feature-name>/hooks/`.
    *   **Example (Auth Feature):**
        *   `mv frontend/src/hooks/useAuth.ts frontend/src/features/auth/hooks/useAuth.ts`
        *   Update imports in `App.tsx` and `AuthSection.tsx`.

4.  **Refactor API Calls (Feature-Specific):**
    *   Review `frontend/src/services/apiClient.ts` and `authClient.ts`.
    *   **Option A (Recommended for true feature separation):** Create feature-specific API modules within `frontend/src/features/<feature-name>/api/`. Extract relevant methods from `apiClient.ts` and `authClient.ts` into these new modules.
        *   **Example (`frontend/src/features/auth/api/auth.ts`):**
            ```typescript
            // This would contain methods like signIn, signUp, signOut, etc.
            // It would internally use the global supabase client or a more granular http client.
            import { auth as globalAuth } from '../../services/authClient'; // Or a more direct import if authClient is moved

            export const auth = {
                signIn: globalAuth.signIn,
                signUp: globalAuth.signUp,
                signOut: globalAuth.signOut,
                getSession: globalAuth.getSession,
                getJwtToken: globalAuth.getJwtToken,
                supabase: globalAuth.supabase
            };
            ```
        *   **Example (`frontend/src/features/memories/api/nodes.ts`):**
            ```typescript
            // This would contain methods like createNode, listNodes, getNode, etc.
            import { api as globalApi } from '../../services/apiClient'; // Or a more direct import

            export const nodesApi = {
                createNode: globalApi.createNode,
                listNodes: globalApi.listNodes,
                getNode: globalApi.getNode,
                // ... other node-related API calls
            };
            ```
    *   **Option B (Simpler, less strict separation):** Keep `apiClient.ts` and `authClient.ts` in `frontend/src/services/` as global API clients. Feature components/hooks would then import directly from `frontend/src/services/`.
        *   *Decision for this plan:* We will proceed with **Option A** to achieve better feature encapsulation. The `frontend/src/services/` directory will then primarily contain the low-level HTTP/WebSocket client setup, and the feature `api/` modules will wrap these for feature-specific use cases.

5.  **Create Feature Barrel File (`index.ts`):**
    *   Inside `frontend/src/features/<feature-name>/`, create an `index.ts` file.
    *   Export all public components, hooks, and API clients from this feature.
    *   **Example (`frontend/src/features/auth/index.ts`):**
        ```typescript
        export * from './components/AuthSection';
        export * from './hooks/useAuth';
        export * from './api/auth';
        // Export types if they are feature-specific and need to be consumed outside
        ```

6.  **Update Imports:**
    *   After moving files, update all import statements to reflect the new paths. Use relative paths within features, and absolute paths (or aliases) for imports from `app/`, `common/`, `services/`, or other `features/`.

### Phase 4: Refine `services/` and `types/`

1.  **`frontend/src/services/`:**
    *   This directory should now primarily contain the low-level, generic API client (`apiClient.ts`) and authentication client (`authClient.ts`) that are used by the feature-specific `api/` modules.
    *   `webSocketClient.ts` should remain here as it's a global communication layer.
    *   The `index.ts` barrel file in `services/` should only export these core clients.

2.  **`frontend/src/types/`:**
    *   This directory should contain all global type definitions:
        *   `generated-types.ts` (from OpenAPI)
        *   `global.d.ts` (global type augmentations)
        *   `cytoscape-cola.d.ts` (third-party type extensions)
        *   `errors.ts` (generic API/Auth errors, if they are truly global and not specific to `services`)
    *   Feature-specific types should reside within `frontend/src/features/<feature-name>/types/`.

### Phase 5: Update Path Aliases (if necessary)

1.  **`tsconfig.json` and `vite.config.ts`:**
    *   Review and update `paths` aliases in `tsconfig.json` and `resolve.alias` in `vite.config.ts` to reflect the new structure.
    *   Consider adding aliases for `app/`, `common/`, and `features/` for cleaner imports.
    *   **Example `tsconfig.json` paths:**
        ```json
        "paths": {
          "@app/*": ["app/*"],
          "@common/*": ["common/*"],
          "@features/*": ["features/*"],
          "@services/*": ["services/*"],
          "@types/*": ["types/*"]
        }
        ```

### Phase 6: Testing & Verification

1.  **Unit/Component Tests:** Update existing tests and write new ones for components and hooks within their new feature directories.
2.  **Integration Tests:** Test the interaction between different layers and features.
3.  **End-to-End Tests:** Verify full application flows.
4.  **Build and Run:** Ensure the application builds without errors and functions as expected.
5.  **Code Review:** Conduct thorough code reviews to ensure adherence to the new structure and best practices.

This detailed plan provides a step-by-step guide for reorganizing the frontend into a more scalable and maintainable feature-based architecture.
