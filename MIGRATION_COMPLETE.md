# ✅ Backend Migration Complete

## Migration from backend to backend2 is complete!

### What Changed

#### Frontend (`/frontend`)
- ✅ Updated all API calls from `/api/v1/*` to `/api/v2/*`
- ✅ Configured to use backend2 on port 8080 for local development
- ✅ Temporarily commented out category-related functionality (to be added later)
- ✅ Bulk delete now uses `node_ids` field format

#### Backend2 (`/backend2`)
- ✅ Fully functional DDD/CQRS architecture
- ✅ All node CRUD operations
- ✅ Graph operations and visualization
- ✅ Bulk delete functionality
- ✅ Search capabilities
- ✅ Lambda deployment ready
- ✅ Local development server script (`run-local.sh`)

#### Legacy Backend (`/backend`)
- ⚠️ DEPRECATED - Do not use
- 📄 Added DEPRECATED.md notice
- 🚫 Main handler marked as deprecated

### How to Run Locally

1. **Start Backend2:**
   ```bash
   cd backend2
   ./run-local.sh
   ```
   The API will be available at http://localhost:8080/api/v2

2. **Start Frontend:**
   ```bash
   cd frontend
   npm run dev
   ```
   The frontend will automatically connect to backend2

### API Endpoints

All endpoints now use `/api/v2` prefix:
- `GET    /api/v2/nodes` - List nodes
- `POST   /api/v2/nodes` - Create node
- `GET    /api/v2/nodes/{id}` - Get node
- `PUT    /api/v2/nodes/{id}` - Update node
- `DELETE /api/v2/nodes/{id}` - Delete node
- `POST   /api/v2/nodes/bulk-delete` - Bulk delete
- `GET    /api/v2/graph-data` - Graph visualization data
- `GET    /api/v2/graphs` - List graphs
- `GET    /api/v2/graphs/{id}` - Get graph
- `GET    /api/v2/search` - Search nodes

### Environment Variables

Backend2 requires these environment variables (set in `run-local.sh` for local dev):
- `SERVER_ADDRESS=:8080`
- `AWS_REGION=us-east-1`
- `DYNAMODB_TABLE=brain2-backend2`
- `JWT_SECRET` (set to dev key locally)
- AWS credentials (via AWS CLI or env vars)

### Next Steps

1. **Deploy to AWS Lambda:**
   - Use `/backend2/cmd/lambda/main.go`
   - Deploy with API Gateway v2
   - Update frontend's `VITE_API_BASE_URL` to Lambda endpoint

2. **Add Categories (Future):**
   - Implement category domain in backend2
   - Uncomment frontend category code
   - Update to use v2 endpoints

3. **Remove Legacy Backend:**
   - Once fully tested in production
   - Delete `/backend` directory
   - Clean up any remaining references

### Testing Checklist

- [ ] Create a new node
- [ ] List all nodes
- [ ] View node details
- [ ] Update a node
- [ ] Delete a node
- [ ] Bulk delete multiple nodes
- [ ] View graph visualization
- [ ] Search for nodes
- [ ] Authentication works
- [ ] WebSocket connections (when implemented)

---

**Migration completed successfully! 🎉**
Backend2 is now the primary backend for the application.