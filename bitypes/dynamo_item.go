package bitypes

import (
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"strings"
	"time"
)

type ImageInfoKey struct {
	Repo   string
	Digest string
}

func (d *ImageInfoKey) Key() map[string]types.AttributeValue {
	m, _ := attributevalue.MarshalMap(map[string]any{
		"pk": fmt.Sprintf("image#%s", d.Repo),
		"sk": fmt.Sprintf("digest#%s", d.Digest),
	})

	return m
}

type ImageInfoStatus string

const (
	ImageInfoStatusPending   ImageInfoStatus = "PENDING"
	ImageInfoStatusRunning   ImageInfoStatus = "RUNNING"
	ImageInfoStatusSucceeded ImageInfoStatus = "SUCCEEDED"
	ImageInfoStatusFailed    ImageInfoStatus = "FAILED"
)

type ImageInfoItem struct {
	ImageInfoKey
	Tags        []string
	TotalSize   int64
	Duration    time.Duration
	Retrieved   time.Time
	ExecutionId string
	Status      ImageInfoStatus
	Manifest    json.RawMessage
	RawConfig   json.RawMessage
}

func (d *ImageInfoItem) DynamoItem() map[string]types.AttributeValue {
	m, _ := attributevalue.MarshalMap(map[string]any{
		"Tags":        d.Tags,
		"TotalSize":   d.TotalSize,
		"Duration":    d.Duration,
		"Retrieved":   d.Retrieved.Format(time.RFC3339Nano),
		"ExecutionId": d.ExecutionId,
		"Status":      d.Status,
		"Manifest":    d.Manifest,
		"RawConfig":   d.RawConfig,
		"ttl":         time.Now().Add(90 * 24 * time.Hour).Unix(),
		"v":           1,
	})

	for k, v := range d.Key() {
		m[k] = v
	}

	return m
}

func (d *ImageInfoItem) UnmarshalDynamoDBAttributeValue(value types.AttributeValue) error {
	mss := map[string]any{}
	err := attributevalue.Unmarshal(value, &mss)
	if err != nil {
		return fmt.Errorf("unmarshalling to string map: %w", err)
	}

	parts := strings.SplitN(mss["pk"].(string), "#", 2)
	if parts[0] != "image" {
		return fmt.Errorf("incorrect format for pk")
	}
	d.Repo = parts[1]

	parts = strings.SplitN(mss["sk"].(string), "#", 2)
	if parts[0] != "digest" {
		return fmt.Errorf("incorrect format for sk")
	}
	d.Digest = parts[1]

	tags := mss["Tags"].([]interface{})
	for _, tag := range tags {
		d.Tags = append(d.Tags, tag.(string))
	}

	d.RawConfig = mss["RawConfig"].([]uint8)
	d.Manifest = mss["Manifest"].([]uint8)
	d.ExecutionId = mss["ExecutionId"].(string)
	d.Status = ImageInfoStatus(mss["Status"].(string))
	d.TotalSize = int64(mss["TotalSize"].(float64))
	d.Duration = time.Duration(mss["Duration"].(float64))

	retrievedStr := mss["Retrieved"].(string)
	d.Retrieved, err = time.Parse(time.RFC3339Nano, retrievedStr)
	if err != nil {
		return fmt.Errorf("parsing retrieval timestamp: %w", err)
	}

	return nil
}
