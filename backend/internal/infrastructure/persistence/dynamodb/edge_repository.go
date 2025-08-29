// Package dynamodb provides the refactored EdgeRepository that uses composition
// to eliminate code duplication.
package dynamodb

import (
	"context"
	"fmt"
	"strings"
	"time"

	"brain2-backend/internal/domain/edge"
	"brain2-backend/internal/domain/shared"
	"brain2-backend/internal/repository"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"go.uber.org/zap"
)

// getCanonicalEdge returns the canonical ordering of two node IDs for consistent edge storage.
func getCanonicalEdge(nodeA, nodeB string) (owner, target string) {
	if nodeA < nodeB {
		return nodeA, nodeB
	}
	return nodeB, nodeA
}

// EdgeRepository implements EdgeReader and EdgeWriter using composition
// This eliminates duplicate code from the original edge repository
type EdgeRepository struct {
	*GenericRepository[*edge.Edge]  // Composition - inherits all CRUD operations
}

// NewEdgeRepository creates a new edge repository with minimal code
func NewEdgeRepository(client *dynamodb.Client, tableName, indexName string, logger *zap.Logger) *EdgeRepository {
	return &EdgeRepository{
		GenericRepository: CreateEdgeRepository(client, tableName, indexName, logger),
	}
}

// ============================================================================
// EDGE-SPECIFIC OPERATIONS (Only what's unique to edges)
// ============================================================================

// FindBySource retrieves all edges from a specific source node
func (r *EdgeRepository) FindBySource(ctx context.Context, userID shared.UserID, sourceID shared.NodeID) ([]*edge.Edge, error) {
	// Due to canonical edge storage, we need to find edges in both directions:
	// 1. Where sourceID is the owner (PK contains sourceID)
	// 2. Where sourceID is stored as SourceID but another node is the owner
	
	// First, find edges where this node is the canonical owner
	pk := BuildUserNodePK(userID.String(), sourceID.String())
	ownerEdges, err := r.Query(ctx, pk, WithSKPrefix("EDGE#"))
	if err != nil {
		return nil, err
	}
	
	// Filter to only edges where this node is actually the source
	result := make([]*edge.Edge, 0)
	for _, e := range ownerEdges {
		if e.SourceID.String() == sourceID.String() {
			result = append(result, e)
		}
	}
	
	// Second, use GSI to find all edges for this user and filter
	allEdges, err := r.FindByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	
	// Add edges where this node is source but not owner
	for _, e := range allEdges {
		if e.SourceID.String() == sourceID.String() {
			// Check if we already have this edge
			found := false
			for _, existing := range result {
				if existing.ID.String() == e.ID.String() {
					found = true
					break
				}
			}
			if !found {
				result = append(result, e)
			}
		}
	}
	
	return result, nil
}

// FindByTarget retrieves all edges to a specific target node
func (r *EdgeRepository) FindByTarget(ctx context.Context, userID shared.UserID, targetID shared.NodeID) ([]*edge.Edge, error) {
	// Due to canonical edge storage, we need to find edges in both directions:
	// 1. Where targetID is the owner (PK contains targetID) 
	// 2. Where targetID is stored as TargetID but another node is the owner
	
	// First, find edges where this node is the canonical owner
	pk := BuildUserNodePK(userID.String(), targetID.String())
	ownerEdges, err := r.Query(ctx, pk, WithSKPrefix("EDGE#"))
	if err != nil {
		return nil, err
	}
	
	// Filter to only edges where this node is actually the target
	result := make([]*edge.Edge, 0)
	for _, e := range ownerEdges {
		if e.TargetID.String() == targetID.String() {
			result = append(result, e)
		}
	}
	
	// Second, use GSI to find all edges for this user and filter
	allEdges, err := r.FindByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	
	// Add edges where this node is target but not owner
	for _, e := range allEdges {
		if e.TargetID.String() == targetID.String() {
			// Check if we already have this edge
			found := false
			for _, existing := range result {
				if existing.ID.String() == e.ID.String() {
					found = true
					break
				}
			}
			if !found {
				result = append(result, e)
			}
		}
	}
	
	return result, nil
}

// FindBetween retrieves edges between two specific nodes
func (r *EdgeRepository) FindBetween(ctx context.Context, userID shared.UserID, node1ID, node2ID shared.NodeID) ([]*edge.Edge, error) {
	// Check both directions due to undirected nature
	edges1, err := r.findDirectedEdge(ctx, userID, node1ID, node2ID)
	if err != nil {
		return nil, err
	}
	
	edges2, err := r.findDirectedEdge(ctx, userID, node2ID, node1ID)
	if err != nil {
		return nil, err
	}
	
	// Combine results
	edges := append(edges1, edges2...)
	return edges, nil
}

// findDirectedEdge finds a specific directed edge
func (r *EdgeRepository) findDirectedEdge(ctx context.Context, userID shared.UserID, sourceID, targetID shared.NodeID) ([]*edge.Edge, error) {
	// Use canonical ordering for storage
	ownerID, canonicalTargetID := getCanonicalEdge(sourceID.String(), targetID.String())
	
	pk := BuildUserNodePK(userID.String(), ownerID)
	sk := fmt.Sprintf("EDGE#RELATES_TO#%s", canonicalTargetID)
	
	// Query for specific edge
	edges, err := r.Query(ctx, pk, WithSK(sk))
	if err != nil {
		return nil, err
	}
	
	return edges, nil
}

// FindConnectedNodes finds all nodes connected to a given node through edges
func (r *EdgeRepository) FindConnectedNodes(ctx context.Context, userID shared.UserID, nodeID shared.NodeID) ([]shared.NodeID, error) {
	edges, err := r.FindByNode(ctx, userID, nodeID)
	if err != nil {
		return nil, err
	}
	
	// Extract unique connected node IDs
	nodeMap := make(map[string]bool)
	for _, e := range edges {
		if e.SourceID.String() == nodeID.String() {
			nodeMap[e.TargetID.String()] = true
		} else {
			nodeMap[e.SourceID.String()] = true
		}
	}
	
	// Convert to slice
	result := make([]shared.NodeID, 0, len(nodeMap))
	for id := range nodeMap {
		if nid, err := shared.ParseNodeID(id); err == nil {
			result = append(result, nid)
		}
	}
	
	return result, nil
}

// CountByNode counts edges connected to a specific node
func (r *EdgeRepository) CountByNode(ctx context.Context, userID shared.UserID, nodeID shared.NodeID) (int, error) {
	edges, err := r.FindByNode(ctx, userID, nodeID)
	if err != nil {
		return 0, err
	}
	return len(edges), nil
}

// FindByNode retrieves all edges connected to a specific node
func (r *EdgeRepository) FindByNode(ctx context.Context, userID shared.UserID, nodeID shared.NodeID, opts ...repository.QueryOption) ([]*edge.Edge, error) {
	// Find edges where node is source
	sourceEdges, err := r.FindBySource(ctx, userID, nodeID)
	if err != nil {
		return nil, err
	}
	
	// Find edges where node is target
	targetEdges, err := r.FindByTarget(ctx, userID, nodeID)
	if err != nil {
		return nil, err
	}
	
	// Combine and deduplicate
	edgeMap := make(map[string]*edge.Edge)
	for _, e := range sourceEdges {
		edgeMap[e.ID.String()] = e
	}
	for _, e := range targetEdges {
		edgeMap[e.ID.String()] = e
	}
	
	result := make([]*edge.Edge, 0, len(edgeMap))
	for _, e := range edgeMap {
		result = append(result, e)
	}
	
	return result, nil
}

// ============================================================================
// INTERFACE COMPLIANCE METHODS (Delegate to generic repository)
// ============================================================================

// FindByID retrieves an edge by its ID
func (r *EdgeRepository) FindByID(ctx context.Context, userID shared.UserID, edgeID shared.NodeID) (*edge.Edge, error) {
	// Parse edge ID to get source and target
	parts := strings.Split(edgeID.String(), "-")
	if len(parts) < 2 {
		return nil, repository.ErrEdgeNotFound("", "")
	}
	
	// Try to find the edge using source and target
	sourceID, _ := shared.ParseNodeID(parts[0])
	targetID, _ := shared.ParseNodeID(parts[1])
	
	edges, err := r.FindBetween(ctx, userID, sourceID, targetID)
	if err != nil {
		return nil, err
	}
	
	for _, e := range edges {
		if e.ID.String() == edgeID.String() {
			return e, nil
		}
	}
	
	return nil, repository.ErrEdgeNotFound("", "")
}

// Exists checks if an edge exists
func (r *EdgeRepository) Exists(ctx context.Context, userID shared.UserID, edgeID shared.NodeID) (bool, error) {
	_, err := r.FindByID(ctx, userID, edgeID)
	if repository.IsNotFound(err) {
		return false, nil
	}
	return err == nil, err
}

// Save creates a new edge
func (r *EdgeRepository) Save(ctx context.Context, e *edge.Edge) error {
	return r.GenericRepository.Save(ctx, e)
}

// Update updates an existing edge
func (r *EdgeRepository) Update(ctx context.Context, e *edge.Edge) error {
	return r.GenericRepository.Update(ctx, e)
}

// Delete deletes an edge
func (r *EdgeRepository) Delete(ctx context.Context, userID shared.UserID, edgeID shared.NodeID) error {
	// First, try to find the edge by ID to get source and target
	allEdges, err := r.FindByUser(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to find edges for deletion: %w", err)
	}
	
	// Find the specific edge
	var targetEdge *edge.Edge
	for _, e := range allEdges {
		if e.ID.String() == edgeID.String() {
			targetEdge = e
			break
		}
	}
	
	if targetEdge == nil {
		return repository.ErrEdgeNotFound("", "")
	}
	
	// Now delete using the actual source and target IDs
	sourceID := targetEdge.SourceID.String()
	targetID := targetEdge.TargetID.String()
	ownerID, canonicalTargetID := getCanonicalEdge(sourceID, targetID)
	
	// Build the key for deletion
	pk := BuildUserNodePK(userID.String(), ownerID)
	sk := fmt.Sprintf("EDGE#RELATES_TO#%s", canonicalTargetID)
	
	return r.GenericRepository.DeleteByKey(ctx, pk, sk)
}

// DeleteByNodes deletes edges between specific nodes
func (r *EdgeRepository) DeleteByNodes(ctx context.Context, userID shared.UserID, sourceID, targetID shared.NodeID) error {
	edges, err := r.FindBetween(ctx, userID, sourceID, targetID)
	if err != nil {
		return err
	}
	
	for _, e := range edges {
		if err := r.Delete(ctx, userID, e.ID); err != nil {
			return err
		}
	}
	
	return nil
}

// FindByUser retrieves all edges for a user
func (r *EdgeRepository) FindByUser(ctx context.Context, userID shared.UserID, opts ...repository.QueryOption) ([]*edge.Edge, error) {
	// Use GSI to query all edges for a user
	return r.QueryByGSI(ctx, BuildUserEdgePK(userID.String()), "")
}

// CountByUser counts edges for a user
func (r *EdgeRepository) CountByUser(ctx context.Context, userID shared.UserID) (int, error) {
	edges, err := r.FindByUser(ctx, userID)
	if err != nil {
		return 0, err
	}
	return len(edges), nil
}

// FindByWeight finds edges within a weight range
func (r *EdgeRepository) FindByWeight(ctx context.Context, userID shared.UserID, minWeight, maxWeight float64) ([]*edge.Edge, error) {
	edges, err := r.FindByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	
	// Filter by weight
	filtered := make([]*edge.Edge, 0)
	for _, e := range edges {
		if e.Strength >= minWeight && e.Strength <= maxWeight {
			filtered = append(filtered, e)
		}
	}
	
	return filtered, nil
}

// UpdateWeight updates the weight of an edge
func (r *EdgeRepository) UpdateWeight(ctx context.Context, userID shared.UserID, edgeID shared.NodeID, newWeight float64, version shared.Version) error {
	e, err := r.FindByID(ctx, userID, edgeID)
	if err != nil {
		return err
	}
	
	// Update weight
	e.Strength = newWeight
	e.UpdatedAt = time.Now()
	
	return r.Update(ctx, e)
}

// BatchGetEdges retrieves multiple edges
func (r *EdgeRepository) BatchGetEdges(ctx context.Context, userID string, edgeIDs []string) (map[string]*edge.Edge, error) {
	return r.GenericRepository.BatchGet(ctx, userID, edgeIDs)
}

// BatchDeleteEdges deletes multiple edges
func (r *EdgeRepository) BatchDeleteEdges(ctx context.Context, userID string, edgeIDs []string) (deleted []string, failed []string, err error) {
	// For each edge ID, parse and delete
	for _, edgeID := range edgeIDs {
		uid, _ := shared.NewUserID(userID)
		nid, _ := shared.ParseNodeID(edgeID) // Using NodeID as EdgeID
		if err := r.Delete(ctx, uid, nid); err != nil {
			failed = append(failed, edgeID)
		} else {
			deleted = append(deleted, edgeID)
		}
	}
	
	if len(failed) > 0 {
		err = fmt.Errorf("failed to delete %d edges", len(failed))
	}
	
	return deleted, failed, err
}

// FindBySourceNode is an alias for FindBySource for interface compliance
func (r *EdgeRepository) FindBySourceNode(ctx context.Context, userID shared.UserID, sourceID shared.NodeID, opts ...repository.QueryOption) ([]*edge.Edge, error) {
	return r.FindBySource(ctx, userID, sourceID)
}

// FindByTargetNode is an alias for FindByTarget for interface compliance
func (r *EdgeRepository) FindByTargetNode(ctx context.Context, userID shared.UserID, targetID shared.NodeID, opts ...repository.QueryOption) ([]*edge.Edge, error) {
	return r.FindByTarget(ctx, userID, targetID)
}

// FindBetweenNodes is an alias for FindBetween for interface compliance
func (r *EdgeRepository) FindBetweenNodes(ctx context.Context, userID shared.UserID, node1ID, node2ID shared.NodeID) ([]*edge.Edge, error) {
	return r.FindBetween(ctx, userID, node1ID, node2ID)
}

// FindStrongConnections finds edges above a weight threshold
func (r *EdgeRepository) FindStrongConnections(ctx context.Context, userID shared.UserID, threshold float64, opts ...repository.QueryOption) ([]*edge.Edge, error) {
	edges, err := r.FindByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	
	filtered := make([]*edge.Edge, 0)
	for _, e := range edges {
		if e.Strength >= threshold {
			filtered = append(filtered, e)
		}
	}
	
	return filtered, nil
}

// FindWeakConnections finds edges below a weight threshold
func (r *EdgeRepository) FindWeakConnections(ctx context.Context, userID shared.UserID, threshold float64, opts ...repository.QueryOption) ([]*edge.Edge, error) {
	edges, err := r.FindByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	
	filtered := make([]*edge.Edge, 0)
	for _, e := range edges {
		if e.Strength < threshold {
			filtered = append(filtered, e)
		}
	}
	
	return filtered, nil
}

// FindBySpecification finds edges matching a specification
func (r *EdgeRepository) FindBySpecification(ctx context.Context, spec repository.Specification, opts ...repository.QueryOption) ([]*edge.Edge, error) {
	// Simplified implementation - would need proper specification pattern
	if spec == nil {
		return nil, fmt.Errorf("invalid specification")
	}
	// This would need proper specification implementation
	return []*edge.Edge{}, nil
}

// CountBySpecification counts edges matching a specification
func (r *EdgeRepository) CountBySpecification(ctx context.Context, spec repository.Specification) (int, error) {
	edges, err := r.FindBySpecification(ctx, spec)
	if err != nil {
		return 0, err
	}
	return len(edges), nil
}

// FindPage retrieves a page of edges
func (r *EdgeRepository) FindPage(ctx context.Context, query repository.EdgeQuery, pagination repository.Pagination) (*repository.EdgePage, error) {
	uid, _ := shared.NewUserID(query.UserID)
	edges, err := r.FindByUser(ctx, uid)
	if err != nil {
		return nil, err
	}
	
	// Simple pagination
	start := 0
	if pagination.HasCursor() {
		// Would need proper cursor decoding
	}
	
	limit := pagination.GetEffectiveLimit()
	end := start + limit
	if end > len(edges) {
		end = len(edges)
	}
	
	pageEdges := edges[start:end]
	
	return &repository.EdgePage{
		Items:      pageEdges,
		HasMore:    end < len(edges),
		NextCursor: "", // Would need proper cursor encoding
		PageInfo:   repository.CreatePageInfo(pagination, len(pageEdges), end < len(edges)),
	}, nil
}

// FindEdges finds edges matching a query
func (r *EdgeRepository) FindEdges(ctx context.Context, query repository.EdgeQuery) ([]*edge.Edge, error) {
	uid, _ := shared.NewUserID(query.UserID)
	return r.FindByUser(ctx, uid)
}

// CountBySourceID counts edges from a specific source
func (r *EdgeRepository) CountBySourceID(ctx context.Context, sourceID shared.NodeID) (int, error) {
	// This would need to query across all users - simplified for now
	return 0, nil
}

// CreateEdge creates a new edge (alias for Save for interface compliance)
func (r *EdgeRepository) CreateEdge(ctx context.Context, e *edge.Edge) error {
	return r.Save(ctx, e)
}

// DeleteEdge deletes an edge by ID
func (r *EdgeRepository) DeleteEdge(ctx context.Context, userID, edgeID string) error {
	uid, _ := shared.NewUserID(userID)
	nid, _ := shared.ParseNodeID(edgeID) // EdgeID is represented as NodeID in the interface
	return r.Delete(ctx, uid, nid)
}

// DeleteEdgesByNode deletes all edges connected to a node
func (r *EdgeRepository) DeleteEdgesByNode(ctx context.Context, userID, nodeID string) error {
	uid, _ := shared.NewUserID(userID)
	nid, _ := shared.ParseNodeID(nodeID)
	
	edges, err := r.FindByNode(ctx, uid, nid)
	if err != nil {
		return err
	}
	
	for _, e := range edges {
		if err := r.Delete(ctx, uid, e.ID); err != nil {
			return err
		}
	}
	
	return nil
}

// DeleteEdgesBetweenNodes deletes edges between two specific nodes
func (r *EdgeRepository) DeleteEdgesBetweenNodes(ctx context.Context, userID, sourceNodeID, targetNodeID string) error {
	uid, _ := shared.NewUserID(userID)
	sid, _ := shared.ParseNodeID(sourceNodeID)
	tid, _ := shared.ParseNodeID(targetNodeID)
	
	return r.DeleteByNodes(ctx, uid, sid, tid)
}

// GetEdgesPage retrieves a paginated list of edges (alias for FindPage)
func (r *EdgeRepository) GetEdgesPage(ctx context.Context, query repository.EdgeQuery, pagination repository.Pagination) (*repository.EdgePage, error) {
	return r.FindPage(ctx, query, pagination)
}

// FindEdgesWithOptions finds edges with query options
func (r *EdgeRepository) FindEdgesWithOptions(ctx context.Context, query repository.EdgeQuery, opts ...repository.QueryOption) ([]*edge.Edge, error) {
	return r.FindEdges(ctx, query)
}

// CreateEdges creates multiple edges from a source to multiple targets
func (r *EdgeRepository) CreateEdges(ctx context.Context, userID, sourceNodeID string, relatedNodeIDs []string) error {
	uid, err := shared.NewUserID(userID)
	if err != nil {
		return err
	}
	
	sourceID, err := shared.ParseNodeID(sourceNodeID)
	if err != nil {
		return err
	}
	
	for _, targetID := range relatedNodeIDs {
		tid, err := shared.ParseNodeID(targetID)
		if err != nil {
			continue
		}
		
		e, err := edge.NewEdge(sourceID, tid, uid, 1.0)
		if err != nil {
			continue
		}
		
		if err := r.Save(ctx, e); err != nil {
			return err
		}
	}
	
	return nil
}

// ============================================================================
// ENSURE INTERFACES ARE IMPLEMENTED
// ============================================================================

// DeleteBatch deletes multiple edges
func (r *EdgeRepository) DeleteBatch(ctx context.Context, userID shared.UserID, edgeIDs []shared.NodeID) error {
	for _, edgeID := range edgeIDs {
		if err := r.Delete(ctx, userID, edgeID); err != nil {
			return err
		}
	}
	return nil
}

// DeleteByNode deletes all edges connected to a node
func (r *EdgeRepository) DeleteByNode(ctx context.Context, userID shared.UserID, nodeID shared.NodeID) error {
	edges, err := r.FindByNode(ctx, userID, nodeID)
	if err != nil {
		return err
	}
	
	// Delete each edge directly using its source and target IDs
	for _, e := range edges {
		sourceID := e.SourceID.String()
		targetID := e.TargetID.String()
		ownerID, canonicalTargetID := getCanonicalEdge(sourceID, targetID)
		
		// Build the key for deletion
		pk := BuildUserNodePK(userID.String(), ownerID)
		sk := fmt.Sprintf("EDGE#RELATES_TO#%s", canonicalTargetID)
		
		if err := r.GenericRepository.DeleteByKey(ctx, pk, sk); err != nil {
			// Log the error but continue deleting other edges
			r.GenericRepository.logger.Error("Failed to delete edge",
				zap.String("edgeID", e.ID.String()),
				zap.String("sourceID", sourceID),
				zap.String("targetID", targetID),
				zap.Error(err))
			// Don't return error, continue with other edges
		}
	}
	
	return nil
}

// DeleteByNodeID is an alias for DeleteByNode for interface compatibility
func (r *EdgeRepository) DeleteByNodeID(ctx context.Context, nodeID shared.NodeID) error {
	// This method needs userID, but we can't get it without querying
	// For now, this is a limitation that needs to be addressed at the interface level
	// The proper method to use is DeleteByNode with userID
	return fmt.Errorf("DeleteByNodeID requires userID, use DeleteByNode instead")
}

// SaveBatch saves multiple edges
func (r *EdgeRepository) SaveBatch(ctx context.Context, edges []*edge.Edge) error {
	return r.GenericRepository.BatchSave(ctx, edges)
}

// SaveManyToOne creates edges from multiple sources to one target
func (r *EdgeRepository) SaveManyToOne(ctx context.Context, userID shared.UserID, sourceID shared.NodeID, targetIDs []shared.NodeID, weights []float64) error {
	for i, targetID := range targetIDs {
		weight := 1.0
		if i < len(weights) {
			weight = weights[i]
		}
		
		e, err := edge.NewEdge(sourceID, targetID, userID, weight)
		if err != nil {
			return err
		}
		
		if err := r.Save(ctx, e); err != nil {
			return err
		}
	}
	return nil
}

// SaveOneToMany creates edges from one source to multiple targets
func (r *EdgeRepository) SaveOneToMany(ctx context.Context, userID shared.UserID, sourceIDs []shared.NodeID, targetID shared.NodeID, weights []float64) error {
	for i, sourceID := range sourceIDs {
		weight := 1.0
		if i < len(weights) {
			weight = weights[i]
		}
		
		e, err := edge.NewEdge(sourceID, targetID, userID, weight)
		if err != nil {
			return err
		}
		
		if err := r.Save(ctx, e); err != nil {
			return err
		}
	}
	return nil
}

var (
	_ repository.EdgeReader     = (*EdgeRepository)(nil)
	_ repository.EdgeWriter     = (*EdgeRepository)(nil)
	_ repository.EdgeRepository = (*EdgeRepository)(nil)
)