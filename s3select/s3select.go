package s3select

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// Quote returns s as an S3 Select SQL string literal. S3 Select escapes a
// single quote by doubling it and does not support backslash escapes.
func Quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// Select runs query against the gzipped JSON Lines object at bucket/key and
// decodes each returned record into a T.
func Select[T any](ctx context.Context, api *s3.Client, bucket, key, query string) ([]T, error) {
	sel, err := api.SelectObjectContent(ctx, &s3.SelectObjectContentInput{
		Bucket:         &bucket,
		Key:            &key,
		Expression:     &query,
		ExpressionType: types.ExpressionTypeSql,
		InputSerialization: &types.InputSerialization{
			CompressionType: types.CompressionTypeGzip,
			JSON:            &types.JSONInput{Type: types.JSONTypeLines},
		},
		OutputSerialization: &types.OutputSerialization{
			JSON: &types.JSONOutput{RecordDelimiter: aws.String("\n")},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("selecting object content: %w", err)
	}

	stream := sel.GetStream()
	defer stream.Close()

	// Records events can split a JSON line across payloads, so collect them
	// all before decoding.
	buf := &bytes.Buffer{}
	for ev := range stream.Events() {
		if records, ok := ev.(*types.SelectObjectContentEventStreamMemberRecords); ok {
			buf.Write(records.Value.Payload)
		}
	}
	if err := stream.Err(); err != nil {
		return nil, fmt.Errorf("reading select event stream: %w", err)
	}

	entries := []T{}
	dec := json.NewDecoder(buf)
	for {
		var t T
		err := dec.Decode(&t)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("unmarshalling JSON record: %w", err)
		}
		entries = append(entries, t)
	}

	return entries, nil
}
