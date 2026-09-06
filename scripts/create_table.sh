#!/usr/bin/env bash
set -euo pipefail

DYNAMO_TABLE=${DYNAMODB_TABLE_NAME:-Urls}
ENDPOINT=${AWS_ENDPOINT:-http://localhost:8000}

echo "Creating DynamoDB table '$DYNAMO_TABLE' at $ENDPOINT"

# Use aws cli if present
if command -v aws >/dev/null 2>&1; then
  aws dynamodb create-table \
    --table-name "$DYNAMO_TABLE" \
    --attribute-definitions AttributeName=short_key,AttributeType=S \
    --key-schema AttributeName=short_key,KeyType=HASH \
    --billing-mode PAY_PER_REQUEST \
    --endpoint-url "$ENDPOINT" || true
  echo "Created (or already exists) table $DYNAMO_TABLE"
  exit 0
fi

# Fallback: print instructions
cat <<EOF
aws cli not found. Install AWS CLI v2 or run the following command manually:

aws dynamodb create-table \
  --table-name $DYNAMO_TABLE \
  --attribute-definitions AttributeName=short_key,AttributeType=S \
  --key-schema AttributeName=short_key,KeyType=HASH \
  --billing-mode PAY_PER_REQUEST \
  --endpoint-url $ENDPOINT
EOF
exit 1
