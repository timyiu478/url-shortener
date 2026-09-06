package repositories

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	icfg "url-shortener/internal/config"
)

var (
	ErrURLNotFound  = fmt.Errorf("URL not found")
	ErrDuplicateKey = fmt.Errorf("duplicate short key")
)

// URLRepository defines the interface for URL storage operations.
type URLRepository interface {
	StoreURL(ctx context.Context, shortKey, originalURL string) error
	GetURL(ctx context.Context, shortKey string) (string, error)
	PingDB(ctx context.Context) error
	Close() error
}

// urlRepository implements URLRepository using DynamoDB.
type urlRepository struct {
	client    *dynamodb.Client
	tableName string
}

// NewURLRepository creates a new URLRepository with DynamoDB.
func NewURLRepository(cfg *icfg.Config) (URLRepository, error) {
	// Use explicit context so region/endpoint resolution doesn't hang on IMDS
	ctx := context.Background()

	// Endpoint resolver: when AWS_ENDPOINT is set, point the SDK to it (useful for local DynamoDB)
	resolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, opts ...interface{}) (aws.Endpoint, error) {
		if cfg.AWS_ENDPOINT != "" {
			return aws.Endpoint{URL: cfg.AWS_ENDPOINT, SigningRegion: cfg.AWSRegion}, nil
		}
		return aws.Endpoint{}, &aws.EndpointNotFoundError{}
	})

	// Provide static credentials when running locally to avoid SDK attempting IMDS
	accessKey := os.Getenv("AWS_ACCESS_KEY_ID")
	secretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	credsProvider := credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")

	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(cfg.AWSRegion),
		config.WithCredentialsProvider(credsProvider),
		config.WithEndpointResolverWithOptions(resolver),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := dynamodb.NewFromConfig(awsCfg)
	return &urlRepository{
		client:    client,
		tableName: cfg.DynamoDBTableName,
	}, nil
}

// Close is a no-op for DynamoDB, as it’s serverless and manages connections internally.
func (r *urlRepository) Close() error {
	return nil
}

// StoreURL stores a URL mapping in DynamoDB with conditional write to ensure uniqueness.
func (r *urlRepository) StoreURL(ctx context.Context, shortKey string, originalURL string) error {
	// Allow reasonable timeout for first-time connections when running in containers
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	item := map[string]types.AttributeValue{
		"short_key":    &types.AttributeValueMemberS{Value: shortKey},
		"original_url": &types.AttributeValueMemberS{Value: originalURL},
	}

	input := &dynamodb.PutItemInput{
		TableName:           aws.String(r.tableName),
		Item:                item,
		ConditionExpression: aws.String("attribute_not_exists(short_key)"),
	}

	_, err := r.client.PutItem(ctx, input)
	if err != nil {
		var cfe *types.ConditionalCheckFailedException
		if ok := (func() bool { _, ok := err.(*types.ConditionalCheckFailedException); return ok })(); ok {
			return ErrDuplicateKey
		}
		// propagate error
		return fmt.Errorf("failed to store URL: %w", err)
	}
	return nil
}

// GetURL retrieves a URL from DynamoDB.
func (r *urlRepository) GetURL(ctx context.Context, shortKey string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	input := &dynamodb.GetItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"short_key": &types.AttributeValueMemberS{Value: shortKey},
		},
		ConsistentRead: aws.Bool(false), // eventual consistency for better performance
	}

	result, err := r.client.GetItem(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to get URL: %w", err)
	}
	if result.Item == nil {
		return "", ErrURLNotFound
	}

	var item struct {
		OriginalURL string `dynamodbav:"original_url"`
	}
	err = attributevalue.UnmarshalMap(result.Item, &item)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal item: %w", err)
	}
	return item.OriginalURL, nil
}

// PingDB checks DynamoDB connectivity by describing the table.
func (r *urlRepository) PingDB(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := r.client.DescribeTable(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(r.tableName),
	})
	if err != nil {
		return fmt.Errorf("failed to ping DynamoDB: %w", err)
	}
	return nil
}
