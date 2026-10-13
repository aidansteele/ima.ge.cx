package main

import (
	"errors"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseImageQuery(t *testing.T) {
	const digest = "sha256:030eaf505bc7d39bfe1c485762603a95b5d43277f98cd3334d85e84547073abb"

	// repo is used verbatim in DynamoDB and S3 keys, so only the tag may be removed.
	for image, wantRepo := range map[string]string{
		"ubuntu":                           "ubuntu",
		"golang:1.19":                      "golang",
		"mcr.microsoft.com/dotnet/sdk:6.0": "mcr.microsoft.com/dotnet/sdk",
		"localhost:5000/team/app":          "localhost:5000/team/app",
		"localhost:5000/team/app:v1":       "localhost:5000/team/app",
	} {
		repo, ref, err := parseImageQuery(url.Values{"image": {image}, "digest": {digest}})
		require.NoError(t, err, image)
		assert.Equal(t, wantRepo, repo, image)
		assert.Equal(t, digest, ref.DigestStr(), image)
	}

	for _, q := range []url.Values{
		{"image": {"ubuntu"}},
		{"image": {"ubuntu"}, "digest": {"sha256:030e"}}, // a prefix would match other digests in DynamoDB
		{"digest": {digest}},
	} {
		_, _, err := parseImageQuery(q)
		var se *statusError
		require.True(t, errors.As(err, &se), q.Encode())
		assert.Equal(t, http.StatusBadRequest, se.status, q.Encode())
	}
}
