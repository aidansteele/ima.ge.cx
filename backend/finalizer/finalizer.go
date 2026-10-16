package main

import (
	"browseimage/bitypes"
	"browseimage/httpclient"
	"browseimage/logging"
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type finalizer struct {
	dynamo *dynamodb.Client
	table  string
}

type FinalizerInput struct {
	Payload struct {
		Repo   string
		Digest string
	}
	Meta struct {
		ExecutionId string
		StartTime   time.Time
		Error       any `json:",omitempty"`
	}
}

type FinalizerOutput struct {
}

func (f *finalizer) handle(ctx context.Context, input *FinalizerInput) (*FinalizerOutput, error) {
	ctx = logging.WithRequestPayload(ctx, input)
	slog.InfoContext(ctx, "handling finalizer request")

	key := &bitypes.ImageInfoKey{
		Repo:   input.Payload.Repo,
		Digest: input.Payload.Digest,
	}

	status := bitypes.ImageInfoStatusSucceeded
	if input.Meta.Error != nil {
		status = bitypes.ImageInfoStatusFailed
	}

	duration := time.Since(input.Meta.StartTime)

	_, err := f.dynamo.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:        &f.table,
		Key:              key.Key(),
		UpdateExpression: aws.String("SET #status = :status, #duration = :duration, ExecutionId = :executionId"),
		//ConditionExpression: aws.String("ExecutionId = :executionId"),
		ExpressionAttributeNames: map[string]string{
			"#status":   "Status",
			"#duration": "Duration",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":status":      &types.AttributeValueMemberS{Value: string(status)},
			":duration":    &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", duration)},
			":executionId": &types.AttributeValueMemberS{Value: input.Meta.ExecutionId},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("updating item: %w", err)
	}

	return &FinalizerOutput{}, nil
}

func main() {
	logging.Init()

	ctx := context.Background()

	cfg, err := config.LoadDefaultConfig(ctx, config.WithHTTPClient(httpclient.DefaultClient))
	if err != nil {
		panic(err)
	}

	f := &finalizer{
		dynamo: dynamodb.NewFromConfig(cfg),
		table:  os.Getenv("TABLE"),
	}

	lambda.Start(logging.Middleware(f.handle))
}
