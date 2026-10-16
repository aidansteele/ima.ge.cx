package main

import (
	"browseimage/bitypes"
	"browseimage/httpclient"
	"browseimage/logging"
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dynamotypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

func main() {
	logging.Init()

	ctx := context.Background()

	cfg, err := config.LoadDefaultConfig(ctx, config.WithHTTPClient(httpclient.DefaultClient))
	if err != nil {
		panic(fmt.Sprintf("%+v", err))
	}

	l := &layerLister{
		dynamodb: dynamodb.NewFromConfig(cfg),
		table:    os.Getenv("TABLE"),
	}

	lambda.Start(logging.Middleware(l.handle))
}

type layerListerInput struct {
	Repo   string `json:"Repo"`
	Digest string `json:"Digest"`
}

type layerListerOutput struct {
	Layers     []string
	LayerSizes map[string]int64
}

type layerLister struct {
	dynamodb *dynamodb.Client
	table    string
}

func (ll *layerLister) handle(ctx context.Context, input *layerListerInput) (any, error) {
	ctx = logging.WithRequestPayload(ctx, input)
	slog.InfoContext(ctx, "handling layer lister request")

	refstr := fmt.Sprintf("%s@%s", input.Repo, input.Digest)
	ref, err := name.ParseReference(refstr)
	if err != nil {
		return nil, fmt.Errorf("parsing ref: %w", err)
	}

	img, err := remote.Image(ref, remote.WithAuthFromKeychain(authn.DefaultKeychain), remote.WithContext(ctx), remote.WithTransport(httpclient.DefaultClient.Transport))
	if err != nil {
		return nil, fmt.Errorf("getting image: %w", err)
	}

	layers, err := img.Layers()
	if err != nil {
		return nil, fmt.Errorf("getting layers: %w", err)
	}

	digests := []string{}
	sizes := make(map[string]int64, len(layers))
	var totalSize int64

	for _, layer := range layers {
		mediaType, err := layer.MediaType()
		if err != nil {
			return nil, fmt.Errorf("getting layer media type: %w", err)
		}

		// The layer reader indexes layers with gztool, which only understands gzip.
		// Artifacts such as signatures and attestations have JSON "layers".
		switch mediaType {
		case types.DockerLayer, types.DockerForeignLayer, types.OCILayer, types.OCIRestrictedLayer:
		default:
			return nil, fmt.Errorf("unsupported layer media type %q: only gzip-compressed tar layers can be indexed", mediaType)
		}

		size, err := layer.Size()
		if err != nil {
			return nil, fmt.Errorf("getting layer size: %w", err)
		}
		totalSize += size

		digest, err := layer.Digest()
		if err != nil {
			return nil, fmt.Errorf("getting layer digest: %w", err)
		}

		digests = append(digests, digest.String())
		sizes[digest.String()] = size
	}

	key := &bitypes.ImageInfoKey{
		Repo:   input.Repo,
		Digest: input.Digest,
	}

	rawConfig, err := img.RawConfigFile()
	if err != nil {
		return nil, fmt.Errorf("getting raw config: %w", err)
	}

	manifest, err := img.RawManifest()
	if err != nil {
		return nil, fmt.Errorf("getting manifest: %w", err)
	}

	_, err = ll.dynamodb.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:        &ll.table,
		Key:              key.Key(),
		UpdateExpression: aws.String("SET TotalSize = :TotalSize, RawConfig = :RawConfig, Manifest = :Manifest"),
		ExpressionAttributeValues: map[string]dynamotypes.AttributeValue{
			":TotalSize": &dynamotypes.AttributeValueMemberN{Value: fmt.Sprintf("%d", totalSize)},
			":RawConfig": &dynamotypes.AttributeValueMemberB{Value: rawConfig},
			":Manifest":  &dynamotypes.AttributeValueMemberB{Value: manifest},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("updating image data in dynamo: %w", err)
	}

	return &layerListerOutput{
		Layers:     digests,
		LayerSizes: sizes,
	}, nil
}
