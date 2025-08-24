package dynamodb

import (
	"fmt"
	"brain2-backend/internal/domain/node"
	"brain2-backend/internal/domain/edge"
	"brain2-backend/internal/domain/category"
	appErrors "brain2-backend/pkg/errors"
	
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/aws"
)

// OperationType defines the type of transactional operation
type OperationType string

const (
	OperationTypePut    OperationType = "PUT"
	OperationTypeUpdate OperationType = "UPDATE"
	OperationTypeDelete OperationType = "DELETE"
	OperationTypeConditionCheck OperationType = "CONDITION_CHECK"
)

// Operation represents a transactional operation to be executed
type Operation struct {
	Type      OperationType
	TableName string
	Item      interface{} // For Put operations
	Key       map[string]types.AttributeValue // For Update/Delete/ConditionCheck
	UpdateExpression    string // For Update operations
	ConditionExpression string // For conditional operations
	ExpressionAttributeNames  map[string]string
	ExpressionAttributeValues map[string]types.AttributeValue
	ReturnValuesOnConditionCheckFailure string
}

// buildTransactItem converts an Operation into a DynamoDB TransactWriteItem
func (uow *DynamoDBUnitOfWork) buildTransactItem(op Operation) (*types.TransactWriteItem, error) {
	switch op.Type {
	case OperationTypePut:
		return uow.buildPutItem(op)
	case OperationTypeUpdate:
		return uow.buildUpdateItem(op)
	case OperationTypeDelete:
		return uow.buildDeleteItem(op)
	case OperationTypeConditionCheck:
		return uow.buildConditionCheckItem(op)
	default:
		return nil, appErrors.NewValidation(fmt.Sprintf("unsupported operation type: %s", op.Type))
	}
}

// buildPutItem creates a Put transaction item
func (uow *DynamoDBUnitOfWork) buildPutItem(op Operation) (*types.TransactWriteItem, error) {
	if op.Item == nil {
		return nil, appErrors.NewValidation("item is required for PUT operation")
	}
	
	// Marshal the item to DynamoDB attribute values
	av, err := attributevalue.MarshalMap(op.Item)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to marshal item for PUT operation")
	}
	
	putItem := &types.Put{
		TableName: aws.String(op.TableName),
		Item:      av,
	}
	
	// Add condition expression if provided (for optimistic locking)
	if op.ConditionExpression != "" {
		putItem.ConditionExpression = aws.String(op.ConditionExpression)
		
		if len(op.ExpressionAttributeNames) > 0 {
			putItem.ExpressionAttributeNames = op.ExpressionAttributeNames
		}
		
		if len(op.ExpressionAttributeValues) > 0 {
			putItem.ExpressionAttributeValues = op.ExpressionAttributeValues
		}
	}
	
	if op.ReturnValuesOnConditionCheckFailure != "" {
		putItem.ReturnValuesOnConditionCheckFailure = types.ReturnValuesOnConditionCheckFailure(op.ReturnValuesOnConditionCheckFailure)
	}
	
	return &types.TransactWriteItem{
		Put: putItem,
	}, nil
}

// buildUpdateItem creates an Update transaction item
func (uow *DynamoDBUnitOfWork) buildUpdateItem(op Operation) (*types.TransactWriteItem, error) {
	if len(op.Key) == 0 {
		return nil, appErrors.NewValidation("key is required for UPDATE operation")
	}
	
	if op.UpdateExpression == "" {
		return nil, appErrors.NewValidation("update expression is required for UPDATE operation")
	}
	
	updateItem := &types.Update{
		TableName:        aws.String(op.TableName),
		Key:             op.Key,
		UpdateExpression: aws.String(op.UpdateExpression),
	}
	
	// Add condition expression if provided
	if op.ConditionExpression != "" {
		updateItem.ConditionExpression = aws.String(op.ConditionExpression)
	}
	
	// Add expression attribute names
	if len(op.ExpressionAttributeNames) > 0 {
		updateItem.ExpressionAttributeNames = op.ExpressionAttributeNames
	}
	
	// Add expression attribute values
	if len(op.ExpressionAttributeValues) > 0 {
		updateItem.ExpressionAttributeValues = op.ExpressionAttributeValues
	}
	
	if op.ReturnValuesOnConditionCheckFailure != "" {
		updateItem.ReturnValuesOnConditionCheckFailure = types.ReturnValuesOnConditionCheckFailure(op.ReturnValuesOnConditionCheckFailure)
	}
	
	return &types.TransactWriteItem{
		Update: updateItem,
	}, nil
}

// buildDeleteItem creates a Delete transaction item
func (uow *DynamoDBUnitOfWork) buildDeleteItem(op Operation) (*types.TransactWriteItem, error) {
	if len(op.Key) == 0 {
		return nil, appErrors.NewValidation("key is required for DELETE operation")
	}
	
	deleteItem := &types.Delete{
		TableName: aws.String(op.TableName),
		Key:       op.Key,
	}
	
	// Add condition expression if provided (e.g., to ensure item exists)
	if op.ConditionExpression != "" {
		deleteItem.ConditionExpression = aws.String(op.ConditionExpression)
		
		if len(op.ExpressionAttributeNames) > 0 {
			deleteItem.ExpressionAttributeNames = op.ExpressionAttributeNames
		}
		
		if len(op.ExpressionAttributeValues) > 0 {
			deleteItem.ExpressionAttributeValues = op.ExpressionAttributeValues
		}
	}
	
	if op.ReturnValuesOnConditionCheckFailure != "" {
		deleteItem.ReturnValuesOnConditionCheckFailure = types.ReturnValuesOnConditionCheckFailure(op.ReturnValuesOnConditionCheckFailure)
	}
	
	return &types.TransactWriteItem{
		Delete: deleteItem,
	}, nil
}

// buildConditionCheckItem creates a ConditionCheck transaction item
func (uow *DynamoDBUnitOfWork) buildConditionCheckItem(op Operation) (*types.TransactWriteItem, error) {
	if len(op.Key) == 0 {
		return nil, appErrors.NewValidation("key is required for CONDITION_CHECK operation")
	}
	
	if op.ConditionExpression == "" {
		return nil, appErrors.NewValidation("condition expression is required for CONDITION_CHECK operation")
	}
	
	conditionCheck := &types.ConditionCheck{
		TableName:           aws.String(op.TableName),
		Key:                op.Key,
		ConditionExpression: aws.String(op.ConditionExpression),
	}
	
	if len(op.ExpressionAttributeNames) > 0 {
		conditionCheck.ExpressionAttributeNames = op.ExpressionAttributeNames
	}
	
	if len(op.ExpressionAttributeValues) > 0 {
		conditionCheck.ExpressionAttributeValues = op.ExpressionAttributeValues
	}
	
	if op.ReturnValuesOnConditionCheckFailure != "" {
		conditionCheck.ReturnValuesOnConditionCheckFailure = types.ReturnValuesOnConditionCheckFailure(op.ReturnValuesOnConditionCheckFailure)
	}
	
	return &types.TransactWriteItem{
		ConditionCheck: conditionCheck,
	}, nil
}

// AddOperation adds a generic operation to the transaction
func (uow *DynamoDBUnitOfWork) AddOperation(op Operation) error {
	if !uow.isActive {
		return appErrors.NewValidation("unit of work not active")
	}
	
	// Set default table name if not provided
	if op.TableName == "" {
		op.TableName = uow.tableName
	}
	
	item, err := uow.buildTransactItem(op)
	if err != nil {
		return appErrors.Wrap(err, "failed to build transaction item")
	}
	
	return uow.AddTransactItem(*item)
}

// Helper methods for common operations

// AddPutOperation adds a PUT operation to the transaction
func (uow *DynamoDBUnitOfWork) AddPutOperation(item interface{}, conditionExpression string) error {
	op := Operation{
		Type:                OperationType Put,
		TableName:           uow.tableName,
		Item:                item,
		ConditionExpression: conditionExpression,
	}
	return uow.AddOperation(op)
}

// AddUpdateOperation adds an UPDATE operation to the transaction
func (uow *DynamoDBUnitOfWork) AddUpdateOperation(key map[string]types.AttributeValue, updateExpression string, conditionExpression string, expressionAttributes map[string]interface{}) error {
	op := Operation{
		Type:                OperationTypeUpdate,
		TableName:           uow.tableName,
		Key:                 key,
		UpdateExpression:    updateExpression,
		ConditionExpression: conditionExpression,
	}
	
	// Separate attribute names and values
	if expressionAttributes != nil {
		op.ExpressionAttributeNames = make(map[string]string)
		op.ExpressionAttributeValues = make(map[string]types.AttributeValue)
		
		for k, v := range expressionAttributes {
			if k[0] == '#' {
				// This is an attribute name placeholder
				if strVal, ok := v.(string); ok {
					op.ExpressionAttributeNames[k] = strVal
				}
			} else if k[0] == ':' {
				// This is an attribute value placeholder
				av, err := attributevalue.Marshal(v)
				if err == nil {
					op.ExpressionAttributeValues[k] = av
				}
			}
		}
	}
	
	return uow.AddOperation(op)
}

// AddDeleteOperation adds a DELETE operation to the transaction
func (uow *DynamoDBUnitOfWork) AddDeleteOperation(key map[string]types.AttributeValue, conditionExpression string) error {
	op := Operation{
		Type:                OperationTypeDelete,
		TableName:           uow.tableName,
		Key:                 key,
		ConditionExpression: conditionExpression,
	}
	return uow.AddOperation(op)
}

// Domain-specific transaction helpers

// AddNodeSaveOperation adds a node save operation to the transaction
func (uow *DynamoDBUnitOfWork) AddNodeSaveOperation(n *node.Node) error {
	// Convert node to DynamoDB item format
	item := map[string]interface{}{
		"PK":        fmt.Sprintf("USER#%s", n.GetUserID()),
		"SK":        fmt.Sprintf("NODE#%s", n.GetID()),
		"Type":      "NODE",
		"Content":   n.GetContent(),
		"Tags":      n.GetTags(),
		"CreatedAt": n.GetCreatedAt(),
		"UpdatedAt": n.GetUpdatedAt(),
		"Version":   n.GetVersion(),
	}
	
	// Add optimistic locking condition
	condition := "attribute_not_exists(PK) OR Version < :newVersion"
	
	return uow.AddPutOperation(item, condition)
}

// AddEdgeSaveOperation adds an edge save operation to the transaction
func (uow *DynamoDBUnitOfWork) AddEdgeSaveOperation(e *edge.Edge) error {
	// Convert edge to DynamoDB item format
	item := map[string]interface{}{
		"PK":        fmt.Sprintf("USER#%s", e.GetUserID()),
		"SK":        fmt.Sprintf("EDGE#%s#%s", e.GetFromNodeID(), e.GetToNodeID()),
		"Type":      "EDGE",
		"FromNode":  e.GetFromNodeID(),
		"ToNode":    e.GetToNodeID(),
		"Weight":    e.GetWeight(),
		"CreatedAt": e.GetCreatedAt(),
	}
	
	return uow.AddPutOperation(item, "")
}

// AddCategorySaveOperation adds a category save operation to the transaction
func (uow *DynamoDBUnitOfWork) AddCategorySaveOperation(c *category.Category) error {
	// Convert category to DynamoDB item format
	item := map[string]interface{}{
		"PK":          fmt.Sprintf("USER#%s", c.GetUserID()),
		"SK":          fmt.Sprintf("CATEGORY#%s", c.GetID()),
		"Type":        "CATEGORY",
		"Title":       c.GetTitle(),
		"Description": c.GetDescription(),
		"Level":       c.GetLevel(),
		"CreatedAt":   c.GetCreatedAt(),
		"UpdatedAt":   c.GetUpdatedAt(),
	}
	
	return uow.AddPutOperation(item, "")
}