package logctx

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

type loggerKey struct{}
type requestIDKey struct{}
type correlationIDKey struct{}
type tripIDKey struct{}
type clientIDKey struct{}

func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey{}, logger)
}
func Logger(ctx context.Context) *slog.Logger {
	logger, ok := ctx.Value(loggerKey{}).(*slog.Logger)
	if !ok || logger == nil {
		logger = slog.Default()
	}
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		logger = logger.With("trace_id", sc.TraceID().String())
	}
	if id := TripID(ctx); id != "" {
		logger = logger.With("trip_id", id)
	}
	if id := ClientID(ctx); id != "" {
		logger = logger.With("client_id", id)
	}
	return logger
}
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}
func RequestID(ctx context.Context) string {
	value, _ := ctx.Value(requestIDKey{}).(string)
	return value
}
func WithCorrelationID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, correlationIDKey{}, id)
}
func CorrelationID(ctx context.Context) string {
	value, _ := ctx.Value(correlationIDKey{}).(string)
	return value
}
func WithBusinessIDs(ctx context.Context, tripID, clientID string) context.Context {
	ctx = context.WithValue(ctx, tripIDKey{}, tripID)
	return context.WithValue(ctx, clientIDKey{}, clientID)
}
func TripID(ctx context.Context) string { value, _ := ctx.Value(tripIDKey{}).(string); return value }
func ClientID(ctx context.Context) string {
	value, _ := ctx.Value(clientIDKey{}).(string)
	return value
}
func Traceparent(ctx context.Context) string {
	headers := propagation.MapCarrier{}
	propagation.TraceContext{}.Inject(ctx, headers)
	return headers.Get("traceparent")
}
func Tracestate(ctx context.Context) string {
	headers := propagation.MapCarrier{}
	propagation.TraceContext{}.Inject(ctx, headers)
	return headers.Get("tracestate")
}
