#### Root Project Commands

*   `chmod +x build.sh && ./build.sh`: This script orchestrates the build process for both the backend Go Lambdas and the frontend application. It's typically run from the project root.

#### Frontend (`frontend/` directory)

Navigate to the `frontend/` directory to run these commands.

*   `npm install`: Installs all necessary Node.js dependencies.
*   `npm run dev`: Starts the development server with hot-reloading for local development.
*   `npm run build`: Cleans the `dist` directory, reinstalls dependencies, generates API types from `openapi.yaml`, performs TypeScript type checking, and then builds the production-ready frontend assets.
*   `npm run preview`: Serves the production build locally for testing.
*   `npm run generate-api-types`: Generates TypeScript types for the API client based on `openapi.yaml`. This ensures type safety between the frontend and backend.
*   `npm test`: Runs TypeScript type checking (`tsc --noEmit`) to catch type-related errors. (Note: This project currently lacks comprehensive unit/integration tests for the frontend beyond type checking.)
*   `npm run clean`: Removes `node_modules` and `dist` directories.

#### Backend (`backend/` directory)

Navigate to the `backend/` directory to run these commands.

*   `./build.sh`: Builds all Lambda functions for deployment to AWS, creating binaries in the `build/` directory.
*   `./run-local.sh`: Runs the backend API server locally on port 8080 for development and debugging.
*   **Wire (Dependency Injection) Commands:**
    *   `go install github.com/google/wire/cmd/wire@latest`: Installs the Wire code generation tool.
    *   `go generate ./infrastructure/di`: Generates dependency injection code based on `wire` directives.
*   `go mod tidy`: Cleans up unused dependencies and adds missing ones in `go.mod` and `go.sum`.
*   `go test ./...`: Runs all tests within the backend project.
*   `go fmt ./...`: Formats all Go code to match the standard Go formatting conventions.
*   `go vet ./...`: Runs Go's built-in static analyzer to find potential issues in the code.

#### Infrastructure (`infra/` directory)

Navigate to the `infra/` directory to run these commands.

*   `npm install`: Installs all necessary Node.js dependencies for the AWS CDK project.
*   `npx cdk deploy [STACK_NAME]`: Deploys the specified CDK stack (e.g., `npx cdk deploy Brain2Stack`). Use `--all` to deploy all stacks.
*   `npx cdk synth [STACK_NAME]`: Synthesizes the CDK application into CloudFormation templates. This shows you what AWS resources will be created.
*   `npx cdk diff [STACK_NAME]`: Compares the current CDK stack definition with the already deployed CloudFormation stack, showing proposed changes.
*   `npx cdk destroy [STACK_NAME]`: Destroys the specified deployed CDK stack and all its resources. **Use with extreme caution!**
*   `npm test`: Runs Jest tests for the infrastructure code.