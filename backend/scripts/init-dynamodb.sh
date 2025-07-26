#!/bin/bash

# Initialize DynamoDB Local tables
# This script creates the necessary tables for the Brain2 application

DYNAMODB_ENDPOINT="http://localhost:8002"
TABLE_NAME="brain2-dev"
KEYWORD_INDEX_NAME="KeywordIndex"

echo "Initializing DynamoDB Local tables..."

# Wait for DynamoDB Local to be ready
echo "Waiting for DynamoDB Local to be ready..."
until curl -s ${DYNAMODB_ENDPOINT}/shell >/dev/null 2>&1; do
    echo "Waiting for DynamoDB Local..."
    sleep 2
done
echo "DynamoDB Local is ready!"

# Create the main table
echo "Creating table: ${TABLE_NAME}"
aws dynamodb create-table \
    --endpoint-url ${DYNAMODB_ENDPOINT} \
    --table-name ${TABLE_NAME} \
    --attribute-definitions \
        AttributeName=PK,AttributeType=S \
        AttributeName=SK,AttributeType=S \
        AttributeName=GSI1PK,AttributeType=S \
        AttributeName=GSI1SK,AttributeType=S \
    --key-schema \
        AttributeName=PK,KeyType=HASH \
        AttributeName=SK,KeyType=RANGE \
    --global-secondary-indexes \
        IndexName=${KEYWORD_INDEX_NAME},KeySchema='[{AttributeName=GSI1PK,KeyType=HASH},{AttributeName=GSI1SK,KeyType=RANGE}]',Projection='{ProjectionType=ALL}',ProvisionedThroughput='{ReadCapacityUnits=5,WriteCapacityUnits=5}' \
    --provisioned-throughput \
        ReadCapacityUnits=5,WriteCapacityUnits=5 \
    --region us-east-1

echo "Table creation initiated. Checking status..."

# Wait for table to be active
aws dynamodb wait table-exists \
    --endpoint-url ${DYNAMODB_ENDPOINT} \
    --table-name ${TABLE_NAME} \
    --region us-east-1

echo "Table ${TABLE_NAME} is now active!"
echo "DynamoDB Local initialization complete."