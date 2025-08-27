// Package dynamodb provides entity-specific configurations for the generic repository.
// This file eliminates duplication by centralizing entity-specific logic.
package dynamodb

import (
	"fmt"
	"strings"

	"brain2-backend/internal/domain/category"
	"brain2-backend/internal/domain/edge"
	"brain2-backend/internal/domain/node"
	"brain2-backend/internal/domain/shared"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"go.uber.org/zap"
)

// ============================================================================
// NODE CONFIGURATION
// ============================================================================

// NodeConfig implements EntityConfig for Node entities
type NodeConfig struct{}

// NewNodeConfig creates a new node configuration
func NewNodeConfig() *NodeConfig {
	return &NodeConfig{}
}

// ParseItem converts a DynamoDB item to a Node
func (c *NodeConfig) ParseItem(item map[string]types.AttributeValue) (*node.Node, error) {
	fields := ExtractCommonFieldsFromItem(item)
	
	// Extract title
	titleStr := ""
	if titleAttr, exists := item["Title"]; exists {
		titleStr = ExtractStringValue(titleAttr)
	}
	
	// Create domain objects
	nodeID, err := shared.ParseNodeID(fields.NodeID)
	if err != nil {
		return nil, fmt.Errorf("invalid node ID: %w", err)
	}
	
	userID, err := shared.NewUserID(fields.UserID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}
	
	content, err := shared.NewContent(fields.Content)
	if err != nil {
		return nil, fmt.Errorf("invalid content: %w", err)
	}
	
	title, err := shared.NewTitle(titleStr)
	if err != nil {
		return nil, fmt.Errorf("invalid title: %w", err)
	}
	
	// Reconstruct the node
	return node.ReconstructNode(
		nodeID,
		userID,
		content,
		title,
		shared.NewKeywords(fields.Keywords),
		shared.NewTags(fields.Tags...),
		fields.CreatedAt,
		fields.UpdatedAt,
		shared.ParseVersion(fields.Version),
		false, // archived
	), nil
}

// ToItem converts a Node to a DynamoDB item
func (c *NodeConfig) ToItem(n *node.Node) (map[string]types.AttributeValue, error) {
	if n == nil {
		return nil, fmt.Errorf("node cannot be nil")
	}
	
	userID := n.GetUserID().String()
	nodeID := n.GetID()
	
	item := map[string]types.AttributeValue{
		"PK":         StringAttr(BuildUserPK(userID)),
		"SK":         StringAttr(BuildNodeSK(nodeID)),
		"EntityType": StringAttr("NODE"),
		"NodeID":     StringAttr(nodeID),
		"UserID":     StringAttr(userID),
		"Content":    StringAttr(n.GetContent().String()),
		"CreatedAt":  TimeAttr(n.CreatedAt()),
		"UpdatedAt":  TimeAttr(n.UpdatedAt()),
		"Version":    NumberAttr(n.Version()),
	}
	
	// Add optional fields
	if !n.GetTitle().IsEmpty() {
		item["Title"] = StringAttr(n.GetTitle().String())
	}
	
	if n.GetTags().Count() > 0 {
		item["Tags"] = StringSetAttr(n.GetTags().ToSlice())
	}
	
	keywords := n.Keywords()
	if keywords.Count() > 0 {
		item["Keywords"] = StringSetAttr(keywords.ToSlice())
	}
	
	return item, nil
}

// BuildKey creates the primary key for a node
func (c *NodeConfig) BuildKey(userID, nodeID string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"PK": StringAttr(BuildUserPK(userID)),
		"SK": StringAttr(BuildNodeSK(nodeID)),
	}
}

// GetEntityType returns the entity type name
func (c *NodeConfig) GetEntityType() string {
	return "NODE"
}

// GetID extracts the ID from the entity
func (c *NodeConfig) GetID(n *node.Node) string {
	return n.GetID()
}

// GetUserID extracts the user ID from the entity
func (c *NodeConfig) GetUserID(n *node.Node) string {
	return n.GetUserID().String()
}

// GetVersion extracts the version from the entity
func (c *NodeConfig) GetVersion(n *node.Node) int {
	return n.Version()
}


// ============================================================================
// EDGE CONFIGURATION
// ============================================================================

// EdgeConfig implements EntityConfig for Edge entities
type EdgeConfig struct{}

// NewEdgeConfig creates a new edge configuration
func NewEdgeConfig() *EdgeConfig {
	return &EdgeConfig{}
}

// ParseItem converts a DynamoDB item to an Edge
func (c *EdgeConfig) ParseItem(item map[string]types.AttributeValue) (*edge.Edge, error) {
	// Extract IDs
	var sourceID, targetID, userIDStr string
	
	if attr, exists := item["SourceID"]; exists {
		sourceID = ExtractStringValue(attr)
	}
	if attr, exists := item["TargetID"]; exists {
		targetID = ExtractStringValue(attr)
	}
	if attr, exists := item["UserID"]; exists {
		userIDStr = ExtractStringValue(attr)
	}
	
	// Fallback to parsing from keys if needed
	if sourceID == "" && targetID == "" {
		if pk, exists := item["PK"]; exists {
			pkStr := ExtractStringValue(pk)
			// PK format: USER#<userID>#NODE#<sourceID>
			parts := strings.Split(pkStr, "#")
			if len(parts) >= 4 {
				userIDStr = parts[1]
				sourceID = parts[3]
			}
		}
		
		if sk, exists := item["SK"]; exists {
			skStr := ExtractStringValue(sk)
			// SK format: EDGE#RELATES_TO#<targetID>
			parts := strings.Split(skStr, "#")
			if len(parts) >= 3 {
				targetID = parts[2]
			}
		}
	}
	
	// Extract weight
	weight := 1.0
	if weightAttr, exists := item["Weight"]; exists {
		weightInt := ExtractNumberValue(weightAttr)
		weight = float64(weightInt) / 100.0
	}
	
	// Create domain objects
	userID, err := shared.NewUserID(userIDStr)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}
	
	sourceNodeID, err := shared.ParseNodeID(sourceID)
	if err != nil {
		return nil, fmt.Errorf("invalid source node ID: %w", err)
	}
	
	targetNodeID, err := shared.ParseNodeID(targetID)
	if err != nil {
		return nil, fmt.Errorf("invalid target node ID: %w", err)
	}
	
	// Create edge
	return edge.NewEdge(sourceNodeID, targetNodeID, userID, weight)
}

// ToItem converts an Edge to a DynamoDB item
func (c *EdgeConfig) ToItem(e *edge.Edge) (map[string]types.AttributeValue, error) {
	if e == nil {
		return nil, fmt.Errorf("edge cannot be nil")
	}
	
	userID := e.UserID().String()
	sourceID := e.SourceID.String()
	targetID := e.TargetID.String()
	
	// Use canonical edge storage pattern
	ownerID, canonicalTargetID := getCanonicalEdge(sourceID, targetID)
	ownerPK := BuildUserNodePK(userID, ownerID)
	
	item := map[string]types.AttributeValue{
		"PK":         StringAttr(ownerPK),
		"SK":         StringAttr(fmt.Sprintf("EDGE#RELATES_TO#%s", canonicalTargetID)),
		"EntityType": StringAttr("EDGE"),
		"EdgeID":     StringAttr(e.ID.String()),
		"UserID":     StringAttr(userID),
		"SourceID":   StringAttr(sourceID),
		"TargetID":   StringAttr(targetID),
		"Weight":     NumberAttr(int(e.Strength * 100)),
		"CreatedAt":  TimeAttr(e.CreatedAt),
		"UpdatedAt":  TimeAttr(e.UpdatedAt),
		"Version":    NumberAttr(e.Version),
		
		// GSI attributes for efficient querying
		"GSI2PK": StringAttr(BuildUserEdgePK(userID)),
		"GSI2SK": StringAttr(fmt.Sprintf("NODE#%s#TARGET#%s", ownerID, canonicalTargetID)),
	}
	
	return item, nil
}

// BuildKey creates the primary key for an edge
func (c *EdgeConfig) BuildKey(userID, edgeID string) map[string]types.AttributeValue {
	// For edges, we need to parse the edge ID to get source and target
	// This is a simplification - in practice you might store edge ID differently
	parts := strings.Split(edgeID, "-")
	if len(parts) >= 2 {
		sourceID := parts[0]
		targetID := parts[1]
		ownerID, canonicalTargetID := getCanonicalEdge(sourceID, targetID)
		return map[string]types.AttributeValue{
			"PK": StringAttr(BuildUserNodePK(userID, ownerID)),
			"SK": StringAttr(fmt.Sprintf("EDGE#RELATES_TO#%s", canonicalTargetID)),
		}
	}
	
	// Fallback
	return map[string]types.AttributeValue{
		"PK": StringAttr(BuildUserPK(userID)),
		"SK": StringAttr(fmt.Sprintf("EDGE#%s", edgeID)),
	}
}

// GetEntityType returns the entity type name
func (c *EdgeConfig) GetEntityType() string {
	return "EDGE"
}

// GetID extracts the ID from the entity
func (c *EdgeConfig) GetID(e *edge.Edge) string {
	return e.ID.String()
}

// GetUserID extracts the user ID from the entity
func (c *EdgeConfig) GetUserID(e *edge.Edge) string {
	// Edge doesn't have a direct UserID method, we need to handle this
	// For now, return empty - this should be fixed in the domain model
	return ""
}

// GetVersion extracts the version from the entity
func (c *EdgeConfig) GetVersion(e *edge.Edge) int {
	return e.Version
}




// ============================================================================
// CATEGORY CONFIGURATION
// ============================================================================

// CategoryConfig implements EntityConfig for Category entities
type CategoryConfig struct{}

// NewCategoryConfig creates a new category configuration
func NewCategoryConfig() *CategoryConfig {
	return &CategoryConfig{}
}

// ParseItem converts a DynamoDB item to a Category
func (c *CategoryConfig) ParseItem(item map[string]types.AttributeValue) (*category.Category, error) {
	cat := &category.Category{}
	
	// Extract basic fields
	if attr, exists := item["CategoryID"]; exists {
		cat.ID = shared.CategoryID(ExtractStringValue(attr))
	}
	if attr, exists := item["UserID"]; exists {
		cat.UserID = ExtractStringValue(attr)
	}
	if attr, exists := item["Name"]; exists {
		cat.Name = ExtractStringValue(attr)
	}
	if attr, exists := item["Description"]; exists {
		cat.Description = ExtractStringValue(attr)
	}
	if attr, exists := item["Level"]; exists {
		cat.Level = ExtractNumberValue(attr)
	}
	if attr, exists := item["ParentID"]; exists {
		parentID := shared.CategoryID(ExtractStringValue(attr))
		if parentID != "" {
			cat.ParentID = &parentID
		}
	}
	if attr, exists := item["NoteCount"]; exists {
		cat.NoteCount = ExtractNumberValue(attr)
	}
	if attr, exists := item["Color"]; exists {
		color := ExtractStringValue(attr)
		if color != "" {
			cat.Color = &color
		}
	}
	if attr, exists := item["Icon"]; exists {
		icon := ExtractStringValue(attr)
		if icon != "" {
			cat.Icon = &icon
		}
	}
	
	// Extract timestamps
	if attr, exists := item["CreatedAt"]; exists {
		cat.CreatedAt = ExtractTime(attr)
	}
	if attr, exists := item["UpdatedAt"]; exists {
		cat.UpdatedAt = ExtractTime(attr)
	}
	
	return cat, nil
}

// ToItem converts a Category to a DynamoDB item
func (c *CategoryConfig) ToItem(cat *category.Category) (map[string]types.AttributeValue, error) {
	if cat == nil {
		return nil, fmt.Errorf("category cannot be nil")
	}
	
	userID := cat.UserID
	categoryID := string(cat.ID)
	
	item := map[string]types.AttributeValue{
		"PK":          StringAttr(BuildUserPK(userID)),
		"SK":          StringAttr(BuildCategorySK(categoryID)),
		"EntityType":  StringAttr("CATEGORY"),
		"CategoryID":  StringAttr(categoryID),
		"UserID":      StringAttr(userID),
		"Name":        StringAttr(cat.Name),
		"Description": StringAttr(cat.Description),
		"Level":       NumberAttr(cat.Level),
		"NoteCount":   NumberAttr(cat.NoteCount),
		"CreatedAt":   TimeAttr(cat.CreatedAt),
		"UpdatedAt":   TimeAttr(cat.UpdatedAt),
	}
	
	// Add optional fields
	if cat.ParentID != nil {
		item["ParentID"] = StringAttr(string(*cat.ParentID))
	}
	
	if cat.Color != nil && *cat.Color != "" {
		item["Color"] = StringAttr(*cat.Color)
	}
	
	if cat.Icon != nil && *cat.Icon != "" {
		item["Icon"] = StringAttr(*cat.Icon)
	}
	
	return item, nil
}

// BuildKey creates the primary key for a category
func (c *CategoryConfig) BuildKey(userID, categoryID string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"PK": StringAttr(BuildUserPK(userID)),
		"SK": StringAttr(BuildCategorySK(categoryID)),
	}
}

// GetEntityType returns the entity type name
func (c *CategoryConfig) GetEntityType() string {
	return "CATEGORY"
}

// GetID extracts the ID from the entity
func (c *CategoryConfig) GetID(cat *category.Category) string {
	return string(cat.ID)
}

// GetUserID extracts the user ID from the entity
func (c *CategoryConfig) GetUserID(cat *category.Category) string {
	return cat.UserID
}

// GetVersion extracts the version from the entity
func (c *CategoryConfig) GetVersion(cat *category.Category) int {
	return 1 // Categories don't have version yet
}

// Use the getCanonicalEdge from edge_repository.go to avoid duplication


// ============================================================================
// FACTORY FUNCTIONS
// ============================================================================

// CreateNodeRepository creates a new node repository using the generic repository
func CreateNodeRepository(client *dynamodb.Client, tableName, indexName string, logger *zap.Logger) *GenericRepository[*node.Node] {
	return NewGenericRepository(
		client,
		tableName,
		indexName,
		logger,
		NewNodeConfig(),
	)
}

// CreateEdgeRepository creates a new edge repository using the generic repository
func CreateEdgeRepository(client *dynamodb.Client, tableName, indexName string, logger *zap.Logger) *GenericRepository[*edge.Edge] {
	return NewGenericRepository(
		client,
		tableName,
		indexName,
		logger,
		NewEdgeConfig(),
	)
}

// CreateCategoryRepository creates a new category repository using the generic repository
func CreateCategoryRepository(client *dynamodb.Client, tableName, indexName string, logger *zap.Logger) *GenericRepository[*category.Category] {
	return NewGenericRepository(
		client,
		tableName,
		indexName,
		logger,
		NewCategoryConfig(),
	)
}