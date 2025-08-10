package repository

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func TestPageRequest(t *testing.T) {
	t.Run("Should create PageRequest with defaults", func(t *testing.T) {
		pageReq := NewPageRequest(0, "")
		if pageReq.GetEffectiveLimit() != DefaultPageSize {
			t.Errorf("Expected default limit %d, got %d", DefaultPageSize, pageReq.GetEffectiveLimit())
		}
		if pageReq.HasNextToken() {
			t.Error("Expected HasNextToken to be false for empty token")
		}
	})

	t.Run("Should enforce max page size", func(t *testing.T) {
		pageReq := NewPageRequest(999, "")
		if pageReq.GetEffectiveLimit() != DefaultPageSize {
			t.Errorf("Expected enforced limit %d, got %d", DefaultPageSize, pageReq.GetEffectiveLimit())
		}
	})

	t.Run("Should allow valid limits", func(t *testing.T) {
		pageReq := NewPageRequest(50, "")
		if pageReq.GetEffectiveLimit() != 50 {
			t.Errorf("Expected limit 50, got %d", pageReq.GetEffectiveLimit())
		}
	})
}

func TestLastEvaluatedKey(t *testing.T) {
	t.Run("Should encode and decode tokens", func(t *testing.T) {
		originalKey := LastEvaluatedKey{
			PK:     "USER#123#NODE#456",
			SK:     "METADATA#v0",
			GSI2PK: "USER#123#EDGE",
			GSI2SK: "NODE#456#TARGET#789",
		}

		// Encode
		token := EncodeNextToken(originalKey)
		if token == "" {
			t.Error("Expected non-empty token")
		}

		// Decode
		decodedKey, err := DecodeNextToken(token)
		if err != nil {
			t.Fatalf("Failed to decode token: %v", err)
		}

		if decodedKey.PK != originalKey.PK {
			t.Errorf("Expected PK %s, got %s", originalKey.PK, decodedKey.PK)
		}
		if decodedKey.SK != originalKey.SK {
			t.Errorf("Expected SK %s, got %s", originalKey.SK, decodedKey.SK)
		}
		if decodedKey.GSI2PK != originalKey.GSI2PK {
			t.Errorf("Expected GSI2PK %s, got %s", originalKey.GSI2PK, decodedKey.GSI2PK)
		}
		if decodedKey.GSI2SK != originalKey.GSI2SK {
			t.Errorf("Expected GSI2SK %s, got %s", originalKey.GSI2SK, decodedKey.GSI2SK)
		}
	})

	t.Run("Should convert to/from DynamoDB key", func(t *testing.T) {
		originalKey := LastEvaluatedKey{
			PK:     "USER#123#NODE#456",
			SK:     "METADATA#v0",
			GSI2PK: "USER#123#EDGE",
			GSI2SK: "NODE#456#TARGET#789",
		}

		// Convert to DynamoDB key
		dynamoKey := originalKey.ToDynamoDBKey()
		if len(dynamoKey) != 4 {
			t.Errorf("Expected 4 attributes, got %d", len(dynamoKey))
		}

		// Convert back from DynamoDB key
		convertedKey := FromDynamoDBKey(dynamoKey)

		if convertedKey.PK != originalKey.PK {
			t.Errorf("Expected PK %s, got %s", originalKey.PK, convertedKey.PK)
		}
		if convertedKey.SK != originalKey.SK {
			t.Errorf("Expected SK %s, got %s", originalKey.SK, convertedKey.SK)
		}
		if convertedKey.GSI2PK != originalKey.GSI2PK {
			t.Errorf("Expected GSI2PK %s, got %s", originalKey.GSI2PK, convertedKey.GSI2PK)
		}
		if convertedKey.GSI2SK != originalKey.GSI2SK {
			t.Errorf("Expected GSI2SK %s, got %s", originalKey.GSI2SK, convertedKey.GSI2SK)
		}
	})
}

func TestCreatePageResponse(t *testing.T) {
	t.Run("Should create response with no next page", func(t *testing.T) {
		items := []string{"item1", "item2"}
		response := CreatePageResponse(items, nil, false)

		if response.Items.([]string)[0] != items[0] || response.Items.([]string)[1] != items[1] {
			t.Error("Expected items to match")
		}
		if response.HasMore {
			t.Error("Expected HasMore to be false")
		}
		if response.NextToken != "" {
			t.Error("Expected empty NextToken when no more pages")
		}
	})

	t.Run("Should create response with next page", func(t *testing.T) {
		items := []string{"item1", "item2"}
		lastKey := map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: "USER#123#NODE#456"},
			"SK": &types.AttributeValueMemberS{Value: "METADATA#v0"},
		}

		response := CreatePageResponse(items, lastKey, true)

		if response.Items.([]string)[0] != items[0] || response.Items.([]string)[1] != items[1] {
			t.Error("Expected items to match")
		}
		if !response.HasMore {
			t.Error("Expected HasMore to be true")
		}
		if response.NextToken == "" {
			t.Error("Expected non-empty NextToken when more pages exist")
		}
	})
}

func TestDecodeEmptyToken(t *testing.T) {
	t.Run("Should handle empty token", func(t *testing.T) {
		key, err := DecodeNextToken("")
		if err != nil {
			t.Errorf("Expected no error for empty token, got %v", err)
		}
		if key != nil {
			t.Error("Expected nil key for empty token")
		}
	})

	t.Run("Should handle invalid token", func(t *testing.T) {
		_, err := DecodeNextToken("invalid-token")
		if err == nil {
			t.Error("Expected error for invalid token")
		}
	})
}