# Brain2: A Graph-Based Personal Knowledge Management System

## 1. Application Overview

**Name:** Brain2
**Concept:** A graph-based personal knowledge management (PKM) system.
**Core Functionality:** Users create "memories" (nodes), and the system automatically forms connections (edges) between them based on content similarity (keyword extraction). This creates an interactive, visual knowledge graph.

## 2. Architecture

The application is a serverless, event-driven system built entirely on AWS, designed to be cost-effective and scalable.

### Frontend

* **Framework:** Vanilla TypeScript (no major framework like React or Angular).
* **Build Tool:** Vite.
* **Core Logic:** Handles user input, authentication flow, API communication, and rendering the UI.
* **Visualization:** Uses [Cytoscape.js](https://cytoscape.org/) to render and manage the interactive knowledge graph.

### Backend

* **Language:** Go (Golang).
* **Deployment:** AWS Lambda functions.
* **Architecture:** A set of specialized, single-responsibility Lambda functions.

### API Layer

* **HTTP API:** Amazon API Gateway handles RESTful requests (CRUD operations for nodes).
* **WebSocket API:** A separate API Gateway endpoint manages real-time communication for instant graph updates.

### Database

* **Type:** Amazon DynamoDB.
* **Design:** Single-table design, a common pattern for NoSQL databases in serverless applications. It uses a generic PK (Partition Key) and SK (Sort Key) to store different entity types (nodes, edges, keywords) in the same table.
* **Indexes:** A Global Secondary Index (GSI) is used to efficiently query nodes by keywords.

### Authentication

* **Provider:** [Supabase Auth](https://supabase.com/docs/guides/auth). It acts as a third-party JWT (JSON Web Token) provider.
* **Mechanism:** A dedicated Lambda authorizer, attached to the API Gateway, validates the JWT from the `Authorization` header of every incoming request.

### Asynchronous Processing

* **Service:** Amazon EventBridge.
* **Workflow:** When a new node is created, the main API Lambda publishes a `NodeCreated` event. This decouples the initial request from the heavier processing of finding connections.

### Infrastructure as Code (IaC)

* **Tool:** AWS CDK (Cloud Development Development Kit) using TypeScript.
* **Scope:** Defines all AWS resources, including DynamoDB tables, Lambda functions, API Gateways, S3 buckets, CloudFront distributions, and IAM roles.

### CI/CD

* **Platform:** GitHub Actions.
* **Workflows:** Separate workflows for deploying the frontend and backend automatically on pushes to the `main` branch.

## 3. Key Codebases & Directories

* `frontend/`: Contains all client-side code (TypeScript, HTML, CSS).
    * `src/ts/app.ts`: Main application logic.
    * `src/ts/apiClient.ts`: Handles communication with the backend API.
    * `src/ts/authClient.ts`: Manages Supabase authentication.
    * `src/ts/graph-viz.ts`: Logic for the Cytoscape.js graph.
    * `src/ts/webSocketClient.ts`: Manages the WebSocket connection.
* `backend/`: Contains all server-side Go code.
    * `cmd/`: Entry points for the different Lambda functions (`main`, `connect-node`, `ws-*`).
    * `internal/domain/`: Core data structures (`Node`, `Edge`, `Graph`).
    * `internal/repository/`: Data access layer interface and its DynamoDB implementation (`ddb/`).
    * `internal/service/memory/`: Business logic (keyword extraction, graph operations).
* `infra/`: Contains the AWS CDK code.
    * `lib/b2-stack.ts`: The main CDK stack definition.
* `lambda/authorizer/`: The TypeScript code for the JWT Lambda authorizer.
* `openapi.yaml`: The [OpenAPI 3.0 specification](https://swagger.io/specification/) for the REST API. This is used to generate type-safe clients.

## 4. Core Data Flow: Creating a Memory

1.  **Frontend:** User enters content and clicks "Save". `apiClient.ts` sends a `POST /api/nodes` request with the content and JWT.
2.  **API Gateway:** The request hits the HTTP API Gateway.
3.  **Authorizer:** The Lambda authorizer validates the JWT with Supabase. If valid, it passes the request to the main backend Lambda, including the user's ID in the context.
4.  **Backend Lambda (`main`):**
    * Receives the request.
    * Extracts keywords from the content.
    * Saves the new node and its keywords to DynamoDB.
    * Publishes a `NodeCreated` event to EventBridge containing the `nodeId`, `userId`, and `keywords`.
    * Returns a `201 Created` response to the user immediately.
5.  **EventBridge:** Routes the `NodeCreated` event.
6.  **Connect Node Lambda:**
    * Is triggered by the event.
    * Queries DynamoDB (using the GSI) to find other nodes with matching keywords.
    * Creates new `Edge` items in DynamoDB for each match.
    * Publishes an `EdgesCreated` event to EventBridge.
7.  **WebSocket Send Lambda:**
    * Is triggered by the `EdgesCreated` event.
    * Looks up the user's active `WebSocket connectionId` from the connections DynamoDB table.
    * Sends a `{"action": "graphUpdated"}` message to the user's browser via the WebSocket API Gateway.
8.  **Frontend:** `webSocketClient.ts` receives the message and dispatches a custom event. `app.ts` listens for this event and calls `graph-viz.ts` to refresh the graph, showing the new connections.