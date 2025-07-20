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