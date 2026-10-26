package middleware

import (
	"log/slog"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/ibobrov/share_trip/internal/observability/logctx"
)

const RequestIDHeader = "X-Request-ID"
const CorrelationIDHeader = "X-Correlation-ID"
const LoggerLocalKey = "logger"

func Correlation(baseLogger *slog.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Copy header values: Fiber reuses its request buffers after the handler returns.
		requestID := strings.Clone(c.Get(RequestIDHeader))
		if requestID == "" {
			requestID = uuid.NewString()
		}
		correlationID := strings.Clone(c.Get(CorrelationIDHeader))
		if correlationID == "" {
			correlationID = requestID
		}
		c.Set(RequestIDHeader, requestID)
		c.Set(CorrelationIDHeader, correlationID)
		logger := baseLogger.With("request_id", requestID, "correlation_id", correlationID, "method", strings.Clone(c.Method()), "path", strings.Clone(c.Path()))
		ctx := logctx.WithRequestID(c.UserContext(), requestID)
		ctx = logctx.WithCorrelationID(ctx, correlationID)
		ctx = logctx.WithBusinessIDs(ctx, strings.Clone(c.Get("X-Trip-ID")), strings.Clone(c.Get("X-User-ID")))
		ctx = logctx.WithLogger(ctx, logger)
		c.SetUserContext(ctx)
		c.Locals(LoggerLocalKey, logger)
		return c.Next()
	}
}
