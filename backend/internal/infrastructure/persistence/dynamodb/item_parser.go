// Package dynamodb provides generic item parsing utilities for DynamoDB operations.
//
// This file contains reusable parsing patterns to reduce code duplication
// when converting between DynamoDB items and domain entities.
package dynamodb

import (
	"fmt"

	"brain2-backend/internal/domain/category"
	"brain2-backend/internal/domain/edge"
	"brain2-backend/internal/domain/node"
	"brain2-backend/internal/domain/shared"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// ============================================================================
// NODE PARSER
// ============================================================================

// NodeParser implements EntityParser for Node entities
type NodeParser struct{}

// NewNodeParser creates a new node parser instance
func NewNodeParser() *NodeParser {
	return &NodeParser{}
}

// ToItem converts a node domain entity to a DynamoDB item
func (p *NodeParser) ToItem(n *node.Node) (map[string]types.AttributeValue, error) {
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
	
	if n.Metadata() != nil {
		metaMap, err := attributevalue.Marshal(n.Metadata())
		if err == nil {
			item["Metadata"] = metaMap
		}
	}
	
	return item, nil
}

// FromItem converts a DynamoDB item to a node domain entity
func (p *NodeParser) FromItem(item map[string]types.AttributeValue) (*node.Node, error) {
	fields := ExtractCommonFieldsFromItem(item)
	
	// Extract additional node-specific fields
	titleStr := ""
	if titleAttr, exists := item["Title"]; exists {
		titleStr = ExtractStringValue(titleAttr)
	}
	
	// Create domain value objects
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
		false, // archived - could be extracted if needed
	), nil
}

// ============================================================================
// EDGE PARSER
// ============================================================================

// EdgeParser implements EntityParser for Edge entities
type EdgeParser struct{}

// NewEdgeParser creates a new edge parser instance
func NewEdgeParser() *EdgeParser {
	return &EdgeParser{}
}

// ToItem converts an edge domain entity to a DynamoDB item
func (p *EdgeParser) ToItem(e *edge.Edge) (map[string]types.AttributeValue, error) {
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
		"Weight":     NumberAttr(int(e.Strength * 100)), // Store as int to avoid float precision issues
		"CreatedAt":  TimeAttr(e.CreatedAt),
		"UpdatedAt":  TimeAttr(e.UpdatedAt),
		"Version":    NumberAttr(e.Version),
		
		// GSI attributes for efficient querying
		"GSI2PK": StringAttr(BuildUserEdgePK(userID)),
		"GSI2SK": StringAttr(fmt.Sprintf("NODE#%s#TARGET#%s", ownerID, canonicalTargetID)),
	}
	
	// Metadata is handled by the private field, skipping for now
	
	return item, nil
}

// FromItem converts a DynamoDB item to an edge domain entity
func (p *EdgeParser) FromItem(item map[string]types.AttributeValue) (*edge.Edge, error) {
	// Extract IDs
	var sourceID, targetID, userIDStr string
	
	// Try direct fields first
	if attr, exists := item["SourceID"]; exists {
		sourceID = ExtractStringValue(attr)
	}
	if attr, exists := item["TargetID"]; exists {
		targetID = ExtractStringValue(attr)
	}
	if attr, exists := item["UserID"]; exists {
		userIDStr = ExtractStringValue(attr)
	}
	
	// Fallback to parsing from PK/SK if needed
	if sourceID == "" || targetID == "" {
		if pk, exists := item["PK"]; exists {
			// PK format: USER#<userID>#NODE#<sourceID>
			pkStr := ExtractStringValue(pk)
			parts := splitPK(pkStr)
			if len(parts) >= 4 {
				userIDStr = parts[1]
				sourceID = parts[3]
			}
		}
		
		if sk, exists := item["SK"]; exists {
			// SK format: EDGE#RELATES_TO#<targetID>
			skStr := ExtractStringValue(sk)
			parts := splitSK(skStr)
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

// ============================================================================
// CATEGORY PARSER
// ============================================================================

// CategoryParser implements EntityParser for Category entities
type CategoryParser struct{}

// NewCategoryParser creates a new category parser instance
func NewCategoryParser() *CategoryParser {
	return &CategoryParser{}
}

// ToItem converts a category domain entity to a DynamoDB item
func (p *CategoryParser) ToItem(c *category.Category) (map[string]types.AttributeValue, error) {
	if c == nil {
		return nil, fmt.Errorf("category cannot be nil")
	}

	userID := c.UserID
	categoryID := string(c.ID)
	
	item := map[string]types.AttributeValue{
		"PK":          StringAttr(BuildUserPK(userID)),
		"SK":          StringAttr(BuildCategorySK(categoryID)),
		"EntityType":  StringAttr("CATEGORY"),
		"CategoryID":  StringAttr(categoryID),
		"UserID":      StringAttr(userID),
		"Name":        StringAttr(c.Name),
		"Description": StringAttr(c.Description),
		"Level":       NumberAttr(c.Level),
		"ParentID":    StringAttr(func() string { if c.ParentID != nil { return string(*c.ParentID) } else { return "" } }()),
		"NoteCount":   NumberAttr(c.NoteCount),
		"CreatedAt":   TimeAttr(c.CreatedAt),
		"UpdatedAt":   TimeAttr(c.UpdatedAt),
	}
	
	// Add color if present
	if c.Color != nil && *c.Color != "" {
		item["Color"] = StringAttr(*c.Color)
	}
	
	// Add icon if present  
	if c.Icon != nil && *c.Icon != "" {
		item["Icon"] = StringAttr(*c.Icon)
	}
	
	return item, nil
}

// FromItem converts a DynamoDB item to a category domain entity
func (p *CategoryParser) FromItem(item map[string]types.AttributeValue) (*category.Category, error) {
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
		cat.Color = &color
	}
	if attr, exists := item["Icon"]; exists {
		icon := ExtractStringValue(attr)
		cat.Icon = &icon
	}
	
	// Extract timestamps
	if attr, exists := item["CreatedAt"]; exists {
		cat.CreatedAt = ExtractTime(attr)
	}
	if attr, exists := item["UpdatedAt"]; exists {
		cat.UpdatedAt = ExtractTime(attr)
	}
	
	// Note: Children field would need to be handled separately
	// as it's not part of the category struct
	
	return cat, nil
}

// ============================================================================
// GENERIC PARSER
// ============================================================================

// GenericParser provides a flexible parser for any entity type
type GenericParser struct {
	toItemFunc   func(interface{}) (map[string]types.AttributeValue, error)
	fromItemFunc func(map[string]types.AttributeValue) (interface{}, error)
}

// NewGenericParser creates a new generic parser with custom conversion functions
func NewGenericParser(
	toItem func(interface{}) (map[string]types.AttributeValue, error),
	fromItem func(map[string]types.AttributeValue) (interface{}, error),
) *GenericParser {
	return &GenericParser{
		toItemFunc:   toItem,
		fromItemFunc: fromItem,
	}
}

// ToItem converts an entity to a DynamoDB item using the custom function
func (p *GenericParser) ToItem(entity interface{}) (map[string]types.AttributeValue, error) {
	return p.toItemFunc(entity)
}

// FromItem converts a DynamoDB item to an entity using the custom function
func (p *GenericParser) FromItem(item map[string]types.AttributeValue) (interface{}, error) {
	return p.fromItemFunc(item)
}

// ============================================================================
// PARSER REGISTRY
// ============================================================================

// ParserRegistry manages parsers for different entity types
type ParserRegistry struct {
	parsers map[string]interface{}
}

// NewParserRegistry creates a new parser registry
func NewParserRegistry() *ParserRegistry {
	registry := &ParserRegistry{
		parsers: make(map[string]interface{}),
	}
	
	// Register default parsers
	registry.Register("NODE", NewNodeParser())
	registry.Register("EDGE", NewEdgeParser())
	registry.Register("CATEGORY", NewCategoryParser())
	
	return registry
}

// Register adds a parser to the registry
func (r *ParserRegistry) Register(entityType string, parser interface{}) {
	r.parsers[entityType] = parser
}

// GetParser retrieves a parser for the given entity type
func (r *ParserRegistry) GetParser(entityType string) (interface{}, error) {
	parser, exists := r.parsers[entityType]
	if !exists {
		return nil, fmt.Errorf("no parser registered for entity type: %s", entityType)
	}
	return parser, nil
}

// ============================================================================
// UTILITY FUNCTIONS
// ============================================================================

// splitPK splits a partition key into its components
func splitPK(pk string) []string {
	// PK format: USER#<userID> or USER#<userID>#NODE#<nodeID>
	var parts []string
	current := ""
	
	for i := 0; i < len(pk); i++ {
		if pk[i] == '#' {
			parts = append(parts, current)
			current = ""
		} else {
			current += string(pk[i])
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	
	return parts
}

// splitSK splits a sort key into its components  
func splitSK(sk string) []string {
	// SK format: NODE#<nodeID> or EDGE#RELATES_TO#<targetID>
	return splitPK(sk) // Same splitting logic
}


// DynamoDBItemToMap converts a DynamoDB item to a generic map
func DynamoDBItemToMap(item map[string]types.AttributeValue) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	
	for key, value := range item {
		switch v := value.(type) {
		case *types.AttributeValueMemberS:
			result[key] = v.Value
		case *types.AttributeValueMemberN:
			result[key] = v.Value
		case *types.AttributeValueMemberBOOL:
			result[key] = v.Value
		case *types.AttributeValueMemberSS:
			result[key] = v.Value
		case *types.AttributeValueMemberL:
			var list []interface{}
			for _, item := range v.Value {
				// Recursively handle list items
				subMap := map[string]types.AttributeValue{"item": item}
				converted, _ := DynamoDBItemToMap(subMap)
				list = append(list, converted["item"])
			}
			result[key] = list
		case *types.AttributeValueMemberM:
			// Recursively handle maps
			subMap, _ := DynamoDBItemToMap(v.Value)
			result[key] = subMap
		}
	}
	
	return result, nil
}

// MapToDynamoDBItem converts a generic map to a DynamoDB item
func MapToDynamoDBItem(data map[string]interface{}) (map[string]types.AttributeValue, error) {
	return attributevalue.MarshalMap(data)
}