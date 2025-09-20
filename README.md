# Brain2 - Your Second Brain

A **graph-based personal knowledge management system** that automatically connects your memories, thoughts, and ideas based on their content.
Built with a **modern, event-driven, serverless architecture** on AWS.

---

## ✨ Features

* **Automatic Memory Connections**: Nodes (memories) are connected via keyword and similarity analysis using NLP services.
* **Interactive Knowledge Graph**: Visualize your memories as an interactive graph powered by Cytoscape.js.
* **Secure & Private**: JWT authentication powered by Supabase, per-user isolation, distributed rate limiting.
* **Real-time Collaboration**: Live graph updates via WebSockets.
* **Scalable Serverless Backend**: Go services on AWS Lambda with DynamoDB single-table design.
* **Bulk Operations & Editing**: Efficiently delete or update multiple memories at once.
* **Monitoring & Tracing**: CloudWatch metrics, AWS X-Ray distributed tracing, and structured logging.
* **CI/CD**: Automated pipelines for backend and frontend with GitHub Actions.

---

## 🏗️ Architecture Overview

Brain2 follows **Domain-Driven Design (DDD)** and **CQRS (Command Query Responsibility Segregation)** principles.

### Frontend

* React 19 + TypeScript + Vite
* Zustand for state management
* TanStack Query for data fetching
* Cytoscape.js for graph visualization
* Framer Motion for animations
* Supabase client for authentication

### Backend (Go)

* Organized by layers: `application`, `domain`, `infrastructure`, `interfaces`
* **CQRS**: commands (create, update, delete) vs queries (fetch, list)
* **Domain models**: Graph, Node, Edge as entities and value objects
* **Event-driven design**: application events, sagas, projections
* **Security**: JWT validation and distributed rate limiting via DynamoDB
* **Persistence**: repository pattern for DynamoDB
* **Real-time**: WebSocket Hub for live updates
* **Observability**: CloudWatch metrics, AWS X-Ray tracing

### Infrastructure (AWS CDK - TypeScript)

* **Stacks**:

  * Database: DynamoDB (memories, connections)
  * Compute: Lambda functions for API, WebSockets, cleanup, background workers
  * API: API Gateway (HTTP + WebSocket)
  * Frontend: S3 + CloudFront
  * Monitoring: CloudWatch dashboards, alarms, X-Ray integration
* **Testing**: Unit tests for CDK constructs and stacks

### CI/CD

* **Backend**: Build, lint, test (unit + integration), deploy to AWS Lambda
* **Frontend**: Build, test (planned), deploy to S3 + CloudFront
* **Infrastructure**: CDK synth + deploy

---

## 📂 Repository Layout

```
b2-main/
├── backend/      # Go backend (CQRS, domain, infra, interfaces)
├── frontend/     # React + Vite frontend (SPA)
├── infra/        # AWS CDK stacks (TypeScript)
├── docs/         # Architecture, design notes, evaluations
├── scripts/      # Build & environment helpers
├── .github/      # CI/CD workflows
└── openapi.yaml  # API specification
```

---

## 🌐 System Design (High-Level)

### Overview

Brain2 is built as a **serverless event-driven system** with clear separation between frontend, backend services, and infrastructure. It leverages AWS managed services for scalability and real-time updates.

```
                           🌐 Brain2 - Event-Driven Architecture

┌─────────────┐     ┌──────────────┐     ┌─────────────┐          
│  CloudFront │────▶│      S3      │     │  Supabase   │          
│    (CDN)    │     │  (Frontend)  │     │    Auth     │          
└─────────────┘     └──────────────┘     └──────┬───────┘          
       │                                         │ JWT              
       │            📡 Real-time Updates         │                  
       ▼                                         ▼                  
┌──────────────────────────────────────────────────────────────────┐
│                          API Gateway                              │
│                (HTTP + WebSocket APIs w/ JWT)                     │
└──────────────────────────────────────────────────────────────────┘
       │                          │                                
       ▼                          ▼                                
┌───────────────┐         ┌───────────────┐                        
│   HTTP Routes │         │  WebSocket Hub │                        
│   (REST API)  │         │ (Connections)  │                        
└───────────────┘         └───────────────┘                        
       │                          │                                
       ▼                          ▼                                
┌──────────────────┐       ┌────────────────────┐                   
│  Backend Lambdas │       │   WS Lambdas       │                   
│ (Go CQRS Handlers│       │ (Connect, Message, │                   
│  Commands/Queries)│       │  Disconnect)       │                   
└──────────────────┘       └────────────────────┘                   
       │                                                          
       ▼                                                          
┌───────────────────────────────────────────┐                     
│                DynamoDB                   │                     
│   (Memories, Nodes, Edges, Connections)   │                     
└───────────────────────────────────────────┘                     
       │                                                          
       ▼                                                          
┌───────────────────────────────────────────┐                     
│           CloudWatch & X-Ray              │                     
│  (Metrics, Logs, Distributed Tracing)     │                     
└───────────────────────────────────────────┘                     
```

### Key Flows

1. **User Authentication** → Supabase issues JWT → API Gateway verifies JWT.
2. **REST Request** → Routed to Lambda (Go handler) → Command/Query → Domain → DynamoDB.
3. **WebSocket Connection** → User connects via API Gateway WS → Hub registers client → Events broadcast in real time.
4. **Persistence** → Nodes/Edges stored in DynamoDB with single-table design + GSIs.
5. **Observability** → Metrics sent to CloudWatch; traces recorded in X-Ray.

---

## 🚀 Quickstart

### Prerequisites

* Node.js 20+
* Go 1.23+
* AWS CLI configured
* Supabase project (for auth)

### 1. Clone & Configure Environment

```bash
git clone https://github.com/your-org/b2-main.git
cd b2-main
cp .env.example .env
```

Edit `.env` with your AWS and Supabase keys. See [docs/ENVIRONMENT\_SETUP.md](docs/ENVIRONMENT_SETUP.md).

### 2. Build All Components

```bash
./build.sh
```

Or build individually:

```bash
cd frontend && npm run dev
cd backend && ./run-local.sh
cd infra && npm run deploy
```

### 3. Running Locally

* **Backend**: `./backend/run-local.sh` → runs API locally on port 8080.
* **Frontend**: `cd frontend && npm run dev` → runs Vite dev server.
* **Infrastructure**: `cd infra && npm run deploy` → deploy to AWS dev environment.

---

## 📖 Documentation

* [docs/ENVIRONMENT\_SETUP.md](docs/ENVIRONMENT_SETUP.md) → Environment setup
* [docs/backend-architecture-plan.md](docs/backend-architecture-plan.md) → Backend design & CQRS patterns
* [docs/domain-model-design.md](docs/domain-model-design.md) → Domain models (Node, Graph, Edge)
* [docs/performance-optimized-architecture.md](docs/performance-optimized-architecture.md) → Performance tuning
* [docs/plans/](docs/plans/) → Historical architecture plans and improvements

Component READMEs:

* [frontend/README.md](frontend/README.md)
* [backend/README.md](backend/README.md)
* [infra/README.md](infra/README.md)

---

## 🧪 Testing

* **Backend (Go)**:

```bash
cd backend
go test ./...
```

* **Infra (CDK)**:

```bash
cd infra
npm test
```

* **Frontend (React)**:
  Tests are planned. Suggested tools: Vitest + React Testing Library + Playwright.

---

## 🧭 Learning Roadmap (for contributors)

1. Start with the **Frontend** → run locally, explore graph visualization.
2. Study the **Backend HTTP handlers** → see how requests map to commands/queries.
3. Dive into **Domain Models** → understand Node, Graph, Edge.
4. Explore **Infrastructure** → CDK stacks and how deployment works.
5. Learn advanced patterns → mediator, sagas, projections, observability.

---

## 📌 Status

* ✅ Stable backend and infrastructure
* ⚠️ Frontend tests missing
* ⚙️ Documentation being expanded

---

## 📜 License

MIT
