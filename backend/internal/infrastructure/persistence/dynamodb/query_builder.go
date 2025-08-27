// Package dynamodb provides query builder utilities for DynamoDB operations.
//
// This file contains reusable query building patterns to reduce code duplication
// and provide a fluent interface for constructing DynamoDB queries.
package dynamodb

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// ============================================================================
// QUERY BUILDER
// ============================================================================

// QueryBuilder provides a fluent interface for building DynamoDB queries
type QueryBuilder struct {
	tableName          string
	indexName          *string
	keyCondition       *expression.KeyConditionBuilder
	filterCondition    *expression.ConditionBuilder
	projectionFields   []string
	limit              *int32
	scanForward        *bool
	exclusiveStartKey  map[string]types.AttributeValue
	consistentRead     *bool
	expressionBuilder  expression.Builder
}

// NewQueryBuilder creates a new query builder instance
func NewQueryBuilder(tableName string) *QueryBuilder {
	return &QueryBuilder{
		tableName:        tableName,
		expressionBuilder: expression.NewBuilder(),
	}
}

// WithIndex sets the index name for the query
func (qb *QueryBuilder) WithIndex(indexName string) *QueryBuilder {
	qb.indexName = &indexName
	return qb
}

// WithUserPartition adds a user partition key condition
func (qb *QueryBuilder) WithUserPartition(userID string) *QueryBuilder {
	keyExpr := expression.Key("PK").Equal(expression.Value(BuildUserPK(userID)))
	qb.keyCondition = &keyExpr
	return qb
}

// WithSortKeyBeginsWith adds a sort key begins with condition
func (qb *QueryBuilder) WithSortKeyBeginsWith(prefix string) *QueryBuilder {
	if qb.keyCondition == nil {
		keyExpr := expression.Key("SK").BeginsWith(prefix)
		qb.keyCondition = &keyExpr
	} else {
		keyExpr := qb.keyCondition.And(expression.Key("SK").BeginsWith(prefix))
		qb.keyCondition = &keyExpr
	}
	return qb
}

// WithSortKeyBetween adds a sort key between condition
func (qb *QueryBuilder) WithSortKeyBetween(start, end string) *QueryBuilder {
	if qb.keyCondition == nil {
		keyExpr := expression.Key("SK").Between(expression.Value(start), expression.Value(end))
		qb.keyCondition = &keyExpr
	} else {
		keyExpr := qb.keyCondition.And(expression.Key("SK").Between(expression.Value(start), expression.Value(end)))
		qb.keyCondition = &keyExpr
	}
	return qb
}

// WithFilter adds a filter expression
func (qb *QueryBuilder) WithFilter(filter expression.ConditionBuilder) *QueryBuilder {
	if qb.filterCondition == nil {
		qb.filterCondition = &filter
	} else {
		combined := qb.filterCondition.And(filter)
		qb.filterCondition = &combined
	}
	return qb
}

// WithAttributeFilter adds an attribute equals filter
func (qb *QueryBuilder) WithAttributeFilter(attribute string, value interface{}) *QueryBuilder {
	filter := expression.Name(attribute).Equal(expression.Value(value))
	return qb.WithFilter(filter)
}

// WithAttributeNotExists adds an attribute not exists filter
func (qb *QueryBuilder) WithAttributeNotExists(attribute string) *QueryBuilder {
	filter := expression.AttributeNotExists(expression.Name(attribute))
	return qb.WithFilter(filter)
}

// WithAttributeExists adds an attribute exists filter
func (qb *QueryBuilder) WithAttributeExists(attribute string) *QueryBuilder {
	filter := expression.AttributeExists(expression.Name(attribute))
	return qb.WithFilter(filter)
}

// WithProjection sets the projection fields
func (qb *QueryBuilder) WithProjection(fields ...string) *QueryBuilder {
	qb.projectionFields = fields
	return qb
}

// WithLimit sets the query limit
func (qb *QueryBuilder) WithLimit(limit int32) *QueryBuilder {
	qb.limit = &limit
	return qb
}

// WithScanDirection sets the scan direction (true = forward, false = backward)
func (qb *QueryBuilder) WithScanDirection(forward bool) *QueryBuilder {
	qb.scanForward = &forward
	return qb
}

// WithExclusiveStartKey sets the exclusive start key for pagination
func (qb *QueryBuilder) WithExclusiveStartKey(key map[string]types.AttributeValue) *QueryBuilder {
	qb.exclusiveStartKey = key
	return qb
}

// WithConsistentRead enables consistent read
func (qb *QueryBuilder) WithConsistentRead(consistent bool) *QueryBuilder {
	qb.consistentRead = &consistent
	return qb
}

// Build constructs the final QueryInput
func (qb *QueryBuilder) Build() (*dynamodb.QueryInput, error) {
	if qb.keyCondition == nil {
		return nil, fmt.Errorf("key condition is required for query")
	}

	// Build expression
	builder := expression.NewBuilder().WithKeyCondition(*qb.keyCondition)
	
	if qb.filterCondition != nil {
		builder = builder.WithFilter(*qb.filterCondition)
	}
	
	if len(qb.projectionFields) > 0 {
		var nameBuilders []expression.NameBuilder
		for _, field := range qb.projectionFields {
			nameBuilders = append(nameBuilders, expression.Name(field))
		}
		builder = builder.WithProjection(expression.ProjectionBuilder{}.AddNames(nameBuilders...))
	}
	
	expr, err := builder.Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build expression: %w", err)
	}

	input := &dynamodb.QueryInput{
		TableName:                 aws.String(qb.tableName),
		KeyConditionExpression:    expr.KeyCondition(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
	}
	
	if qb.indexName != nil {
		input.IndexName = qb.indexName
	}
	
	if qb.filterCondition != nil {
		input.FilterExpression = expr.Filter()
	}
	
	if len(qb.projectionFields) > 0 {
		input.ProjectionExpression = expr.Projection()
	}
	
	if qb.limit != nil {
		input.Limit = qb.limit
	}
	
	if qb.scanForward != nil {
		input.ScanIndexForward = qb.scanForward
	}
	
	if qb.exclusiveStartKey != nil {
		input.ExclusiveStartKey = qb.exclusiveStartKey
	}
	
	if qb.consistentRead != nil {
		input.ConsistentRead = qb.consistentRead
	}
	
	return input, nil
}

// ============================================================================
// SCAN BUILDER
// ============================================================================

// ScanBuilder provides a fluent interface for building DynamoDB scans
type ScanBuilder struct {
	tableName         string
	indexName         *string
	filterCondition   *expression.ConditionBuilder
	projectionFields  []string
	limit             *int32
	exclusiveStartKey map[string]types.AttributeValue
	consistentRead    *bool
	segment           *int32
	totalSegments     *int32
}

// NewScanBuilder creates a new scan builder instance
func NewScanBuilder(tableName string) *ScanBuilder {
	return &ScanBuilder{
		tableName: tableName,
	}
}

// WithIndex sets the index name for the scan
func (sb *ScanBuilder) WithIndex(indexName string) *ScanBuilder {
	sb.indexName = &indexName
	return sb
}

// WithFilter adds a filter expression
func (sb *ScanBuilder) WithFilter(filter expression.ConditionBuilder) *ScanBuilder {
	if sb.filterCondition == nil {
		sb.filterCondition = &filter
	} else {
		combined := sb.filterCondition.And(filter)
		sb.filterCondition = &combined
	}
	return sb
}

// WithUserFilter adds a user filter
func (sb *ScanBuilder) WithUserFilter(userID string) *ScanBuilder {
	filter := expression.Name("PK").Equal(expression.Value(BuildUserPK(userID)))
	return sb.WithFilter(filter)
}

// WithEntityTypeFilter adds an entity type filter
func (sb *ScanBuilder) WithEntityTypeFilter(entityType string) *ScanBuilder {
	filter := expression.Name("EntityType").Equal(expression.Value(entityType))
	return sb.WithFilter(filter)
}

// WithProjection sets the projection fields
func (sb *ScanBuilder) WithProjection(fields ...string) *ScanBuilder {
	sb.projectionFields = fields
	return sb
}

// WithLimit sets the scan limit
func (sb *ScanBuilder) WithLimit(limit int32) *ScanBuilder {
	sb.limit = &limit
	return sb
}

// WithExclusiveStartKey sets the exclusive start key for pagination
func (sb *ScanBuilder) WithExclusiveStartKey(key map[string]types.AttributeValue) *ScanBuilder {
	sb.exclusiveStartKey = key
	return sb
}

// WithConsistentRead enables consistent read
func (sb *ScanBuilder) WithConsistentRead(consistent bool) *ScanBuilder {
	sb.consistentRead = &consistent
	return sb
}

// WithParallelScan configures parallel scanning
func (sb *ScanBuilder) WithParallelScan(segment, totalSegments int32) *ScanBuilder {
	sb.segment = &segment
	sb.totalSegments = &totalSegments
	return sb
}

// Build constructs the final ScanInput
func (sb *ScanBuilder) Build() (*dynamodb.ScanInput, error) {
	builder := expression.NewBuilder()
	
	if sb.filterCondition != nil {
		builder = builder.WithFilter(*sb.filterCondition)
	}
	
	if len(sb.projectionFields) > 0 {
		var nameBuilders []expression.NameBuilder
		for _, field := range sb.projectionFields {
			nameBuilders = append(nameBuilders, expression.Name(field))
		}
		builder = builder.WithProjection(expression.ProjectionBuilder{}.AddNames(nameBuilders...))
	}
	
	expr, err := builder.Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build expression: %w", err)
	}
	
	input := &dynamodb.ScanInput{
		TableName: aws.String(sb.tableName),
	}
	
	if sb.indexName != nil {
		input.IndexName = sb.indexName
	}
	
	if sb.filterCondition != nil {
		input.FilterExpression = expr.Filter()
		input.ExpressionAttributeNames = expr.Names()
		input.ExpressionAttributeValues = expr.Values()
	}
	
	if len(sb.projectionFields) > 0 {
		input.ProjectionExpression = expr.Projection()
		if input.ExpressionAttributeNames == nil {
			input.ExpressionAttributeNames = expr.Names()
		}
	}
	
	if sb.limit != nil {
		input.Limit = sb.limit
	}
	
	if sb.exclusiveStartKey != nil {
		input.ExclusiveStartKey = sb.exclusiveStartKey
	}
	
	if sb.consistentRead != nil {
		input.ConsistentRead = sb.consistentRead
	}
	
	if sb.segment != nil && sb.totalSegments != nil {
		input.Segment = sb.segment
		input.TotalSegments = sb.totalSegments
	}
	
	return input, nil
}

// ============================================================================
// UPDATE EXPRESSION BUILDER
// ============================================================================

// UpdateBuilder provides a fluent interface for building update expressions
type UpdateBuilder struct {
	setExpressions    []string
	removeExpressions []string
	addExpressions    []string
	deleteExpressions []string
	attrNames         map[string]string
	attrValues        map[string]types.AttributeValue
}

// NewUpdateBuilder creates a new update builder
func NewUpdateBuilder() *UpdateBuilder {
	return &UpdateBuilder{
		attrNames:  make(map[string]string),
		attrValues: make(map[string]types.AttributeValue),
	}
}

// Set adds a SET expression
func (ub *UpdateBuilder) Set(attribute string, value interface{}) *UpdateBuilder {
	placeholder := fmt.Sprintf("#%s", attribute)
	valuePlaceholder := fmt.Sprintf(":%s", attribute)
	
	ub.setExpressions = append(ub.setExpressions, fmt.Sprintf("%s = %s", placeholder, valuePlaceholder))
	ub.attrNames[placeholder] = attribute
	
	switch v := value.(type) {
	case string:
		ub.attrValues[valuePlaceholder] = &types.AttributeValueMemberS{Value: v}
	case int:
		ub.attrValues[valuePlaceholder] = &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", v)}
	case int64:
		ub.attrValues[valuePlaceholder] = &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", v)}
	case float64:
		ub.attrValues[valuePlaceholder] = &types.AttributeValueMemberN{Value: fmt.Sprintf("%f", v)}
	case bool:
		ub.attrValues[valuePlaceholder] = &types.AttributeValueMemberBOOL{Value: v}
	case []string:
		ub.attrValues[valuePlaceholder] = &types.AttributeValueMemberSS{Value: v}
	case types.AttributeValue:
		ub.attrValues[valuePlaceholder] = v
	}
	
	return ub
}

// Remove adds a REMOVE expression
func (ub *UpdateBuilder) Remove(attribute string) *UpdateBuilder {
	placeholder := fmt.Sprintf("#%s", attribute)
	ub.removeExpressions = append(ub.removeExpressions, placeholder)
	ub.attrNames[placeholder] = attribute
	return ub
}

// Add adds an ADD expression (for numeric increment or set addition)
func (ub *UpdateBuilder) Add(attribute string, value interface{}) *UpdateBuilder {
	placeholder := fmt.Sprintf("#%s", attribute)
	valuePlaceholder := fmt.Sprintf(":%s", attribute)
	
	ub.addExpressions = append(ub.addExpressions, fmt.Sprintf("%s %s", placeholder, valuePlaceholder))
	ub.attrNames[placeholder] = attribute
	
	switch v := value.(type) {
	case int:
		ub.attrValues[valuePlaceholder] = &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", v)}
	case int64:
		ub.attrValues[valuePlaceholder] = &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", v)}
	case []string:
		ub.attrValues[valuePlaceholder] = &types.AttributeValueMemberSS{Value: v}
	}
	
	return ub
}

// Delete adds a DELETE expression (for set deletion)
func (ub *UpdateBuilder) Delete(attribute string, value []string) *UpdateBuilder {
	placeholder := fmt.Sprintf("#%s", attribute)
	valuePlaceholder := fmt.Sprintf(":%s", attribute)
	
	ub.deleteExpressions = append(ub.deleteExpressions, fmt.Sprintf("%s %s", placeholder, valuePlaceholder))
	ub.attrNames[placeholder] = attribute
	ub.attrValues[valuePlaceholder] = &types.AttributeValueMemberSS{Value: value}
	
	return ub
}

// Build constructs the final update expression and attribute maps
func (ub *UpdateBuilder) Build() (string, map[string]string, map[string]types.AttributeValue) {
	var expressions []string
	
	if len(ub.setExpressions) > 0 {
		expressions = append(expressions, fmt.Sprintf("SET %s", strings.Join(ub.setExpressions, ", ")))
	}
	
	if len(ub.removeExpressions) > 0 {
		expressions = append(expressions, fmt.Sprintf("REMOVE %s", strings.Join(ub.removeExpressions, ", ")))
	}
	
	if len(ub.addExpressions) > 0 {
		expressions = append(expressions, fmt.Sprintf("ADD %s", strings.Join(ub.addExpressions, ", ")))
	}
	
	if len(ub.deleteExpressions) > 0 {
		expressions = append(expressions, fmt.Sprintf("DELETE %s", strings.Join(ub.deleteExpressions, ", ")))
	}
	
	return strings.Join(expressions, " "), ub.attrNames, ub.attrValues
}

// ============================================================================
// CONDITION BUILDER HELPERS
// ============================================================================

// BuildOptimisticLockCondition creates a condition for optimistic locking
func BuildOptimisticLockCondition(version int) expression.ConditionBuilder {
	return expression.Name("Version").Equal(expression.Value(version))
}

// BuildExistsCondition creates a condition that checks if an item exists
func BuildExistsCondition() expression.ConditionBuilder {
	return expression.AttributeExists(expression.Name("PK")).
		And(expression.AttributeExists(expression.Name("SK")))
}

// BuildNotExistsCondition creates a condition that checks if an item does not exist
func BuildNotExistsCondition() expression.ConditionBuilder {
	return expression.AttributeNotExists(expression.Name("PK")).
		And(expression.AttributeNotExists(expression.Name("SK")))
}

// BuildUserOwnershipCondition creates a condition that verifies user ownership
func BuildUserOwnershipCondition(userID string) expression.ConditionBuilder {
	return expression.Name("UserID").Equal(expression.Value(userID))
}

// ============================================================================
// COMMON QUERY PATTERNS
// ============================================================================

// QueryPattern represents a reusable query pattern
type QueryPattern struct {
	Name        string
	Description string
	Builder     func(params map[string]interface{}) *QueryBuilder
}

// CommonQueryPatterns provides pre-built query patterns
var CommonQueryPatterns = map[string]QueryPattern{
	"user_nodes": {
		Name:        "User Nodes Query",
		Description: "Query all nodes for a specific user",
		Builder: func(params map[string]interface{}) *QueryBuilder {
			userID := params["userID"].(string)
			return NewQueryBuilder(params["tableName"].(string)).
				WithUserPartition(userID).
				WithSortKeyBeginsWith("NODE#")
		},
	},
	"user_edges": {
		Name:        "User Edges Query",
		Description: "Query all edges for a specific user",
		Builder: func(params map[string]interface{}) *QueryBuilder {
			userID := params["userID"].(string)
			return NewQueryBuilder(params["tableName"].(string)).
				WithUserPartition(userID).
				WithSortKeyBeginsWith("EDGE#")
		},
	},
	"node_edges": {
		Name:        "Node Edges Query",
		Description: "Query all edges connected to a specific node",
		Builder: func(params map[string]interface{}) *QueryBuilder {
			userID := params["userID"].(string)
			nodeID := params["nodeID"].(string)
			return NewQueryBuilder(params["tableName"].(string)).
				WithUserPartition(fmt.Sprintf("%s#NODE#%s", userID, nodeID)).
				WithSortKeyBeginsWith("EDGE#")
		},
	},
	"recent_items": {
		Name:        "Recent Items Query",
		Description: "Query recent items with timestamp filtering",
		Builder: func(params map[string]interface{}) *QueryBuilder {
			userID := params["userID"].(string)
			days := params["days"].(int)
			cutoff := fmt.Sprintf("%d", days)
			
			return NewQueryBuilder(params["tableName"].(string)).
				WithUserPartition(userID).
				WithAttributeFilter("DaysOld", cutoff)
		},
	},
}