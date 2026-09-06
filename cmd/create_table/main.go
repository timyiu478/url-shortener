package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func main() {
	ctx := context.Background()
	table := getenv("DYNAMODB_TABLE_NAME", "Urls")
	endpoint := getenv("AWS_ENDPOINT", "http://localhost:8000")
	region := getenv("AWS_REGION", "us-east-1")

	fmt.Printf("Creating DynamoDB table %s at %s (region=%s)\n", table, endpoint, region)

	// Configure SDK to use local endpoint when provided
	resolver := aws.EndpointResolverWithOptionsFunc(func(service, regionStr string, opts ...interface{}) (aws.Endpoint, error) {
		if endpoint != "" && service == dynamodb.ServiceID {
			return aws.Endpoint{URL: endpoint, SigningRegion: region}, nil
		}
		return aws.Endpoint{}, &aws.EndpointNotFoundError{}
	})

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region), config.WithEndpointResolverWithOptions(resolver))
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load aws config: %v\n", err)
		os.Exit(1)
	}

	client := dynamodb.NewFromConfig(cfg)

	// Check if table exists
	_, err = client.DescribeTable(ctx, &dynamodb.DescribeTableInput{TableName: &table})
	if err == nil {
		fmt.Printf("Table %s already exists\n", table)
		return
	}

	// Create table
	_, err = client.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName: &table,
		AttributeDefinitions: []types.AttributeDefinition{
			{AttributeName: aws.String("short_key"), AttributeType: types.ScalarAttributeTypeS},
		},
		KeySchema: []types.KeySchemaElement{
			{AttributeName: aws.String("short_key"), KeyType: types.KeyTypeHash},
		},
		BillingMode: types.BillingModePayPerRequest,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create table: %v\n", err)
		os.Exit(1)
	}

	// Wait until table is active
	for i := 0; i < 10; i++ {
		out, err := client.DescribeTable(ctx, &dynamodb.DescribeTableInput{TableName: &table})
		if err == nil && out.Table != nil && out.Table.TableStatus == types.TableStatusActive {
			fmt.Printf("Table %s created and active\n", table)
			return
		}
		time.Sleep(1 * time.Second)
	}

	fmt.Printf("Created table %s (pending activation)\n", table)
}

func getenv(k, def string) string {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	return v
}
