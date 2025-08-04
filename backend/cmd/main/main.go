// Brain2 Main HTTP API - RESTful Memory Management Server
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"brain2-backend/internal/domain"
	infraDynamoDB "brain2-backend/infrastructure/dynamodb"
	"brain2-backend/internal/service/memory"
	"brain2-backend/internal/service/category"
	"brain2-backend/internal/service/llm"
	"brain2-backend/pkg/api"
	"brain2-backend/pkg/config"
	appErrors "brain2-backend/pkg/errors"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge/types"

	chiadapter "github.com/awslabs/aws-lambda-go-api-proxy/chi"
	"github.com/awslabs/aws-lambda-go-api-proxy/core"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/google/uuid"
)

type contextKey struct {
	name string
}

var userIDKey = contextKey{"userID"}
var chiLambda *chiadapter.ChiLambdaV2
var memoryService memory.Service
var categoryService category.Service
var llmService *llm.Service
var eventbridgeClient *eventbridge.Client

func init() {
	cfg := config.New()
	
	awsCfg, err := awsConfig.LoadDefaultConfig(context.TODO(), awsConfig.WithRegion(cfg.Region))
	if err != nil {
		log.Fatalf("unable to load SDK config: %v", err)
	}
	
	dbClient := dynamodb.NewFromConfig(awsCfg)
	eventbridgeClient = eventbridge.NewFromConfig(awsCfg)
	
	repo := infraDynamoDB.NewRepository(dbClient, cfg.TableName, cfg.KeywordIndexName)
	memoryService = memory.NewService(repo)
	
	// Initialize LLM service with mock provider for now
	llmService = llm.NewService(llm.NewMockProvider())
	
	// Use enhanced category service with LLM integration
	categoryService = category.NewEnhancedService(repo, llmService)
	
	r := chi.NewRouter()
	
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"}, // SECURITY: Consider restricting in production
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		AllowCredentials: true,
	}))
	
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	
	r.Route("/api", func(r chi.Router) {
		r.Use(Authenticator)
		
		r.Get("/nodes", listNodesHandler)
		r.Post("/nodes", createNodeHandler)
		r.Get("/nodes/{nodeId}", getNodeHandler)
		r.Put("/nodes/{nodeId}", updateNodeHandler)
		r.Delete("/nodes/{nodeId}", deleteNodeHandler)
		
		r.Post("/nodes/bulk-delete", bulkDeleteNodesHandler)
		
		r.Get("/graph-data", getGraphDataHandler)
		
		// Category routes
		r.Get("/categories", listCategoriesHandler)
		r.Post("/categories", createCategoryHandler)
		r.Get("/categories/{categoryId}", getCategoryHandler)
		r.Put("/categories/{categoryId}", updateCategoryHandler)
		r.Delete("/categories/{categoryId}", deleteCategoryHandler)
		
		// Category-memory association routes
		r.Post("/categories/{categoryId}/memories", addMemoryToCategoryHandler)
		r.Get("/categories/{categoryId}/memories", getMemoriesInCategoryHandler)
		r.Delete("/categories/{categoryId}/memories/{memoryId}", removeMemoryFromCategoryHandler)
		
		// Enhanced category routes
		r.Get("/categories/hierarchy", getCategoryHierarchyHandler)
		r.Post("/categories/suggest", suggestCategoriesHandler)
		r.Post("/categories/rebuild", rebuildCategoriesHandler)
		r.Get("/categories/insights", getCategoryInsightsHandler)
		
		// Node categorization routes
		r.Get("/nodes/{nodeId}/categories", getNodeCategoriesHandler)
		r.Post("/nodes/{nodeId}/categories", categorizeNodeHandler)
	})
	
	chiLambda = chiadapter.NewV2(r)
	
	log.Println("Service initialized successfully")
}

func Authenticator(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxyCtx, ok := core.GetAPIGatewayV2ContextFromContext(r.Context())
		if !ok {
			log.Println("Error: could not get proxy request context from context")
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		userID, ok := proxyCtx.Authorizer.Lambda["sub"].(string)
		if !ok || userID == "" {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), userIDKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func checkOwnership(ctx context.Context, nodeID string) (*domain.Node, error) {
	userID := ctx.Value(userIDKey).(string)
	node, _, err := memoryService.GetNodeDetails(ctx, userID, nodeID)
	if err != nil {
		// If the underlying error is a "not found" error, we return that.
		if appErrors.IsNotFound(err) {
			return nil, err
		}
		// Otherwise, it's an internal server error.
		return nil, appErrors.NewInternal("failed to verify node ownership", err)
	}

	// This check is redundant if GetNodeDetails is implemented correctly,
	// but it provides an explicit layer of defense-in-depth.
	if node.UserID != userID {
		return nil, appErrors.NewNotFound("node not found") // Obscure the reason for security
	}

	return node, nil
}

func createNodeHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)
	var req api.CreateNodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Content == "" {
		api.Error(w, http.StatusBadRequest, "Content cannot be empty")
		return
	}

	// Create the node object
	tags := []string{}
	if req.Tags != nil {
		tags = *req.Tags
	}
	node := domain.Node{
		ID:        uuid.New().String(),
		UserID:    userID,
		Content:   req.Content,
		Keywords:  memory.ExtractKeywords(req.Content),
		Tags:      tags,
		CreatedAt: time.Now(),
		Version:   0,
	}

	// Save the node and its keywords to DynamoDB
	if err := memoryService.CreateNodeAndKeywords(r.Context(), node); err != nil {
		handleServiceError(w, err)
		return
	}

	// Publish "NodeCreated" event to EventBridge
	eventDetail, err := json.Marshal(map[string]interface{}{
		"userId":   node.UserID,
		"nodeId":   node.ID,
		"content":  node.Content,
		"keywords": node.Keywords,
	})
	if err != nil {
		handleServiceError(w, err)
		return
	}

	_, err = eventbridgeClient.PutEvents(r.Context(), &eventbridge.PutEventsInput{
		Entries: []types.PutEventsRequestEntry{
			{
				Source:       aws.String("brain2.api"),
				DetailType:   aws.String("NodeCreated"),
				Detail:       aws.String(string(eventDetail)),
				EventBusName: aws.String("B2EventBus"),
			},
		},
	})
	if err != nil {
		handleServiceError(w, err)
		return
	}

	// Return immediate success to the client
	api.Success(w, http.StatusCreated, api.NodeResponse{
		NodeID:    node.ID,
		Content:   node.Content,
		Tags:      node.Tags,
		Timestamp: node.CreatedAt.Format(time.RFC3339),
		Version:   node.Version,
	})
}

func listNodesHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)
	graph, err := memoryService.GetGraphData(r.Context(), userID)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	var nodesResponse []api.NodeResponse
	for _, node := range graph.Nodes {
		nodesResponse = append(nodesResponse, api.NodeResponse{
			NodeID:    node.ID,
			Content:   node.Content,
			Tags:      node.Tags,
			Timestamp: node.CreatedAt.Format(time.RFC3339),
			Version:   node.Version,
		})
	}
	api.Success(w, http.StatusOK, map[string][]api.NodeResponse{"nodes": nodesResponse})
}

func getNodeHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)
	nodeID := chi.URLParam(r, "nodeId")

	node, edges, err := memoryService.GetNodeDetails(r.Context(), userID, nodeID)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	edgeIDs := make([]string, len(edges))
	for i, edge := range edges {
		edgeIDs[i] = edge.TargetID
	}

	api.Success(w, http.StatusOK, api.NodeDetailsResponse{
		NodeID:    node.ID,
		Content:   node.Content,
		Tags:      node.Tags,
		Timestamp: node.CreatedAt.Format(time.RFC3339),
		Version:   node.Version,
		Edges:     edgeIDs,
	})
}

func updateNodeHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)
	nodeID := chi.URLParam(r, "nodeId")

	// Verify ownership before proceeding
	_, err := checkOwnership(r.Context(), nodeID)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	var req api.UpdateNodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Add server-side validation
	if len(req.Content) == 0 || len(req.Content) > 5000 {
		api.Error(w, http.StatusBadRequest, "Content must be between 1 and 5000 characters.")
		return
	}

	tags := []string{}
	if req.Tags != nil {
		tags = *req.Tags
	}
	_, err = memoryService.UpdateNode(r.Context(), userID, nodeID, req.Content, tags)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	api.Success(w, http.StatusOK, map[string]string{"message": "Node updated successfully"})
}

func deleteNodeHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)
	nodeID := chi.URLParam(r, "nodeId")

	_, err := checkOwnership(r.Context(), nodeID)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	if err := memoryService.DeleteNode(r.Context(), userID, nodeID); err != nil {
		handleServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func bulkDeleteNodesHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)

	var req api.BulkDeleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if len(req.NodeIds) == 0 {
		api.Error(w, http.StatusBadRequest, "NodeIds cannot be empty")
		return
	}

	if len(req.NodeIds) > 100 {
		api.Error(w, http.StatusBadRequest, "Cannot delete more than 100 nodes at once")
		return
	}

	deletedCount, failedNodeIds, err := memoryService.BulkDeleteNodes(r.Context(), userID, req.NodeIds)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	message := fmt.Sprintf("Successfully deleted %d nodes", deletedCount)
	if len(failedNodeIds) > 0 {
		message += fmt.Sprintf(", failed to delete %d nodes", len(failedNodeIds))
	}

	api.Success(w, http.StatusOK, api.BulkDeleteResponse{
		DeletedCount:  &deletedCount,
		FailedNodeIds: &failedNodeIds,
		Message:       &message,
	})
}

func getGraphDataHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)
	graph, err := memoryService.GetGraphData(r.Context(), userID)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	var elements []api.GraphDataResponse_Elements_Item

	for _, node := range graph.Nodes {
		label := node.Content
		if len(label) > 50 {
			label = label[:47] + "..."
		}

		graphNode := api.GraphNode{
			Data: &api.NodeData{
				Id:    node.ID,
				Label: label,
			},
		}

		var element api.GraphDataResponse_Elements_Item
		if err := element.FromGraphNode(graphNode); err != nil {
			log.Printf("Error converting graph node: %v", err)
			continue
		}
		elements = append(elements, element)
	}

	for _, edge := range graph.Edges {
		edgeID := fmt.Sprintf("%s-%s", edge.SourceID, edge.TargetID)
		graphEdge := api.GraphEdge{
			Data: &api.EdgeData{
				Id:     edgeID,
				Source: edge.SourceID,
				Target: edge.TargetID,
			},
		}

		var element api.GraphDataResponse_Elements_Item
		if err := element.FromGraphEdge(graphEdge); err != nil {
			log.Printf("Error converting graph edge: %v", err)
			continue
		}
		elements = append(elements, element)
	}

	api.Success(w, http.StatusOK, api.GraphDataResponse{Elements: &elements})
}

func handleServiceError(w http.ResponseWriter, err error) {
	if appErrors.IsValidation(err) {
		api.Error(w, http.StatusBadRequest, err.Error())
	} else if appErrors.IsNotFound(err) {
		api.Error(w, http.StatusNotFound, err.Error())
	} else {
		log.Printf("INTERNAL ERROR: %v", err)
		api.Error(w, http.StatusInternalServerError, "An internal error occurred")
	}
}

// Category handlers

func listCategoriesHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)
	categories, err := categoryService.ListCategories(r.Context(), userID)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	type CategoryResponse struct {
		ID          string  `json:"id"`
		Title       string  `json:"title"`
		Description string  `json:"description"`
		Level       int     `json:"level"`
		ParentID    *string `json:"parentId"`
		Color       *string `json:"color"`
		Icon        *string `json:"icon"`
		AIGenerated bool    `json:"aiGenerated"`
		NoteCount   int     `json:"noteCount"`
		CreatedAt   string  `json:"createdAt"`
		UpdatedAt   string  `json:"updatedAt"`
	}

	var categoriesResponse []CategoryResponse
	for _, category := range categories {
		categoriesResponse = append(categoriesResponse, CategoryResponse{
			ID:          category.ID,
			Title:       category.Title,
			Description: category.Description,
			Level:       category.Level,
			ParentID:    category.ParentID,
			Color:       category.Color,
			Icon:        category.Icon,
			AIGenerated: category.AIGenerated,
			NoteCount:   category.NoteCount,
			CreatedAt:   category.CreatedAt.Format(time.RFC3339),
			UpdatedAt:   category.UpdatedAt.Format(time.RFC3339),
		})
	}

	api.Success(w, http.StatusOK, map[string][]CategoryResponse{"categories": categoriesResponse})
}

func createCategoryHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)
	
	type CreateCategoryRequest struct {
		Title       string `json:"title"`
		Description string `json:"description"`
	}

	var req CreateCategoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Title == "" {
		api.Error(w, http.StatusBadRequest, "Title cannot be empty")
		return
	}

	category, err := categoryService.CreateCategory(r.Context(), userID, req.Title, req.Description)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	type CategoryResponse struct {
		ID          string  `json:"id"`
		Title       string  `json:"title"`
		Description string  `json:"description"`
		Level       int     `json:"level"`
		ParentID    *string `json:"parentId"`
		Color       *string `json:"color"`
		Icon        *string `json:"icon"`
		AIGenerated bool    `json:"aiGenerated"`
		NoteCount   int     `json:"noteCount"`
		CreatedAt   string  `json:"createdAt"`
		UpdatedAt   string  `json:"updatedAt"`
	}

	api.Success(w, http.StatusCreated, CategoryResponse{
		ID:          category.ID,
		Title:       category.Title,
		Description: category.Description,
		Level:       category.Level,
		ParentID:    category.ParentID,
		Color:       category.Color,
		Icon:        category.Icon,
		AIGenerated: category.AIGenerated,
		NoteCount:   category.NoteCount,
		CreatedAt:   category.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   category.UpdatedAt.Format(time.RFC3339),
	})
}

func getCategoryHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)
	categoryID := chi.URLParam(r, "categoryId")

	category, err := categoryService.GetCategory(r.Context(), userID, categoryID)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	type CategoryResponse struct {
		ID          string  `json:"id"`
		Title       string  `json:"title"`
		Description string  `json:"description"`
		Level       int     `json:"level"`
		ParentID    *string `json:"parentId"`
		Color       *string `json:"color"`
		Icon        *string `json:"icon"`
		AIGenerated bool    `json:"aiGenerated"`
		NoteCount   int     `json:"noteCount"`
		CreatedAt   string  `json:"createdAt"`
		UpdatedAt   string  `json:"updatedAt"`
	}

	api.Success(w, http.StatusOK, CategoryResponse{
		ID:          category.ID,
		Title:       category.Title,
		Description: category.Description,
		Level:       category.Level,
		ParentID:    category.ParentID,
		Color:       category.Color,
		Icon:        category.Icon,
		AIGenerated: category.AIGenerated,
		NoteCount:   category.NoteCount,
		CreatedAt:   category.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   category.UpdatedAt.Format(time.RFC3339),
	})
}

func updateCategoryHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)
	categoryID := chi.URLParam(r, "categoryId")

	type UpdateCategoryRequest struct {
		Title       string `json:"title"`
		Description string `json:"description"`
	}

	var req UpdateCategoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Title == "" {
		api.Error(w, http.StatusBadRequest, "Title cannot be empty")
		return
	}

	category, err := categoryService.UpdateCategory(r.Context(), userID, categoryID, req.Title, req.Description)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	api.Success(w, http.StatusOK, map[string]interface{}{
		"message":    "Category updated successfully",
		"categoryId": category.ID,
	})
}

func deleteCategoryHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)
	categoryID := chi.URLParam(r, "categoryId")

	if err := categoryService.DeleteCategory(r.Context(), userID, categoryID); err != nil {
		handleServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func addMemoryToCategoryHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)
	categoryID := chi.URLParam(r, "categoryId")

	type AddMemoryRequest struct {
		MemoryID string `json:"memoryId"`
	}

	var req AddMemoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.MemoryID == "" {
		api.Error(w, http.StatusBadRequest, "MemoryID cannot be empty")
		return
	}

	if err := categoryService.AddMemoryToCategory(r.Context(), userID, categoryID, req.MemoryID); err != nil {
		handleServiceError(w, err)
		return
	}

	api.Success(w, http.StatusOK, map[string]string{"message": "Memory added to category successfully"})
}

func getMemoriesInCategoryHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)
	categoryID := chi.URLParam(r, "categoryId")

	memories, err := categoryService.GetMemoriesInCategory(r.Context(), userID, categoryID)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	type MemoryResponse struct {
		NodeID    string   `json:"nodeId"`
		Content   string   `json:"content"`
		Tags      []string `json:"tags"`
		Timestamp string   `json:"timestamp"`
		Version   int      `json:"version"`
	}

	var memoriesResponse []MemoryResponse
	for _, memory := range memories {
		memoriesResponse = append(memoriesResponse, MemoryResponse{
			NodeID:    memory.ID,
			Content:   memory.Content,
			Tags:      memory.Tags,
			Timestamp: memory.CreatedAt.Format(time.RFC3339),
			Version:   memory.Version,
		})
	}

	api.Success(w, http.StatusOK, map[string][]MemoryResponse{"memories": memoriesResponse})
}

func removeMemoryFromCategoryHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)
	categoryID := chi.URLParam(r, "categoryId")
	memoryID := chi.URLParam(r, "memoryId")

	if err := categoryService.RemoveMemoryFromCategory(r.Context(), userID, categoryID, memoryID); err != nil {
		handleServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Enhanced category handlers

func getCategoryHierarchyHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)
	
	// Cast to enhanced service to access hierarchy methods
	enhancedSvc, ok := categoryService.(category.EnhancedService)
	if !ok {
		api.Error(w, http.StatusInternalServerError, "Enhanced category features not available")
		return
	}
	
	// Get hierarchical category tree
	categories, err := enhancedSvc.GetCategoryTree(r.Context(), userID)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	// Build hierarchy map
	hierarchy := make(map[string][]string)
	for _, cat := range categories {
		if cat.ParentID != nil {
			parentID := *cat.ParentID
			if hierarchy[parentID] == nil {
				hierarchy[parentID] = []string{}
			}
			hierarchy[parentID] = append(hierarchy[parentID], cat.ID)
		}
	}

	api.Success(w, http.StatusOK, map[string]interface{}{
		"categories": categories,
		"hierarchy":  hierarchy,
	})
}

func suggestCategoriesHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)
	
	type SuggestCategoriesRequest struct {
		Content string `json:"content"`
	}

	var req SuggestCategoriesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Content == "" {
		api.Error(w, http.StatusBadRequest, "Content cannot be empty")
		return
	}

	// Cast to enhanced service
	enhancedSvc, ok := categoryService.(category.EnhancedService)
	if !ok {
		api.Error(w, http.StatusInternalServerError, "Enhanced category features not available")
		return
	}

	suggestions, err := enhancedSvc.SuggestCategories(r.Context(), req.Content, userID)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	api.Success(w, http.StatusOK, map[string]interface{}{"suggestions": suggestions})
}

func rebuildCategoriesHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)
	
	// Cast to enhanced service
	enhancedSvc, ok := categoryService.(category.EnhancedService)
	if !ok {
		api.Error(w, http.StatusInternalServerError, "Enhanced category features not available")
		return
	}
	
	err := enhancedSvc.OptimizeCategoryStructure(r.Context(), userID)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	api.Success(w, http.StatusOK, map[string]interface{}{
		"message":             "Category structure rebuilt successfully",
		"categoriesProcessed": 0, // Placeholder values since we don't track these yet
		"hierarchiesCreated":  0,
		"categoriesMerged":    0,
	})
}

func getCategoryInsightsHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)
	
	// Cast to enhanced service
	enhancedSvc, ok := categoryService.(category.EnhancedService)
	if !ok {
		api.Error(w, http.StatusInternalServerError, "Enhanced category features not available")
		return
	}
	
	insights, err := enhancedSvc.GenerateCategoryInsights(r.Context(), userID)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	api.Success(w, http.StatusOK, insights)
}

func getNodeCategoriesHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)
	nodeID := chi.URLParam(r, "nodeId")

	// Verify ownership before proceeding
	_, err := checkOwnership(r.Context(), nodeID)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	categories, err := categoryService.GetCategoriesForMemory(r.Context(), userID, nodeID)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	// Transform to include method and confidence if available
	var categoryResponses []map[string]interface{}
	for _, category := range categories {
		categoryData := map[string]interface{}{
			"id":          category.ID,
			"title":       category.Title,
			"description": category.Description,
			"level":       category.Level,
			"parentId":    category.ParentID,
			"color":       category.Color,
			"icon":        category.Icon,
			"aiGenerated": category.AIGenerated,
			"noteCount":   category.NoteCount,
			"createdAt":   category.CreatedAt.Format(time.RFC3339),
			"updatedAt":   category.UpdatedAt.Format(time.RFC3339),
			"confidence":  0.9, // Default confidence
			"method":      "manual", // Default method
		}
		categoryResponses = append(categoryResponses, categoryData)
	}

	api.Success(w, http.StatusOK, map[string]interface{}{"categories": categoryResponses})
}

func categorizeNodeHandler(w http.ResponseWriter, r *http.Request) {
	nodeID := chi.URLParam(r, "nodeId")

	// Verify ownership before proceeding
	node, err := checkOwnership(r.Context(), nodeID)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	// Cast to enhanced service
	enhancedSvc, ok := categoryService.(category.EnhancedService)
	if !ok {
		api.Error(w, http.StatusInternalServerError, "Enhanced category features not available")
		return
	}

	categories, err := enhancedSvc.CategorizeNode(r.Context(), *node)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	api.Success(w, http.StatusOK, map[string]interface{}{
		"categories": categories,
		"message":    "Node categorized successfully",
	})
}

func main() {
	lambda.Start(func(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
		return chiLambda.ProxyWithContextV2(ctx, req)
	})
}
