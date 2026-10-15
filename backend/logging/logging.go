package logging

import (
	"context"
	"log/slog"
	"os"

	"github.com/aws/aws-lambda-go/lambdacontext"
	slogctx "github.com/veqryn/slog-context"
)

// Init initializes the global slog logger with slog-context support.
// Call this at the start of main() in each Lambda.
func Init() {
	handler := slogctx.NewHandler(
		slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}),
		&slogctx.HandlerOptions{
			Prependers: []slogctx.AttrExtractor{
				slogctx.ExtractPrepended,
			},
			Appenders: []slogctx.AttrExtractor{
				slogctx.ExtractAppended,
			},
		},
	)
	slog.SetDefault(slog.New(handler))
}

// Middleware wraps a Lambda handler to inject the Lambda request ID into the logging context.
func Middleware[T any, R any](handler func(context.Context, T) (R, error)) func(context.Context, T) (R, error) {
	return func(ctx context.Context, input T) (R, error) {
		if lc, ok := lambdacontext.FromContext(ctx); ok {
			ctx = slogctx.Prepend(ctx, "requestId", lc.AwsRequestID)
		}
		return handler(ctx, input)
	}
}

// WithRequestPayload adds the Lambda request payload to the context for logging.
func WithRequestPayload(ctx context.Context, payload any) context.Context {
	return slogctx.Prepend(ctx, "request", payload)
}

// WithAttrs adds attributes to the context that will be prepended to all log messages.
func WithAttrs(ctx context.Context, args ...any) context.Context {
	return slogctx.Prepend(ctx, args...)
}
