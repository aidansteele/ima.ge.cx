package bitypes

import (
	"fmt"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"strings"
	"time"
)

type LayerProgressKey struct {
	Repo        string
	ImageDigest string
	LayerDigest string
}

func (l *LayerProgressKey) Key() map[string]types.AttributeValue {
	m, _ := attributevalue.MarshalMap(map[string]any{
		"pk": fmt.Sprintf("image#%s", l.Repo),
		"sk": fmt.Sprintf("digest#%s#layer#%s", l.ImageDigest, l.LayerDigest),
	})

	return m
}

type LayerProgress struct {
	LayerProgressKey
	TotalBytes     int64
	CompletedBytes int64
	TotalFiles     int64
	CompletedFiles int64
}

func (l *LayerProgress) Marshal() map[string]types.AttributeValue {
	m, _ := attributevalue.MarshalMap(map[string]any{
		"TotalBytes":     l.TotalBytes,
		"CompletedBytes": l.CompletedBytes,
		"TotalFiles":     l.TotalFiles,
		"CompletedFiles": l.CompletedFiles,
		"ttl":            time.Now().Add(90 * 24 * time.Hour).Unix(),
		"v":              1,
	})

	for k, v := range l.Key() {
		m[k] = v
	}

	return m
}

func (l *LayerProgress) UnmarshalDynamoDBAttributeValue(value types.AttributeValue) error {
	m := map[string]any{}
	err := attributevalue.Unmarshal(value, &m)
	if err != nil {
		return fmt.Errorf("unmarshalling: %w", err)
	}

	parts := strings.SplitN(m["pk"].(string), "#", 2)
	if parts[0] != "image" {
		return fmt.Errorf("incorrect format for pk")
	}
	l.Repo = parts[1]

	parts = strings.SplitN(m["sk"].(string), "#", 4)
	if parts[0] != "digest" || parts[2] != "layer" {
		return fmt.Errorf("incorrect format for sk")
	}
	l.ImageDigest = parts[1]
	l.LayerDigest = parts[3]

	l.TotalBytes = int64(m["TotalBytes"].(float64))
	l.CompletedBytes = int64(m["CompletedBytes"].(float64))
	l.TotalFiles = int64(m["TotalFiles"].(float64))
	l.CompletedFiles = int64(m["CompletedFiles"].(float64))

	return nil
}
