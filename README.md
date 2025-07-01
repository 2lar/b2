# Brain2 - Your Second Brain

A graph-based personal knowledge management system that automatically connects your memories, thoughts, and ideas based on their content. Built with modern cloud technologies using AWS free tier services.

---

## 📖 **Educational Guide: Understanding the System Architecture**

> **For Learning Purposes**: This codebase has been extensively documented with educational comments to help you understand modern full-stack development, serverless architecture, and real-time web applications.

### 🎯 **How to Navigate This Codebase for Maximum Learning**

This system demonstrates many advanced concepts in software engineering. Here's the recommended order to explore the code for educational purposes:

#### **Phase 1: Understanding the Big Picture (Start Here)**

1. **📋 System Overview** 
   - Read the [Architecture](#-architecture) section below
   - Study the system diagram to understand data flow
   - Review the technology stack and understand why each tool was chosen

2. **📁 Project Structure**
   ```
   /b2/
   ├── frontend/          # TypeScript SPA with graph visualization
   ├── backend/           # Go serverless functions  
   ├── infra/             # AWS CDK infrastructure as code
   ├── openapi.yaml       # API specification (single source of truth)
   └── build.sh           # Master build orchestration
   ```

#### **Phase 2: Core Architecture Patterns (Foundational Concepts)**

3. **🏗️ Infrastructure as Code** → Start with `infra/lib/b2-stack.ts`
   - **Why start here?** Understand the cloud architecture before diving into code
   - **Key concepts**: Serverless architecture, event-driven systems, CDN, databases
   - **Learning focus**: How modern cloud applications are structured

4. **🎯 Domain-Driven Design** → Read `backend/internal/domain/node.go`
   - **Why important?** See how business concepts are modeled in code
   - **Key concepts**: Domain entities, clean architecture, data modeling
   - **Learning focus**: Separating business logic from infrastructure

#### **Phase 3: Business Logic and Data Flow (How It Works)**

5. **⚙️ Service Layer Patterns** → Study `backend/internal/service/memory/service.go`
   - **Core algorithms**: Keyword extraction, connection discovery, graph building
   - **Key concepts**: Service patterns, natural language processing, error handling
   - **Learning focus**: How complex business workflows are orchestrated

6. **🔗 API Design** → Follow the OpenAPI-first approach
   - Start with `openapi.yaml` to understand the API contract
   - See how types are generated for both frontend and backend
   - **Key concepts**: API-first development, type safety, contract-driven development

#### **Phase 4: Frontend Architecture (User Experience)**

7. **🎮 Application Controller** → Explore `frontend/src/ts/app.ts`
   - **Why crucial?** This orchestrates the entire user experience
   - **Key concepts**: Event delegation, state management, real-time updates
   - **Learning focus**: Modern JavaScript patterns and user interaction design

8. **🌐 API Communication** → Review `frontend/src/ts/apiClient.ts`
   - **Integration patterns**: How frontend talks to backend securely
   - **Key concepts**: HTTP clients, authentication, error handling
   - **Learning focus**: Type-safe API communication

9. **🔐 Authentication Flow** → Study `frontend/src/ts/authClient.ts`
   - **Security patterns**: JWT tokens, session management, secure storage
   - **Key concepts**: OAuth flows, token refresh, user state management
   - **Learning focus**: Modern web authentication

#### **Phase 5: Advanced Features (Complex Interactions)**

10. **📡 Real-Time Communication** → Examine `frontend/src/ts/webSocketClient.ts`
    - **Real-time patterns**: WebSocket management, automatic reconnection
    - **Key concepts**: Event-driven updates, connection resilience
    - **Learning focus**: Building responsive, real-time applications

11. **📊 Data Visualization** → Deep dive into `frontend/src/ts/graph-viz.ts`
    - **Visualization algorithms**: Force-directed layouts, graph theory, performance optimization
    - **Key concepts**: Interactive graphics, large dataset handling, user experience
    - **Learning focus**: Advanced frontend visualization techniques

#### **Phase 6: DevOps and Deployment (Production Readiness)**

12. **🚀 Build Orchestration** → Study `build.sh`
    - **Multi-language builds**: Coordinating Go, TypeScript, and infrastructure
    - **Key concepts**: Build automation, dependency management, error handling
    - **Learning focus**: Modern DevOps practices

### 💡 **Key Learning Themes Throughout the Codebase**

**🏛️ Architectural Patterns:**
- Clean Architecture (separation of concerns)
- Domain-Driven Design (business-focused modeling)
- Event-Driven Architecture (loose coupling)
- Serverless Patterns (stateless, scalable functions)

**⚡ Performance Optimization:**
- Database query optimization (single-table design)
- Frontend performance (batching, caching, lazy loading)
- Network optimization (CDN, compression, bundling)
- Real-time efficiency (WebSocket management)

**🔒 Security Best Practices:**
- Authentication and authorization (JWT, user isolation)
- Input validation and sanitization (XSS prevention)
- Secure communication (HTTPS, WSS)
- Infrastructure security (IAM, VPC, encryption)

**🧠 Advanced Algorithms:**
- Natural Language Processing (keyword extraction, stop words)
- Graph Theory (force-directed layouts, connection algorithms)
- Real-time Systems (event propagation, state synchronization)

### 🎓 **Suggested Learning Path by Experience Level**

**🟢 Beginner (New to Web Development):**
1. Start with the system overview and basic concepts
2. Focus on `frontend/src/ts/app.ts` for JavaScript patterns
3. Study `frontend/src/ts/apiClient.ts` for HTTP communication
4. Review the build process in `build.sh`

**🟡 Intermediate (Some Full-Stack Experience):**
1. Begin with the infrastructure (`infra/lib/b2-stack.ts`)
2. Study the service layer (`backend/internal/service/memory/service.go`) 
3. Explore real-time features (`frontend/src/ts/webSocketClient.ts`)
4. Understand the data modeling (`backend/internal/domain/node.go`)

**🔴 Advanced (Experienced Developer):**
1. Analyze the complete architecture from infrastructure to frontend
2. Focus on the advanced algorithms (NLP, graph visualization)
3. Study the performance optimizations throughout
4. Examine the security patterns and deployment strategies

### 📚 **Additional Resources for Context**

- **Clean Architecture**: Robert C. Martin's book on software architecture
- **Domain-Driven Design**: Eric Evans' approach to complex software
- **AWS Well-Architected Framework**: Best practices for cloud architecture
- **Graph Theory**: Understanding network visualization and algorithms
- **Real-Time Web Applications**: WebSocket and event-driven patterns

---

## 🧠 **Features**

- **Automatic Memory Connections**: Write a memory, and the system automatically connects it to related memories using keyword extraction
- **Interactive Knowledge Graph**: Visualize all your memories as an interactive graph showing connections
- **Secure & Private**: Each user's data is completely isolated with JWT-based authentication
- **Real-time Updates**: Graph updates instantly as you add new memories
- **Scalable Architecture**: Built on AWS serverless technologies for automatic scaling

## 🏗️ Architecture

### Technology Stack

- **Frontend**: Vanilla JavaScript (ES6+), HTML5, CSS3, Cytoscape.js
- **Authentication**: Supabase Auth (JWT provider)
- **Backend**: Go on AWS Lambda
- **Database**: AWS DynamoDB (Single-table design)
- **API**: AWS API Gateway (HTTP API)
- **Hosting**: AWS S3 + CloudFront
- **Infrastructure**: AWS CDK (TypeScript)
- **CI/CD**: GitHub Actions

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

🔄 Data Flow:
1. User creates memory → HTTP API → Memory Lambda → DynamoDB
2. Lambda triggers EventBridge → Keyword extraction → Connection discovery  
3. New connections → WebSocket Lambda → Real-time graph updates
4. Client receives live updates → Graph visualization refreshes automatically

🏗️ Key Architectural Patterns:
• Event-Driven: Asynchronous processing via EventBridge
• Real-time: WebSocket connections for live graph updates  
• Serverless: Auto-scaling Lambda functions
• Single-table: Efficient DynamoDB design with GSI
• Clean Architecture: Domain-driven design with service layers
```

## 🚀 Setup Instructions

### Prerequisites

- AWS Account (free tier eligible)
- Supabase Account (free tier)
- Node.js 20+ and npm
- Go 1.21+
- AWS CLI configured
- AWS CDK CLI (`npm install -g aws-cdk`)

### 1. Clone the Repository

```bash
git clone https://github.com/2lar/b2.git
cd b2
```

### 2. Set Up Supabase

1. Create a new project at [supabase.com](https://supabase.com)
2. Go to Settings → API
3. Copy your Project URL and `anon` public key
4. Update `frontend/js/auth.js`:
   ```javascript
   const SUPABASE_URL = 'your-project-url';
   const SUPABASE_ANON_KEY = 'your-anon-key';
   ```

### 3. Configure AWS CDK

1. Update the JWT issuer URL in `infra/lib/b2-stack.ts`:
   ```typescript
   const authorizer = new HttpJwtAuthorizer(
     'SupabaseJwtAuthorizer',
     'https://YOUR_PROJECT_REF.supabase.co/auth/v1',
     // ...
   );
   ```

2. Bootstrap CDK (first time only):
   ```bash
   cd infra
   npm install
   cdk bootstrap
   ```

---

Backend : oapi-codegen -generate types -package api -o backend/pkg/api/generated-api.go openapi.yaml
frontend : npm run generate-api-types

# 1. Build Backend
echo "Building backend..."
cd backend
go mod tidy (it's like the npm install for go)
./build.sh
cd ..

# 2. Build Frontend
echo "Building frontend..."
cd frontend
-- you can run npm run clean but build already does it, as well as install --
npm run build
cd ..

# 3. Build Authorizer & Deploy Infrastructure
echo "Building authorizer and deploying infrastructure..."
cd infra
npm install
(cd lambda/authorizer && npm run build)
cdk deploy

---

### 4. Deploy the Backend

```bash
# Build the Go Lambda function
cd backend
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o bootstrap main.go node_operations.go graph_operations.go
mkdir -p build
zip build/function.zip bootstrap

# Deploy with CDK
cd ../infra
npm run build
cdk deploy
```

After deployment, note the outputs:
- `ApiUrl`: Your API Gateway endpoint
- `CloudFrontUrl`: Your frontend URL

### 5. Configure and Deploy Frontend

1. Update `frontend/js/api.js` with your API Gateway URL:
   ```javascript
   const API_BASE_URL = 'your-api-gateway-url';
   ```

2. The frontend is automatically deployed with the CDK stack

### 6. Set Up GitHub Actions (Optional)

For automated deployments:

1. Create an IAM role for GitHub Actions with necessary permissions
2. Add these secrets to your GitHub repository:
   - `AWS_DEPLOY_ROLE_ARN`: ARN of the IAM role
   - `AWS_REGION`: Your AWS region (e.g., us-east-1)
   - `SUPABASE_URL`: Your Supabase project URL (optional)
   - `SUPABASE_ANON_KEY`: Your Supabase anon key (optional)


lol don't do the above !!!!!!!!!!!!!!!!!!!!!!!!!!!!

## 📖 Usage

1. **Sign Up/Sign In**: Create an account or sign in with your email
2. **Create Memories**: Write any thought, idea, or memory in the text area
3. **Automatic Connections**: The system extracts keywords and connects related memories
4. **Explore Your Graph**: View the interactive graph showing all connections
5. **Click Nodes**: Click on any node in the graph to see details and connections

## 🔧 Development

### Local Development

1. **Frontend**: Open `frontend/index.html` in a browser (use a local server for modules)
2. **Backend**: Use SAM CLI for local Lambda testing
3. **Infrastructure**: Use `cdk diff` to preview changes before deploying

### Project Structure

```
/b2/
├── frontend/          # Static frontend files
├── backend/           # Go Lambda function
├── infra/            # AWS CDK infrastructure
└── .github/          # CI/CD workflows
```

## 🎯 Key Features Implementation

### Natural Language Processing & Keyword Extraction

The system implements an advanced NLP pipeline for automatic memory connections:

**Algorithm Pipeline:**
1. **Text Normalization**: Convert to lowercase, remove punctuation
2. **Tokenization**: Smart word boundary detection with regex
3. **Stop Word Filtering**: Remove 176+ common English words (articles, pronouns, etc.)
4. **Length Filtering**: Eliminate words < 3 characters
5. **Deduplication**: Create unique keyword sets for efficient storage
6. **Connection Discovery**: Find memories sharing 1+ keywords
7. **Bidirectional Edge Creation**: Maintain graph consistency

**Future Enhancements Ready:**
- TF-IDF scoring for keyword importance
- Word embeddings (BERT) for semantic similarity  
- Named entity recognition
- Domain-specific vocabulary learning

### Real-Time Graph Visualization

**Advanced Cytoscape.js Implementation:**
- **Force-Directed Layout**: COSE algorithm with physics-based positioning
- **Performance Optimizations**: Viewport culling, texture rendering, motion blur
- **Smart Initial Positioning**: Connectivity-based node placement to prevent clustering
- **Interactive Features**: Node selection, connection highlighting, smooth animations
- **Responsive Design**: Dynamic viewport adaptation and zoom controls

**Layout Algorithm:**
```javascript
// Connectivity-based positioning prevents visual chaos
nodesByConnectivity.forEach((node, index) => {
    const connectivity = adjacency.get(node.data.id)?.size || 0;
    const radiusMultiplier = connectivity > 3 ? 0.7 : (connectivity > 1 ? 0.85 : 1);
    // Hubs positioned closer to center for better visual hierarchy
});
```

### Event-Driven Architecture

**Asynchronous Processing Pipeline:**
1. **Immediate Response**: User gets instant feedback via HTTP API
2. **Background Processing**: EventBridge triggers keyword extraction
3. **Connection Discovery**: Parallel processing finds related memories  
4. **Real-time Updates**: WebSocket pushes graph changes to all clients
5. **Optimistic UI**: Frontend updates immediately, syncs with backend

**Benefits:**
- **Scalability**: Handle thousands of concurrent users
- **Responsiveness**: No blocking operations in user workflow
- **Resilience**: Graceful degradation if background processing fails
- **Consistency**: Eventually consistent with real-time synchronization

### Single-Table DynamoDB Design

**Optimized for Graph Operations:**

```
PK (Partition Key) Examples:
- USER#123#NODE#abc-def     → Node metadata
- USER#123#KEYWORD#machine  → Keyword index  
- USER#123#CONNECTION#xyz   → WebSocket connections

SK (Sort Key) Examples:  
- METADATA#v1               → Node content & timestamps
- EDGE#RELATES_TO#NODE#xyz  → Graph relationships
- KEYWORD#learning          → Searchable terms
- CONNECTION#session-id     → Active WebSocket sessions
```

**Global Secondary Index (GSI):**
- **KeywordIndex**: Enables fast keyword-based memory discovery
- **UserIndex**: Efficient user data isolation and queries
- **EdgeIndex**: Quick relationship traversal for graph operations

**Performance Benefits:**
- **Single Query**: Retrieve node + edges + keywords in one request
- **Hot Partitions**: Even distribution prevents throttling
- **Cost Efficient**: On-demand billing scales with actual usage

## 🔒 Security

- JWT-based authentication with Supabase
- User data isolation at the database level
- HTTPS everywhere (CloudFront + API Gateway)
- No cross-user data access possible

## 💰 Cost Optimization

Designed for AWS free tier:
- DynamoDB: On-demand billing
- Lambda: 1M free requests/month
- API Gateway: 1M free API calls/month
- S3 & CloudFront: Minimal storage and transfer costs

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Commit your changes
4. Push to the branch
5. Open a Pull Request

## 📄 License

MIT License - feel free to use this project for personal or commercial purposes.

## 🆘 Troubleshooting

### Common Issues

1. **CORS Errors**: Ensure CloudFront URL is in API Gateway CORS settings
2. **Auth Failures**: Verify Supabase URL and keys are correct
3. **Graph Not Loading**: Check browser console for API errors
4. **Deployment Fails**: Ensure AWS credentials are configured correctly

### Support

Open an issue on GitHub for bugs or feature requests.