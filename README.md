# Brain2 - Your Second Brain

A graph-based personal knowledge management system that automatically connects your memories, thoughts, and ideas based on their content. Built with a modern, event-driven, serverless architecture on AWS.

## Features

-   **Automatic Memory Connections**: Write a memory, and the system automatically connects it to related memories using keyword extraction.
-   **Interactive Knowledge Graph**: Visualize all your memories as an interactive graph showing connections, powered by Cytoscape.js.
-   **Secure & Private**: Each user's data is completely isolated with JWT-based authentication provided by Supabase.
-   **Real-time Updates**: The graph updates instantly as you add new memories, powered by WebSockets.
-   **Scalable Architecture**: Built on AWS serverless technologies (Lambda, DynamoDB, API Gateway) for automatic scaling.
-   **Bulk Operations**: Efficiently delete multiple memories at once.
-   **Inline Editing**: Edit your memories directly in the list view.

## Architecture

### Technology Stack

-   **Frontend**: Vanilla TypeScript, HTML5, CSS3, Vite, and Cytoscape.js
-   **Authentication**: Supabase Auth (JWT provider)
-   **Backend**: Go on AWS Lambda
-   **Database**: AWS DynamoDB (Single-table design)
-   **API**: AWS API Gateway (HTTP and WebSocket APIs)
-   **Hosting**: AWS S3 + CloudFront
-   **Infrastructure**: AWS CDK (TypeScript)
-   **CI/CD**: GitHub Actions

### System Design

```
                           🌐 Brain2 - Event-Driven Architecture
                                                                    
┌─────────────┐     ┌──────────────┐     ┌─────────────┐          
│  CloudFront │────▶│      S3      │     │  Supabase   │          
│    (CDN)    │     │  (Frontend)  │     │    Auth     │          
└─────────────┘     └──────────────┘     └──────┬───────┘          
       │                                         │ JWT              
       │            📡 Real-time Updates         │                  
       │                                         │                  
┌──────▼──────┐                           ┌──────▼──────┐          
│   Client    │◀────── WebSocket ────────▶│ API Gateway │          
│  (Browser)  │        Connection         │ (HTTP + WS) │          
└─────────────┘                           └──────┬───────┘          
       │                                         │                  
       │ HTTP API Calls                          │                  
       │                                         │                  
       └─────────────────────────────────────────┘                  
                                                 │                  
                              ┌──────────────────┼──────────────────┐
                              │                  │                  │
                     ┌────────▼────────┐ ┌──────▼──────┐ ┌────────▼────────┐
                     │  Memory Lambda  │ │ Auth Lambda │ │ WebSocket Lambda│
                     │  (CRUD + NLP)   │ │ (JWT Valid) │ │ (Real-time)     │
                     └────────┬────────┘ └─────────────┘ └────────┬────────┘
                              │                                   │         
                              │          🎯 Event-Driven          │         
                              │                                   │         
                     ┌────────▼────────┐                 ┌────────▼────────┐
                     │  EventBridge    │                 │ Connection Mgmt │
                     │ (Event Router)  │                 │   (DynamoDB)    │
                     └────────┬────────┘                 └─────────────────┘
                              │                                             
                              │                                             
                     ┌────────▼────────┐                                    
                     │   DynamoDB      │                                    
                     │ ┌─────────────┐ │                                    
                     │ │ Graph Nodes │ │                                    
                     │ │ + Keywords  │ │                                    
                     │ │ + Edges     │ │                                    
                     │ │ + Users     │ │                                    
                     │ └─────────────┘ │                                    
                     └─────────────────┘                                    
```

## Prerequisites

- AWS Account (free tier eligible)
- Supabase Account (free tier)
- Node.js 20+ and npm
- Go 1.21+
- AWS CLI configured
- AWS CDK CLI (`npm install -g aws-cdk`)

## Setup

1.  **Clone the repository**:
    ```bash
    git clone [https://github.com/your-username/brain2.git](https://github.com/your-username/brain2.git)
    cd brain2
    ```

2.  **Set up Supabase**:
    -   Create a new project in your Supabase dashboard.
    -   In your Supabase project, go to **Authentication -> Providers** and make sure **Email** is enabled.
    -   Go to **Project Settings -> API**. You will need the **Project URL**, the **`anon` (public) key**, and the **`service_role` key**.

3.  **Configure Environment Variables**:
    -   Create a `.env` file in the `infra` directory (`infra/.env`). **Do not** include the `/auth/v1` path in the URL.
        ```bash
        # infra/.env
        SUPABASE_URL=https://<your-project-id>.supabase.co
        SUPABASE_SERVICE_ROLE_KEY=<your-supabase-service-role-key>
        ```
    -   Create a `.env` file in the `frontend` directory (`frontend/.env`).
        ```bash
        # frontend/.env
        VITE_SUPABASE_URL=https://<your-project-id>.supabase.co
        VITE_SUPABASE_ANON_KEY=<your-supabase-anon-key>
        ```

4.  **Build the application**:
    -   From the root of the project, run the build script. This will build the Go Lambdas, the Lambda authorizer, and the frontend application.
    ```bash
    chmod +x build.sh
    ./build.sh
    ```

5.  **Deploy the Infrastructure**:
    -   Navigate to the `infra` directory and deploy the CDK stack. This will provision all the necessary AWS resources.
    ```bash
    cd infra
    npm install
    npx cdk deploy --all --require-approval never --outputs-file outputs.json
    ```
    -   After deployment, the CDK will create an `outputs.json` file in the `infra` directory.

6.  **Update Frontend with Deployed Endpoints**:
    -   Open the `infra/outputs.json` file.
    -   Find the `HttpApiUrl` and `WebSocketApiUrl` values.
    -   Update your `frontend/.env` file with these values:
        ```bash
        # frontend/.env
        VITE_API_BASE_URL=<your-HttpApiUrl-value>
        VITE_WEBSOCKET_URL=<your-WebSocketApiUrl-value>
        ```

7.  **Re-deploy the Frontend**:
    -   Since the frontend environment variables have changed, you need to rebuild and redeploy it.
    -   From the project root, run the build script again:
        ```bash
        ./build.sh
        ```
    -   From the `infra` directory, run `cdk deploy` again. The CDK is smart enough to only update the changed resources (in this case, the S3 bucket content).
        ```bash
        cd infra
        npx cdk deploy
        ```

## Security

- JWT-based authentication with Supabase
- User data isolation at the database level
- HTTPS everywhere (CloudFront + API Gateway)
- No cross-user data access possible

## Cost Optimization

Designed for AWS free tier:
- DynamoDB: On-demand billing
- Lambda: 1M free requests/month
- API Gateway: 1M free API calls/month
- S3 & CloudFront: Minimal storage and transfer costs

## License

MIT License

### Development and Build Commands

This section outlines the essential commands and scripts for developing, building, and deploying the Brain2 application components.

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

*   `./build.sh`: This script cleans, installs dependencies, runs tests, generates dependency injection code, and builds all Go Lambda functions.
*   **Wire (Dependency Injection) Commands:**
    *   `go install github.com/google/wire/cmd/wire@latest`: Installs the Wire code generation tool.
    *   `go generate ./internal/di`: Generates dependency injection code based on `wire` directives. This is also run by `./build.sh`.
*   **OpenAPI Code Generation (Inferred):**
    *   `oapi-codegen -package api -generate types,server -o pkg/api/api.gen.go ../openapi.yaml`: This command is inferred for generating Go server-side API code from `openapi.yaml`. Its explicit usage is not found in existing scripts, but it's a common pattern for `oapi-codegen`.
*   `go mod tidy`: Cleans up unused dependencies and adds missing ones in `go.mod` and `go.sum`.
*   `go build ./cmd/main`: Compiles the main backend Lambda function executable. You might need to specify `GOOS=linux GOARCH=amd64` for AWS Lambda compatibility.
*   `go test ./...`: Runs all unit and integration tests within the backend project.
*   `go run ./cmd/main`: Runs the main backend application locally (useful for local development and debugging outside of a Lambda environment).

#### Infrastructure (`infra/` directory)

Navigate to the `infra/` directory to run these commands.

*   `npm install`: Installs all necessary Node.js dependencies for the AWS CDK project.
*   `npx cdk deploy [STACK_NAME]`: Deploys the specified CDK stack (e.g., `npx cdk deploy Brain2Stack`). Use `--all` to deploy all stacks.
*   `npx cdk synth [STACK_NAME]`: Synthesizes the CDK application into CloudFormation templates. This shows you what AWS resources will be created.
*   `npx cdk diff [STACK_NAME]`: Compares the current CDK stack definition with the already deployed CloudFormation stack, showing proposed changes.
*   `npx cdk destroy [STACK_NAME]`: Destroys the specified deployed CDK stack and all its resources. **Use with extreme caution!**
*   `npm test`: Runs Jest tests for the infrastructure code.