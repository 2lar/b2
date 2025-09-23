# Frontend Gold-Standard Upgrade Plan

## 0. Guardrails and Baseline
- [x] Capture current behaviour with lightweight QA checklist (auth flows, dashboard interactions, memory CRUD) to validate after refactors.
- [ ] Enable preview deploy or Storybook stub to visually verify components as styles move.

## 1. Styling Modernisation
- [ ] Inventory all classNames still bound to `styles/base.css`, `new-layout.css`, and other legacy sheets.
- [x] Extract component-scoped CSS modules for the dashboard main area, memory list/pagination controls, graph container, auth UI, and global typography.
- [ ] Replace string-based `className` compositions with module imports; remove unused selectors from legacy sheets.
- [ ] Deprecate and delete `new-layout.css` once all selectors are migrated.
- [ ] Introduce design tokens/primitives (spacing scale, typography scale) in a shared module for reuse across CSS modules.

## 2. Component Simplification & Comments
- [ ] Replace banner-style comments with concise docstrings or inline explanations only where logic is non-obvious (MemoryInput, MemoryList, GraphVisualization, SmartMemoryInput, DocumentEditor, AuthSection, etc.).
- [ ] Split oversized components:
  - MemoryInput → compact form + document editor wrapper.
  - MemoryList → table/virtual list renderer vs. actions toolbar.
  - GraphVisualization → data loader hook + presentational renderer.
- [ ] Add Storybook or unit snapshots for these components after splitting to lock in behaviour.

## 3. Data Fetching & State Patterns
- [x] Introduce React Query `useInfiniteQuery` for dashboard memory pagination instead of manual token Map.
- [x] Wrap category fetches in dedicated hooks (`useCategories`, `useCategoryNodes`) with proper loading/error states; surface results through the sidebar.
- [ ] Implement optimistic mutations via React Query mutations instead of bespoke Zustand/Map logic.
- [ ] Establish a lightweight graph state hook (e.g., `useGraphData`) returning typed data and imperative helpers consumed by GraphVisualization.

## 4. API Hardening
- [ ] Confirm API client aligns with actual backend capabilities (category endpoints, node suggestions). Add feature flags or graceful fallbacks when endpoints are absent.
- [ ] Centralise error translation so UI components display consistent messaging.
- [ ] Remove dead code paths (debug auth helpers, console logs) and document how to enable debug tooling in development.

## 5. Tooling & Quality Gates
- [ ] Add ESLint + Prettier config aligned with repo standards (four-space indentation, import order, hooks rules).
- [ ] Configure Vitest with jsdom for React components; add sample tests for hooks (useDashboardData) and components (NotificationBanner, LeftPanel).
- [ ] Update npm scripts: `lint`, `test`, `test:watch`, `format`, integrate into CI instructions.
- [ ] Document expected quality checks in `frontend/README.md`.

## 6. Accessibility & UX Polish
- [x] Audit interactive controls for keyboard focus styles and ARIA labelling (LeftPanel toggles, MemoryList actions, Graph controls, Auth forms).
- [ ] Ensure color contrast meets WCAG AA in both dark/light themes.
- [x] Add skip-to-content link and landmark roles for major layout sections.

## 7. Documentation & Developer Experience
- [ ] Update `frontend/README.md` with architecture overview (state management, styling approach, testing strategy).
- [ ] Add ADR or short write-ups explaining key decisions (React Query pattern, CSS modules, auth flow).
- [ ] Provide contributor checklist (lint/test commands, visual regression steps).

## 8. Validation & Rollout
- [ ] Run full regression (manual + automated) after each milestone.
- [ ] Capture screenshots or recordings of key flows for documentation.
- [ ] Publish changelog/upgrade guide summarising the transformation for future contributors.
