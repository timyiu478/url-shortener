package repositories_test

import (
    "context"
    "testing"
    "time"

    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/service/dynamodb"
    "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
    "url-shortener/internal/config"
    "url-shortener/internal/repositories"
)

// mockDynamoDBClient is a test double for DynamoDB.
type mockDynamoDBClient struct {
    putItemFunc       func(ctx context.Context, input *dynamodb.PutItemInput) (*dynamodb.PutItemOutput, error)
    getItemFunc       func(ctx context.Context, input *dynamodb.GetItemInput) (*dynamodb.GetItemOutput, error)
    describeTableFunc func(ctx context.Context, input *dynamodb.DescribeTableInput) (*dynamodb.DescribeTableOutput, error)
}

func (m *mockDynamoDBClient) PutItem(ctx context.Context, input *dynamodb.PutItemInput, opts ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
    return m.putItemFunc(ctx, input)
}

func (m *mockDynamoDBClient) GetItem(ctx context.Context, input *dynamodb.GetItemInput, opts ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
    return m.getItemFunc(ctx, input)
}

func (m *mockDynamoDBClient) DescribeTable(ctx context.Context, input *dynamodb.DescribeTableInput, opts ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error) {
    return m.describeTableFunc(ctx, input)
}

func setupTestConfig() *config.Config {
    return &config.Config{
        AWSRegion:        "us-west-2",
        DynamoDBTableName: "urls",
    }
}

func TestNewURLRepository(t *testing.T) {
    cfg := setupTestConfig()
    repo, err := repositories.NewURLRepository(cfg)
    if err != nil {
        t.Fatalf("NewURLRepository failed: %v", err)
    }
    defer repo.Close()

    if _, ok := repo.(*repositories.urlRepository); !ok {
        t.Error("NewURLRepository returned wrong type")
    }
}

func TestStoreURL(t *testing.T) {
    cfg := setupTestConfig()
    mockClient := &mockDynamoDBClient{
        putItemFunc: func(ctx context.Context, input *dynamodb.PutItemInput) (*dynamodb.PutItemOutput, error) {
            return &dynamodb.PutItemOutput{}, nil
        },
    }
    repo := &repositories.urlRepository{client: mockClient, tableName: cfg.DynamoDBTableName}
    ctx := context.Background()
    shortKey := "abc123"
    originalURL := "https://example.com"

    err := repo.StoreURL(ctx, shortKey, originalURL)
    if err != nil {
        t.Errorf("StoreURL failed: %v", err)
    }
}

func TestStoreURL_DuplicateKey(t *testing.T) {
    cfg := setupTestConfig()
    mockClient := &mockDynamoDBClient{
        putItemFunc: func(ctx context.Context, input *dynamodb.PutItemInput) (*dynamodb.PutItemOutput, error) {
            return nil, &types.ConditionalCheckFailedException{}
        },
    }
    repo := &repositories.urlRepository{client: mockClient, tableName: cfg.DynamoDBTableName}
    ctx := context.Background()
    shortKey := "abc123"
    originalURL := "https://example.com"

    err := repo.StoreURL(ctx, shortKey, originalURL)
    if err == nil {
        t.Error("Expected duplicate key error, got none")
    }
    if !errors.Is(err, repositories.ErrDuplicateKey) {
        t.Errorf("Expected ErrDuplicateKey, got %v", err)
    }
}

func TestGetURL(t *testing.T) {
    cfg := setupTestConfig()
    mockClient := &mockDynamoDBClient{
        getItemFunc: func(ctx context.Context, input *dynamodb.GetItemInput) (*dynamodb.GetItemOutput, error) {
            return &dynamodb.GetItemOutput{
                Item: map[string]types.AttributeValue{
                    "short_key":    &types.AttributeValueMemberS{Value: "abc123"},
                    "original_url": &types.AttributeValueMemberS{Value: "https://example.com"},
                },
            }, nil
        },
    }
    repo := &repositories.urlRepository{client: mockClient, tableName: cfg.DynamoDBTableName}
    ctx := context.Background()
    shortKey := "abc123"
    originalURL := "https://example.com"

    gotURL, err := repo.GetURL(ctx, shortKey)
    if err != nil || gotURL != originalURL {
        t.Errorf("GetURL failed: got %s, want %s, err: %v", gotURL, originalURL, err)
    }
}

func TestGetURL_NotFound(t *testing.T) {
    cfg := setupTestConfig()
    mockClient := &mockDynamoDBClient{
        getItemFunc: func(ctx context.Context, input *dynamodb.GetItemInput) (*dynamodb.GetItemOutput, error) {
            return &dynamodb.GetItemOutput{Item: nil}, nil
        },
    }
    repo := &repositories.urlRepository{client: mockClient, tableName: cfg.DynamoDBTableName}
    ctx := context.Background()

    _, err := repo.GetURL(ctx, "abc123")
    if err == nil || !errors.Is(err, repositories.ErrURLNotFound) {
        t.Errorf("Expected ErrURLNotFound, got %v", err)
    }
}

func TestPingDB(t *testing.T) {
    cfg := setupTestConfig()
    mockClient := &mockDynamoDBClient{
        describeTableFunc: func(ctx context.Context, input *dynamodb.DescribeTableInput) (*dynamodb.DescribeTableOutput, error) {
            return &dynamodb.DescribeTableOutput{}, nil
        },
    }
    repo := &repositories.urlRepository{client: mockClient, tableName: cfg.DynamoDBTableName}
    ctx := context.Background()

    err := repo.PingDB(ctx)
    if err != nil {
        t.Errorf("PingDB failed: %v", err)
    }
}

func TestClose(t *testing.T) {
    cfg := setupTestConfig()
    mockClient := &mockDynamoDBClient{}
    repo := &repositories.urlRepository{client: mockClient, tableName: cfg.DynamoDBTableName}

    err := repo.Close()
    if err != nil {
        t.Errorf("Close failed: %v", err)
    }
}
