# Brain2- Your Second Brain

A graph-based personal knowledge management system that automatically connects your memories, thoughts, and ideas based on their content. Built with modern cloud technologies using AWS free tier services.

## 🧠 Features

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
┌─────────────┐     ┌──────────────┐     ┌─────────────┐
│  CloudFront │────▶│      S3      │     │  Supabase   │
│    (CDN)    │     │  (Frontend)  │     │    Auth     │
└─────────────┘     └──────────────┘     └──────┬───────┘
                                                 │ JWT
┌─────────────┐     ┌──────────────┐     ┌──────▼──────┐
│   Client    │────▶│ API Gateway  │────▶│   Lambda    │
│  (Browser)  │     │ (JWT Auth)   │     │    (Go)     │
└─────────────┘     └──────────────┘     └──────┬───────┘
                                                 │
                                         ┌───────▼───────┐
                                         │   DynamoDB    │
                                         │ (Graph Data)  │
                                         └───────────────┘
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

### Keyword Extraction Algorithm

The system uses a simple but effective algorithm:
1. Convert content to lowercase
2. Remove punctuation and special characters
3. Split into words
4. Remove common stop words
5. Create unique keyword set
6. Find all nodes sharing keywords
7. Create bidirectional edges

### Graph Visualization

- Uses Cytoscape.js with force-directed layout
- Real-time updates when new memories are added
- Interactive nodes with click-to-view details
- Automatic layout optimization

### Data Model

Single-table DynamoDB design:
- **PK**: `USER#{userId}#NODE#{nodeId}`
- **SK**: `METADATA#v{version}` | `EDGE#RELATES_TO#NODE#{nodeId}` | `KEYWORD#{keyword}`
- **GSI**: KeywordIndex for efficient keyword searches

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