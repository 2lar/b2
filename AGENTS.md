# Agent Operations Guide

## Essentials
- Follow these instructions first when planning work
- Prefer quick edits before automation; keep formatting consistent
- Ask for clarification if expectations or prior work are unclear

## Repository Layout
- `backend/` – Go services organised by DDD layers (`domain/`, `application/`, `interfaces/`); entrypoints live in `cmd/`
- `frontend/` – Vite + TypeScript client (`src/` UI, generated API types in `src/types/generated/`, static assets in `public/`)
- `infra/` – AWS CDK stacks and the Lambda authorizer
- `scripts/` – Automation helpers (linting, builds, env loading)
- `openapi.yaml`, `docs/` – Shared API contracts and documentation
- `backend/tests/` – Integration fixtures and end-to-end scenarios

## Build, Test, and Dev Loops
- Repo root: `./build.sh` builds Go Lambdas, bundles the authorizer, and compiles the frontend
- Backend: `make build` or `./build.sh`; run `make test` or `./test.sh --all --coverage` for full coverage (reports in `backend/coverage/`)
- Frontend: `npm install` then `npm run dev` for live reload, `npm run build` for production, regenerate types with `npm run generate-api-types`
- Infra: `npm install` then `npx cdk deploy --all --require-approval never`

## Coding Standards
- Go: always run `gofmt`/`goimports`; use lowercase package names; keep CQRS boundaries clear by placing adapters in `interfaces/`
- TypeScript: four-space indent, PascalCase components, camelCase hooks/utilities, colocate styling; run `npm test` (tsc) after changes
- Comments: explain intent or tricky logic only—skip obvious restatements

## Engineering Practices
- **Readability & Clarity** – Use descriptive names, keep functions focused, explain complex business decisions, rely on formatters
- **Structure & Organization** – Keep related code together, enforce separation of concerns, introduce meaningful abstractions, guard-clauses over deep nesting
- **Error Handling & Robustness** – Validate inputs early, surface clear errors, choose appropriate error types, log with relevant context
- **Performance & Efficiency** – Start simple, choose data structures deliberately, release resources promptly, cache only when justified
- **Maintainability** – Write unit/integration tests, trim dependencies, keep commits small and focused, document major design choices

## Testing Guidance
- Name Go tests `*_test.go`; keep table-driven tests near the code they cover
- Broader scenarios belong in `backend/tests/` with descriptive directories
- Backend changes: run `./test.sh --all --coverage` and note coverage shifts when possible
- Frontend changes: ensure `npm test` passes; add Vitest/Cypress specs in `frontend/src/__tests__/` for interactive behavior

## Commit & PR Expectations
- Follow Conventional Commit prefixes (`feat:`, `fix:`, `refactor:`); keep subjects imperative and ≤ ~72 chars
- Include matching tests with code changes where applicable
- PRs should describe the problem, solution, rollout steps, and link issues
- Provide evidence of `make test`, `npm run build`, or `cdk diff` as appropriate, plus UI screenshots for visual updates
- Request reviewers from each affected area

## Security & Configuration
- Load secrets with `scripts/load-env.sh`; keep `.env` files scoped to `frontend/` and `infra/`
- Never commit secrets; prefer environment variables or secure vaults
- Validate IAM/network changes with `npx cdk diff` before deployment
- Do not change Supabase JWT settings without coordinating with auth owners
- Default to secure posture: validate all external inputs, fail closed, grant least privilege
