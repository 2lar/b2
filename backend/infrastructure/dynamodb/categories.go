// Enhanced category operations for DynamoDB implementation
package dynamodb

import (
	"context"
	"fmt"
	"log"
	"time"

	"brain2-backend/internal/domain"
	"brain2-backend/internal/repository"
	appErrors "brain2-backend/pkg/errors"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// Enhanced DynamoDB structures for hierarchical categories

// ddbEnhancedCategory represents the enhanced category structure in DynamoDB
type ddbEnhancedCategory struct {
	PK          string  `dynamodbav:"PK"`
	SK          string  `dynamodbav:"SK"`
	CategoryID  string  `dynamodbav:"CategoryID"`
	UserID      string  `dynamodbav:"UserID"`
	Title       string  `dynamodbav:"Title"`
	Description string  `dynamodbav:"Description"`
	Level       int     `dynamodbav:"Level"`
	ParentID    *string `dynamodbav:"ParentID,omitempty"`
	Color       *string `dynamodbav:"Color,omitempty"`
	Icon        *string `dynamodbav:"Icon,omitempty"`
	AIGenerated bool    `dynamodbav:"AIGenerated"`
	NoteCount   int     `dynamodbav:"NoteCount"`
	CreatedAt   string  `dynamodbav:"CreatedAt"`
	UpdatedAt   string  `dynamodbav:"UpdatedAt"`
	// GSI fields
	GSI1PK      string  `dynamodbav:"GSI1PK"` // For level queries
	GSI1SK      string  `dynamodbav:"GSI1SK"`
}

// ddbCategoryHierarchy represents category hierarchy relationships
type ddbCategoryHierarchy struct {
	PK        string `dynamodbav:"PK"`
	SK        string `dynamodbav:"SK"`
	UserID    string `dynamodbav:"UserID"`
	ParentID  string `dynamodbav:"ParentID"`
	ChildID   string `dynamodbav:"ChildID"`
	CreatedAt string `dynamodbav:"CreatedAt"`
}

// ddbNodeCategory represents node-category relationships
type ddbNodeCategory struct {
	PK         string  `dynamodbav:"PK"`
	SK         string  `dynamodbav:"SK"`
	UserID     string  `dynamodbav:"UserID"`
	NodeID     string  `dynamodbav:"NodeID"`
	CategoryID string  `dynamodbav:"CategoryID"`
	Confidence float64 `dynamodbav:"Confidence"`
	Method     string  `dynamodbav:"Method"`
	CreatedAt  string  `dynamodbav:"CreatedAt"`
	// GSI fields
	GSI1PK     string  `dynamodbav:"GSI1PK"` // CAT#{categoryID}
	GSI1SK     string  `dynamodbav:"GSI1SK"` // NODE#{nodeID}
}

// Enhanced category operations

// FindCategoriesByLevel retrieves categories at a specific hierarchy level
func (r *ddbRepository) FindCategoriesByLevel(ctx context.Context, userID string, level int) ([]domain.Category, error) {
	// Use GSI to query by level
	input := &dynamodb.QueryInput{
		TableName:              aws.String(r.config.TableName),
		IndexName:              aws.String(r.config.IndexName),
		KeyConditionExpression: aws.String("GSI1PK = :gsi1pk"),
		FilterExpression:       aws.String("UserID = :userid AND #level = :level"),
		ExpressionAttributeNames: map[string]string{
			"#level": "Level",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":gsi1pk": &types.AttributeValueMemberS{Value: fmt.Sprintf("CAT_LEVEL#%d", level)},
			":userid": &types.AttributeValueMemberS{Value: userID},
			":level":  &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", level)},
		},
	}

	result, err := r.dbClient.Query(ctx, input)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to query categories by level")
	}

	var categories []domain.Category
	for _, item := range result.Items {
		var ddbCat ddbEnhancedCategory
		if err := attributevalue.UnmarshalMap(item, &ddbCat); err != nil {
			log.Printf("Failed to unmarshal category: %v", err)
			continue
		}

		category := r.toDomainCategory(ddbCat)
		categories = append(categories, category)
	}

	return categories, nil
}

// Category hierarchy operations

// CreateCategoryHierarchy creates a parent-child relationship between categories
func (r *ddbRepository) CreateCategoryHierarchy(ctx context.Context, hierarchy domain.CategoryHierarchy) error {
	ddbHierarchy := ddbCategoryHierarchy{
		PK:        fmt.Sprintf("USER#%s", hierarchy.UserID),
		SK:        fmt.Sprintf("HIERARCHY#PARENT#%s#CHILD#%s", hierarchy.ParentID, hierarchy.ChildID),
		UserID:    hierarchy.UserID,
		ParentID:  hierarchy.ParentID,
		ChildID:   hierarchy.ChildID,
		CreatedAt: hierarchy.CreatedAt.Format(time.RFC3339),
	}

	item, err := attributevalue.MarshalMap(ddbHierarchy)
	if err != nil {
		return appErrors.Wrap(err, "failed to marshal hierarchy")
	}

	_, err = r.dbClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.config.TableName),
		Item:      item,
	})

	return appErrors.Wrap(err, "failed to create category hierarchy")
}

// DeleteCategoryHierarchy removes a parent-child relationship
func (r *ddbRepository) DeleteCategoryHierarchy(ctx context.Context, userID, parentID, childID string) error {
	_, err := r.dbClient.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(r.config.TableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s", userID)},
			"SK": &types.AttributeValueMemberS{Value: fmt.Sprintf("HIERARCHY#PARENT#%s#CHILD#%s", parentID, childID)},
		},
	})

	return appErrors.Wrap(err, "failed to delete category hierarchy")
}

// FindChildCategories retrieves all child categories of a parent
func (r *ddbRepository) FindChildCategories(ctx context.Context, userID, parentID string) ([]domain.Category, error) {
	// First find hierarchy relationships
	input := &dynamodb.QueryInput{
		TableName:              aws.String(r.config.TableName),
		KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :sk_prefix)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk":        &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s", userID)},
			":sk_prefix": &types.AttributeValueMemberS{Value: fmt.Sprintf("HIERARCHY#PARENT#%s#", parentID)},
		},
	}

	result, err := r.dbClient.Query(ctx, input)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to query child hierarchy")
	}

	var childIDs []string
	for _, item := range result.Items {
		var hierarchy ddbCategoryHierarchy
		if err := attributevalue.UnmarshalMap(item, &hierarchy); err != nil {
			continue
		}
		childIDs = append(childIDs, hierarchy.ChildID)
	}

	if len(childIDs) == 0 {
		return []domain.Category{}, nil
	}

	// Now fetch the actual categories
	var categories []domain.Category
	for _, childID := range childIDs {
		category, err := r.FindCategoryByID(ctx, userID, childID)
		if err != nil {
			log.Printf("Failed to fetch child category %s: %v", childID, err)
			continue
		}
		if category != nil {
			categories = append(categories, *category)
		}
	}

	return categories, nil
}

// FindParentCategory retrieves the parent category of a child
func (r *ddbRepository) FindParentCategory(ctx context.Context, userID, childID string) (*domain.Category, error) {
	// Query for hierarchy relationships where this is the child
	input := &dynamodb.QueryInput{
		TableName:              aws.String(r.config.TableName),
		KeyConditionExpression: aws.String("PK = :pk AND contains(SK, :child_id)"),
		FilterExpression:       aws.String("contains(SK, :hierarchy_prefix)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk":               &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s", userID)},
			":child_id":         &types.AttributeValueMemberS{Value: fmt.Sprintf("CHILD#%s", childID)},
			":hierarchy_prefix": &types.AttributeValueMemberS{Value: "HIERARCHY#"},
		},
	}

	result, err := r.dbClient.Query(ctx, input)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to query parent hierarchy")
	}

	if len(result.Items) == 0 {
		return nil, nil // No parent found
	}

	var hierarchy ddbCategoryHierarchy
	if err := attributevalue.UnmarshalMap(result.Items[0], &hierarchy); err != nil {
		return nil, appErrors.Wrap(err, "failed to unmarshal hierarchy")
	}

	return r.FindCategoryByID(ctx, userID, hierarchy.ParentID)
}

// GetCategoryTree retrieves the complete category hierarchy
func (r *ddbRepository) GetCategoryTree(ctx context.Context, userID string) ([]domain.Category, error) {
	// Get all categories for the user
	categories, err := r.FindCategories(ctx, repository.CategoryQuery{UserID: userID})
	if err != nil {
		return nil, err
	}

	// Sort by level to ensure proper hierarchy order
	// This is a simple implementation - in production you might want more sophisticated sorting
	var sortedCategories []domain.Category
	for level := 0; level <= 2; level++ { // Support up to 3 levels
		for _, cat := range categories {
			if cat.Level == level {
				sortedCategories = append(sortedCategories, cat)
			}
		}
	}

	return sortedCategories, nil
}

// Node-Category operations

// AssignNodeToCategory creates a relationship between a node and category
func (r *ddbRepository) AssignNodeToCategory(ctx context.Context, mapping domain.NodeCategory) error {
	ddbMapping := ddbNodeCategory{
		PK:         fmt.Sprintf("USER#%s", mapping.UserID),
		SK:         fmt.Sprintf("NODE_CAT#NODE#%s#CAT#%s", mapping.NodeID, mapping.CategoryID),
		UserID:     mapping.UserID,
		NodeID:     mapping.NodeID,
		CategoryID: mapping.CategoryID,
		Confidence: mapping.Confidence,
		Method:     mapping.Method,
		CreatedAt:  mapping.CreatedAt.Format(time.RFC3339),
		GSI1PK:     fmt.Sprintf("CAT#%s", mapping.CategoryID),
		GSI1SK:     fmt.Sprintf("NODE#%s", mapping.NodeID),
	}

	item, err := attributevalue.MarshalMap(ddbMapping)
	if err != nil {
		return appErrors.Wrap(err, "failed to marshal node-category mapping")
	}

	_, err = r.dbClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.config.TableName),
		Item:      item,
	})

	return appErrors.Wrap(err, "failed to assign node to category")
}

// RemoveNodeFromCategory removes a relationship between a node and category
func (r *ddbRepository) RemoveNodeFromCategory(ctx context.Context, userID, nodeID, categoryID string) error {
	_, err := r.dbClient.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(r.config.TableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s", userID)},
			"SK": &types.AttributeValueMemberS{Value: fmt.Sprintf("NODE_CAT#NODE#%s#CAT#%s", nodeID, categoryID)},
		},
	})

	return appErrors.Wrap(err, "failed to remove node from category")
}

// FindNodesByCategory retrieves all nodes in a specific category
func (r *ddbRepository) FindNodesByCategory(ctx context.Context, userID, categoryID string) ([]domain.Node, error) {
	// Use GSI to find node-category mappings
	input := &dynamodb.QueryInput{
		TableName:              aws.String(r.config.TableName),
		IndexName:              aws.String(r.config.IndexName),
		KeyConditionExpression: aws.String("GSI1PK = :gsi1pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":gsi1pk": &types.AttributeValueMemberS{Value: fmt.Sprintf("CAT#%s", categoryID)},
		},
	}

	result, err := r.dbClient.Query(ctx, input)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to query nodes by category")
	}

	var nodeIDs []string
	for _, item := range result.Items {
		var mapping ddbNodeCategory
		if err := attributevalue.UnmarshalMap(item, &mapping); err != nil {
			continue
		}
		nodeIDs = append(nodeIDs, mapping.NodeID)
	}

	if len(nodeIDs) == 0 {
		return []domain.Node{}, nil
	}

	// Fetch the actual nodes
	var nodes []domain.Node
	for _, nodeID := range nodeIDs {
		node, err := r.FindNodeByID(ctx, userID, nodeID)
		if err != nil {
			log.Printf("Failed to fetch node %s: %v", nodeID, err)
			continue
		}
		if node != nil {
			nodes = append(nodes, *node)
		}
	}

	return nodes, nil
}

// FindCategoriesForNode retrieves all categories that contain a specific node
func (r *ddbRepository) FindCategoriesForNode(ctx context.Context, userID, nodeID string) ([]domain.Category, error) {
	// Query for node-category mappings
	input := &dynamodb.QueryInput{
		TableName:              aws.String(r.config.TableName),
		KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :sk_prefix)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk":        &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s", userID)},
			":sk_prefix": &types.AttributeValueMemberS{Value: fmt.Sprintf("NODE_CAT#NODE#%s#", nodeID)},
		},
	}

	result, err := r.dbClient.Query(ctx, input)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to query categories for node")
	}

	var categoryIDs []string
	for _, item := range result.Items {
		var mapping ddbNodeCategory
		if err := attributevalue.UnmarshalMap(item, &mapping); err != nil {
			continue
		}
		categoryIDs = append(categoryIDs, mapping.CategoryID)
	}

	if len(categoryIDs) == 0 {
		return []domain.Category{}, nil
	}

	// Fetch the actual categories
	var categories []domain.Category
	for _, categoryID := range categoryIDs {
		category, err := r.FindCategoryByID(ctx, userID, categoryID)
		if err != nil {
			log.Printf("Failed to fetch category %s: %v", categoryID, err)
			continue
		}
		if category != nil {
			categories = append(categories, *category)
		}
	}

	return categories, nil
}

// Batch operations

// BatchAssignCategories efficiently assigns multiple nodes to categories
func (r *ddbRepository) BatchAssignCategories(ctx context.Context, mappings []domain.NodeCategory) error {
	if len(mappings) == 0 {
		return nil
	}

	// DynamoDB batch write limit is 25 items
	const batchSize = 25
	
	for i := 0; i < len(mappings); i += batchSize {
		end := i + batchSize
		if end > len(mappings) {
			end = len(mappings)
		}

		batch := mappings[i:end]
		if err := r.batchWriteNodeCategories(ctx, batch); err != nil {
			return err
		}
	}

	return nil
}

// batchWriteNodeCategories writes a batch of node-category mappings
func (r *ddbRepository) batchWriteNodeCategories(ctx context.Context, mappings []domain.NodeCategory) error {
	var writeRequests []types.WriteRequest

	for _, mapping := range mappings {
		ddbMapping := ddbNodeCategory{
			PK:         fmt.Sprintf("USER#%s", mapping.UserID),
			SK:         fmt.Sprintf("NODE_CAT#NODE#%s#CAT#%s", mapping.NodeID, mapping.CategoryID),
			UserID:     mapping.UserID,
			NodeID:     mapping.NodeID,
			CategoryID: mapping.CategoryID,
			Confidence: mapping.Confidence,
			Method:     mapping.Method,
			CreatedAt:  mapping.CreatedAt.Format(time.RFC3339),
			GSI1PK:     fmt.Sprintf("CAT#%s", mapping.CategoryID),
			GSI1SK:     fmt.Sprintf("NODE#%s", mapping.NodeID),
		}

		item, err := attributevalue.MarshalMap(ddbMapping)
		if err != nil {
			return appErrors.Wrap(err, "failed to marshal node-category mapping")
		}

		writeRequests = append(writeRequests, types.WriteRequest{
			PutRequest: &types.PutRequest{
				Item: item,
			},
		})
	}

	_, err := r.dbClient.BatchWriteItem(ctx, &dynamodb.BatchWriteItemInput{
		RequestItems: map[string][]types.WriteRequest{
			r.config.TableName: writeRequests,
		},
	})

	return appErrors.Wrap(err, "failed to batch write node-category mappings")
}

// UpdateCategoryNoteCounts updates the note counts for multiple categories
func (r *ddbRepository) UpdateCategoryNoteCounts(ctx context.Context, userID string, categoryCounts map[string]int) error {
	// Update each category's note count
	for categoryID, count := range categoryCounts {
		err := r.updateSingleCategoryNoteCount(ctx, userID, categoryID, count)
		if err != nil {
			log.Printf("Failed to update note count for category %s: %v", categoryID, err)
		}
	}

	return nil
}

// updateSingleCategoryNoteCount updates the note count for a single category
func (r *ddbRepository) updateSingleCategoryNoteCount(ctx context.Context, userID, categoryID string, count int) error {
	_, err := r.dbClient.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(r.config.TableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s", userID)},
			"SK": &types.AttributeValueMemberS{Value: fmt.Sprintf("CATEGORY#%s", categoryID)},
		},
		UpdateExpression: aws.String("SET NoteCount = :count, UpdatedAt = :updated"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":count":   &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", count)},
			":updated": &types.AttributeValueMemberS{Value: time.Now().Format(time.RFC3339)},
		},
	})

	return appErrors.Wrap(err, "failed to update category note count")
}

// Helper functions

// toDomainCategory converts a DynamoDB category to domain model
func (r *ddbRepository) toDomainCategory(ddbCat ddbEnhancedCategory) domain.Category {
	createdAt, _ := time.Parse(time.RFC3339, ddbCat.CreatedAt)
	updatedAt, _ := time.Parse(time.RFC3339, ddbCat.UpdatedAt)

	return domain.Category{
		ID:          ddbCat.CategoryID,
		UserID:      ddbCat.UserID,
		Title:       ddbCat.Title,
		Description: ddbCat.Description,
		Level:       ddbCat.Level,
		ParentID:    ddbCat.ParentID,
		Color:       ddbCat.Color,
		Icon:        ddbCat.Icon,
		AIGenerated: ddbCat.AIGenerated,
		NoteCount:   ddbCat.NoteCount,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}
}

// Override CreateCategory to use enhanced structure
func (r *ddbRepository) CreateEnhancedCategory(ctx context.Context, category domain.Category) error {
	pk := fmt.Sprintf("USER#%s", category.UserID)
	sk := fmt.Sprintf("CATEGORY#%s", category.ID)

	ddbCategory := ddbEnhancedCategory{
		PK:          pk,
		SK:          sk,
		CategoryID:  category.ID,
		UserID:      category.UserID,
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
		GSI1PK:      fmt.Sprintf("CAT_LEVEL#%d", category.Level),
		GSI1SK:      fmt.Sprintf("CAT#%s", category.ID),
	}

	item, err := attributevalue.MarshalMap(ddbCategory)
	if err != nil {
		return appErrors.Wrap(err, "failed to marshal enhanced category")
	}

	_, err = r.dbClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.config.TableName),
		Item:      item,
	})

	return appErrors.Wrap(err, "failed to create enhanced category")
}

// UpdateEnhancedCategory updates an existing category with enhanced fields
func (r *ddbRepository) UpdateEnhancedCategory(ctx context.Context, category domain.Category) error {
	pk := fmt.Sprintf("USER#%s", category.UserID)
	sk := fmt.Sprintf("CATEGORY#%s", category.ID)

	_, err := r.dbClient.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(r.config.TableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: pk},
			"SK": &types.AttributeValueMemberS{Value: sk},
		},
		UpdateExpression: aws.String("SET Title = :title, Description = :desc, #level = :level, ParentID = :parent, Color = :color, Icon = :icon, AIGenerated = :ai, NoteCount = :count, UpdatedAt = :updated"),
		ExpressionAttributeNames: map[string]string{
			"#level": "Level", // Level is a reserved word in DynamoDB
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":title":   &types.AttributeValueMemberS{Value: category.Title},
			":desc":    &types.AttributeValueMemberS{Value: category.Description},
			":level":   &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", category.Level)},
			":ai":      &types.AttributeValueMemberBOOL{Value: category.AIGenerated},
			":count":   &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", category.NoteCount)},
			":updated": &types.AttributeValueMemberS{Value: category.UpdatedAt.Format(time.RFC3339)},
		},
	})

	// Handle optional fields
	if category.ParentID != nil {
		_, err = r.dbClient.UpdateItem(ctx, &dynamodb.UpdateItemInput{
			TableName: aws.String(r.config.TableName),
			Key: map[string]types.AttributeValue{
				"PK": &types.AttributeValueMemberS{Value: pk},
				"SK": &types.AttributeValueMemberS{Value: sk},
			},
			UpdateExpression: aws.String("SET ParentID = :parent"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":parent": &types.AttributeValueMemberS{Value: *category.ParentID},
			},
		})
	}

	if category.Color != nil {
		_, err = r.dbClient.UpdateItem(ctx, &dynamodb.UpdateItemInput{
			TableName: aws.String(r.config.TableName),
			Key: map[string]types.AttributeValue{
				"PK": &types.AttributeValueMemberS{Value: pk},
				"SK": &types.AttributeValueMemberS{Value: sk},
			},
			UpdateExpression: aws.String("SET Color = :color"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":color": &types.AttributeValueMemberS{Value: *category.Color},
			},
		})
	}

	if category.Icon != nil {
		_, err = r.dbClient.UpdateItem(ctx, &dynamodb.UpdateItemInput{
			TableName: aws.String(r.config.TableName),
			Key: map[string]types.AttributeValue{
				"PK": &types.AttributeValueMemberS{Value: pk},
				"SK": &types.AttributeValueMemberS{Value: sk},
			},
			UpdateExpression: aws.String("SET Icon = :icon"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":icon": &types.AttributeValueMemberS{Value: *category.Icon},
			},
		})
	}

	return appErrors.Wrap(err, "failed to update enhanced category")
}