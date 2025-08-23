// Package dynamodb provides shared utilities for DynamoDB operations.
//
// This file contains common patterns to reduce code duplication across repositories
// by providing standardized methods for:
//   - Primary key construction (PK/SK patterns)
//   - Attribute value creation and extraction
//   - Common field parsing from DynamoDB items
//   - Domain object construction helpers
//
// All utility functions are designed to be stateless and thread-safe,
// making them suitable for use across concurrent repository operations.
package dynamodb

import (
	"fmt"
	"strings"
	"time"

	"brain2-backend/internal/domain/shared"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// ============================================================================
// PRIMARY KEY CONSTRUCTION UTILITIES
// ============================================================================

// BuildUserPK constructs a user partition key in the standard format: USER#{userId}
func BuildUserPK(userID string) string {
	return fmt.Sprintf("USER#%s", userID)
}

// BuildNodeSK constructs a node sort key in the standard format: NODE#{nodeId}
func BuildNodeSK(nodeID string) string {
	return fmt.Sprintf("NODE#%s", nodeID)
}

// BuildEdgeSK constructs an edge sort key in the standard format: EDGE#{targetId}
func BuildEdgeSK(targetID string) string {
	return fmt.Sprintf("EDGE#%s", targetID)
}

// BuildCategorySK constructs a category sort key in the standard format: CATEGORY#{categoryId}
func BuildCategorySK(categoryID string) string {
	return fmt.Sprintf("CATEGORY#%s", categoryID)
}

// BuildUserNodePK constructs a user+node partition key: USER#{userId}#NODE#{nodeId}
func BuildUserNodePK(userID, nodeID string) string {
	return fmt.Sprintf("USER#%s#NODE#%s", userID, nodeID)
}

// BuildUserEdgePK constructs a user edge GSI partition key: USER#{userId}#EDGE
func BuildUserEdgePK(userID string) string {
	return fmt.Sprintf("USER#%s#EDGE", userID)
}

// ============================================================================
// ATTRIBUTE VALUE CONSTRUCTION UTILITIES
// ============================================================================

// StringAttr creates a DynamoDB string attribute value
func StringAttr(value string) *types.AttributeValueMemberS {
	return &types.AttributeValueMemberS{Value: value}
}

// NumberAttr creates a DynamoDB number attribute value
func NumberAttr(value int) *types.AttributeValueMemberN {
	return &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", value)}
}

// StringSetAttr creates a DynamoDB string set attribute value
func StringSetAttr(values []string) *types.AttributeValueMemberSS {
	if len(values) == 0 {
		return nil
	}
	return &types.AttributeValueMemberSS{Value: values}
}

// TimeAttr creates a DynamoDB string attribute value for timestamps in RFC3339 format
func TimeAttr(t time.Time) *types.AttributeValueMemberS {
	return &types.AttributeValueMemberS{Value: t.Format(time.RFC3339)}
}

// ============================================================================
// ATTRIBUTE VALUE EXTRACTION UTILITIES
// ============================================================================

// ExtractStringValue extracts string value from DynamoDB attribute, returns empty string if not found
func ExtractStringValue(attr types.AttributeValue) string {
	if v, ok := attr.(*types.AttributeValueMemberS); ok {
		return v.Value
	}
	return ""
}

// ExtractNumberValue extracts integer value from DynamoDB attribute, returns 0 if not found or invalid
func ExtractNumberValue(attr types.AttributeValue) int {
	if v, ok := attr.(*types.AttributeValueMemberN); ok {
		var result int
		fmt.Sscanf(v.Value, "%d", &result)
		return result
	}
	return 0
}

// ExtractStringSet extracts string slice from DynamoDB attribute, returns nil if not found
func ExtractStringSet(attr types.AttributeValue) []string {
	if v, ok := attr.(*types.AttributeValueMemberSS); ok {
		return v.Value
	}
	if v, ok := attr.(*types.AttributeValueMemberL); ok {
		var result []string
		for _, item := range v.Value {
			if str, ok := item.(*types.AttributeValueMemberS); ok {
				result = append(result, str.Value)
			}
		}
		return result
	}
	return nil
}

// ExtractTime extracts time from DynamoDB string attribute in RFC3339 format, returns current time if not found
func ExtractTime(attr types.AttributeValue) time.Time {
	if v, ok := attr.(*types.AttributeValueMemberS); ok {
		if t, err := time.Parse(time.RFC3339, v.Value); err == nil {
			return t
		}
	}
	return time.Now()
}

// ============================================================================
// KEY EXTRACTION UTILITIES
// ============================================================================

// ExtractIDFromPK extracts ID from partition key with given prefix
// Example: ExtractIDFromPK("USER#123", "USER#") returns "123"
func ExtractIDFromPK(pk, prefix string) string {
	if strings.HasPrefix(pk, prefix) {
		return strings.TrimPrefix(pk, prefix)
	}
	return ""
}

// ExtractIDFromSK extracts ID from sort key with given prefix
// Example: ExtractIDFromSK("NODE#456", "NODE#") returns "456"
func ExtractIDFromSK(sk, prefix string) string {
	if strings.HasPrefix(sk, prefix) {
		return strings.TrimPrefix(sk, prefix)
	}
	return ""
}

// ExtractUserID extracts user ID from either UserID field or PK field
func ExtractUserID(item map[string]types.AttributeValue) string {
	// Try direct UserID field first
	if userIDAttr, exists := item["UserID"]; exists {
		return ExtractStringValue(userIDAttr)
	}
	
	// Try extracting from PK field
	if pkAttr, exists := item["PK"]; exists {
		pk := ExtractStringValue(pkAttr)
		return ExtractIDFromPK(pk, "USER#")
	}
	
	return ""
}

// ExtractNodeID extracts node ID from either NodeID field or SK field
func ExtractNodeID(item map[string]types.AttributeValue) string {
	// Try direct NodeID field first
	if nodeIDAttr, exists := item["NodeID"]; exists {
		return ExtractStringValue(nodeIDAttr)
	}
	
	// Try extracting from SK field
	if skAttr, exists := item["SK"]; exists {
		sk := ExtractStringValue(skAttr)
		return ExtractIDFromSK(sk, "NODE#")
	}
	
	return ""
}

// ============================================================================
// COMMON FIELD EXTRACTION UTILITIES
// ============================================================================

// ExtractCommonFields extracts commonly used fields from DynamoDB item
type CommonFields struct {
	UserID    string
	NodeID    string
	Content   string
	Version   int
	CreatedAt time.Time
	UpdatedAt time.Time
	Tags      []string
	Keywords  []string
}

// ExtractCommonFieldsFromItem extracts common fields from DynamoDB item in a single operation.
//
// This function provides a centralized way to parse the most commonly accessed fields
// from DynamoDB items across different entity types. It handles multiple storage formats
// for backward compatibility and provides sensible defaults for missing fields.
//
// Field extraction logic:
//   - UserID: Tries direct field first, then extracts from PK with USER# prefix
//   - NodeID: Tries direct field first, then extracts from SK with NODE# prefix  
//   - Content: Direct string field extraction
//   - Version: Number field with default value of 1
//   - Timestamps: RFC3339 format with current time as fallback
//   - Collections: Supports both List and StringSet formats
//
// Parameters:
//   - item: DynamoDB item map from query/scan operations
//
// Returns:
//   - CommonFields: Struct containing all extracted fields with safe defaults
func ExtractCommonFieldsFromItem(item map[string]types.AttributeValue) CommonFields {
	fields := CommonFields{
		UserID:  ExtractUserID(item),
		NodeID:  ExtractNodeID(item),
		Version: 1, // default version
	}
	
	// Extract content
	if contentAttr, exists := item["Content"]; exists {
		fields.Content = ExtractStringValue(contentAttr)
	}
	
	// Extract version
	if versionAttr, exists := item["Version"]; exists {
		fields.Version = ExtractNumberValue(versionAttr)
	}
	
	// Extract timestamps
	fields.CreatedAt = time.Now()
	fields.UpdatedAt = time.Now()
	
	if createdAtAttr, exists := item["CreatedAt"]; exists {
		fields.CreatedAt = ExtractTime(createdAtAttr)
	}
	
	if updatedAtAttr, exists := item["UpdatedAt"]; exists {
		fields.UpdatedAt = ExtractTime(updatedAtAttr)
	}
	
	// Extract collections
	if tagsAttr, exists := item["Tags"]; exists {
		fields.Tags = ExtractStringSet(tagsAttr)
	}
	
	if keywordsAttr, exists := item["Keywords"]; exists {
		fields.Keywords = ExtractStringSet(keywordsAttr)
	}
	
	return fields
}

// ============================================================================
// DOMAIN OBJECT CONSTRUCTION HELPERS
// ============================================================================

// BuildNodeIDFromFields safely constructs a shared.NodeID from extracted fields
func BuildNodeIDFromFields(fields CommonFields) (shared.NodeID, error) {
	if fields.NodeID == "" {
		return shared.NodeID{}, fmt.Errorf("node ID is empty")
	}
	return shared.ParseNodeID(fields.NodeID)
}

// BuildUserIDFromFields safely constructs a shared.UserID from extracted fields
func BuildUserIDFromFields(fields CommonFields) (shared.UserID, error) {
	if fields.UserID == "" {
		return shared.UserID{}, fmt.Errorf("user ID is empty")
	}
	return shared.NewUserID(fields.UserID)
}

// BuildContentFromFields safely constructs a shared.Content from extracted fields
func BuildContentFromFields(fields CommonFields) (shared.Content, error) {
	if fields.Content == "" {
		return shared.Content{}, fmt.Errorf("content is empty")
	}
	return shared.NewContent(fields.Content)
}