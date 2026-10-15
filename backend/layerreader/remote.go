package layerreader

import (
	"browseimage/bitypes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/google/go-containerregistry/pkg/v1"
)

type Remote struct {
	Lambda      *lambda.Client
	FunctionArn string
}

type RemoteInput struct {
	Key   *bitypes.ImageInfoKey
	Layer v1.Hash
}

type Put struct {
	Key       string
	VersionId string
	Size      int64
}

type RemoteOutput struct {
	Gzi Put
	Tar Put
}

func (r Remote) ReadLayer(ctx context.Context, key *bitypes.ImageInfoKey, layer v1.Layer) ([]MyTarHeader, error) {
	digest, _ := layer.Digest()
	input, _ := json.Marshal(RemoteInput{Key: key, Layer: digest})

	invoke, err := r.Lambda.Invoke(ctx, &lambda.InvokeInput{
		FunctionName: &r.FunctionArn,
		Payload:      input,
		LogType:      types.LogTypeTail,
	})
	if err != nil {
		return nil, fmt.Errorf("invoking lambda: %w", err)
	}

	if invoke.FunctionError != nil {
		output := ""
		if invoke.LogResult != nil {
			decoded, _ := base64.StdEncoding.DecodeString(*invoke.LogResult)
			if decoded != nil {
				output = string(decoded)
			}
		}

		return nil, fmt.Errorf("invoking lambda: %s: %s (input was %s)", *invoke.FunctionError, output, string(input))
	}

	ro := RemoteOutput{}
	err = json.Unmarshal(invoke.Payload, &ro)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling lambda response: %w", err)
	}

	// The lambda stores files to S3 and returns upload metadata.
	// The actual headers are not returned through this function.
	return nil, nil
}
