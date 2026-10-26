package contract

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	contractgen "github.com/ibobrov/share_trip/internal/clients/contract/gen"
	"github.com/ibobrov/share_trip/internal/observability/logctx"
	"github.com/ibobrov/share_trip/internal/observability/metrics"
	"github.com/ibobrov/share_trip/internal/service"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

const RequestTimeout = 500 * time.Millisecond

type ClientConfig struct {
	AttemptTimeout time.Duration
	CheckTimeout   time.Duration
	RetryAttempts  int
	RetryDelay     time.Duration
}

func DefaultClientConfig() ClientConfig {
	return ClientConfig{AttemptTimeout: RequestTimeout, CheckTimeout: 1200 * time.Millisecond, RetryAttempts: 2, RetryDelay: 100 * time.Millisecond}
}

type Client struct {
	generated *contractgen.ClientWithResponses
	config    ClientConfig
	metrics   *metrics.Metrics
}

func NewClient(server string, httpClient *http.Client, cfg ClientConfig, observer *metrics.Metrics) (*Client, error) {
	generated, err := contractgen.NewClientWithResponses(server, contractgen.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("create contract client: %w", err)
	}
	return &Client{generated: generated, config: cfg, metrics: observer}, nil
}

func (c *Client) CheckOption(ctx context.Context, companyID string, optionCode string) (result service.CheckResult, err error) {
	ctx, span := otel.Tracer("sharetrip").Start(ctx, "Contract.CheckOption", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()
	started := time.Now()
	defer func() {
		outcome := "allowed"
		if err != nil {
			outcome = "error"
			span.RecordError(err)
			span.SetStatus(codes.Error, "operation failed")
		} else if !result.Allowed {
			outcome = "denied"
		}
		if c.metrics != nil {
			c.metrics.ContractRequestTotal.WithLabelValues(outcome).Inc()
			c.metrics.ContractRequestDuration.WithLabelValues(outcome).Observe(time.Since(started).Seconds())
		}
		level := slog.LevelInfo
		if err != nil {
			level = slog.LevelError
		}
		logctx.Logger(ctx).Log(ctx, level, "contract permission check completed", "component", "ContractClient", "operation", "CheckOption", "company_id", companyID, "option_code", optionCode, "result", outcome, "error", err)
	}()
	code := contractgen.OptionCode(optionCode)
	if !code.Valid() {
		return service.CheckResult{Allowed: false}, fmt.Errorf("invalid option code")
	}

	resp, err := c.checkOption(ctx, contractgen.CheckOptionJSONRequestBody{
		CompanyID:  companyID,
		OptionCode: code,
	})

	if err != nil {
		return service.CheckResult{}, fmt.Errorf("contract.CheckOption: %w", err)
	}
	if resp.JSON200 == nil {
		switch {
		case resp.JSON400 != nil:
			return service.CheckResult{}, fmt.Errorf("contract.CheckOption: HTTP %d", resp.StatusCode())
		case resp.JSON500 != nil:
			return service.CheckResult{}, fmt.Errorf("contract.CheckOption: HTTP %d", resp.StatusCode())
		default:
			return service.CheckResult{}, fmt.Errorf("contract.CheckOption: unexpected response, HTTP %d", resp.StatusCode())
		}
	}
	result = service.CheckResult{Allowed: resp.JSON200.Data.Allowed}
	if resp.JSON200.Data.Reason != nil {
		if !resp.JSON200.Data.Reason.Valid() {
			return service.CheckResult{}, fmt.Errorf("contract.CheckOption: invalid denial reason")
		}
		result.Reason = string(*resp.JSON200.Data.Reason)
	}
	return result, nil
}

func (c *Client) checkOption(ctx context.Context, request contractgen.CheckOptionJSONRequestBody) (*contractgen.CheckOptionClientResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, c.config.CheckTimeout)
	defer cancel()

	for attempt := 0; attempt < c.config.RetryAttempts; attempt++ {
		attemptCtx, cancelAttempt := context.WithTimeout(ctx, c.config.AttemptTimeout)
		response, err := c.generated.CheckOption(attemptCtx, request, func(ctx context.Context, req *http.Request) error {
			for key, value := range map[string]string{
				"X-Request-ID": logctx.RequestID(ctx), "X-Correlation-ID": logctx.CorrelationID(ctx),
				"X-Trip-ID": logctx.TripID(ctx), "X-User-ID": logctx.ClientID(ctx),
			} {
				if value != "" {
					req.Header.Set(key, value)
				}
			}
			propagation.TraceContext{}.Inject(ctx, propagation.HeaderCarrier(req.Header))
			return nil
		})
		if err != nil {
			cancelAttempt()
			return nil, err
		}

		if response.StatusCode != http.StatusInternalServerError || attempt+1 == c.config.RetryAttempts {
			parsed, err := contractgen.ParseCheckOptionClientResponse(response)
			cancelAttempt()
			return parsed, err
		}

		// Close the first response before retrying, even if its body is not valid JSON.
		_, _ = io.Copy(io.Discard, response.Body)
		_ = response.Body.Close()
		cancelAttempt()

		timer := time.NewTimer(c.config.RetryDelay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	return nil, fmt.Errorf("contract check attempts exhausted")
}
