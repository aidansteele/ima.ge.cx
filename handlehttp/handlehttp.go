package handlehttp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-lambda-go/lambdacontext"
	slogctx "github.com/veqryn/slog-context"
)

func WrapHandler(h http.Handler) lambda.Handler {
	return &handler{handler: h}
}

type handler struct {
	handler http.Handler
}

type inputPayload struct {
	Version               string            `json:"version"`
	RouteKey              string            `json:"routeKey"`
	RawPath               string            `json:"rawPath"`
	RawQueryString        string            `json:"rawQueryString"`
	Cookies               []string          `json:"cookies"`
	Headers               map[string]string `json:"headers"`
	QueryStringParameters map[string]string `json:"queryStringParameters"`
	RequestContext        RequestContext    `json:"requestContext"`
	Body                  string            `json:"body"`
	IsBase64Encoded       bool              `json:"isBase64Encoded"`
}

func (h *handler) Invoke(ctx context.Context, payload []byte) ([]byte, error) {
	if lc, ok := lambdacontext.FromContext(ctx); ok {
		ctx = slogctx.Prepend(ctx, "requestId", lc.AwsRequestID)
	}
	slog.InfoContext(ctx, "received Lambda invocation", "payload", string(payload))

	input := inputPayload{}
	err := json.Unmarshal(payload, &input)
	if err != nil {
		return nil, fmt.Errorf("parsing payload: %w", err)
	}

	if input.Version != "2.0" {
		return nil, fmt.Errorf("request payload format version not 2.0: %s", input.Version)
	}

	headers := http.Header{}
	for key, val := range input.Headers {
		headers.Set(key, val)
	}

	var body io.Reader = strings.NewReader(input.Body)
	if input.IsBase64Encoded {
		body = base64.NewDecoder(base64.StdEncoding, body)
	}

	u := fmt.Sprintf("https://%s%s?%s", headers.Get("Host"), input.RawPath, input.RawQueryString)

	r := httptest.NewRequest(input.RequestContext.HTTP.Method, u, body)
	r = r.WithContext(context.WithValue(ctx, requestContextKey, &input.RequestContext))
	w := httptest.NewRecorder()
	h.handler.ServeHTTP(w, r)

	res := w.Result()
	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	resHeaders := map[string]string{}
	for key, vals := range res.Header {
		resHeaders[key] = strings.Join(vals, ",")
	}

	b64 := base64.StdEncoding.EncodeToString(resBody)
	output := events.APIGatewayV2HTTPResponse{
		StatusCode:      res.StatusCode,
		Headers:         resHeaders,
		Body:            b64,
		IsBase64Encoded: true,
	}

	return json.Marshal(output)
}
