# Brain2 - Your Second Brain

A graph-based personal knowledge management system that automatically connects your memories, thoughts, and ideas based on their content. Built with modern cloud technologies using AWS free tier services.

## Architecture

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
```

## Prerequisites

- AWS Account (free tier eligible)
- Supabase Account (free tier)
- Node.js 20+ and npm
- Go 1.21+
- AWS CLI configured
- AWS CDK CLI (`npm install -g aws-cdk`)

## Setup

1. Clone repository
2. Set up Supabase project and update frontend configuration
3. Update CDK stack with Supabase JWT issuer URL
4. Deploy using CDK

## Features

- **Automatic Memory Connections**: Write a memory, and the system automatically connects it to related memories using keyword extraction
- **Interactive Knowledge Graph**: Visualize all your memories as an interactive graph showing connections
- **Secure & Private**: Each user's data is completely isolated with JWT-based authentication
- **Real-time Updates**: Graph updates instantly as you add new memories
- **Scalable Architecture**: Built on AWS serverless technologies for automatic scaling

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

MIT License - feel free to use this project for personal or commercial purposes.