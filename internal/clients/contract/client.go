package contract

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	contractgen "github.com/ibobrov/share_trip/internal/clients/contract/gen"
	"github.com/ibobrov/share_trip/internal/service"
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
}

func NewClient(server string, httpClient *http.Client, cfg ClientConfig) (*Client, error) {
	generated, err := contractgen.NewClientWithResponses(server, contractgen.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("create contract client: %w", err)
	}
	return &Client{generated: generated, config: cfg}, nil
}

func (c *Client) CheckOption(ctx context.Context, companyID string, optionCode string) (service.CheckResult, error) {
	code := contractgen.OptionCode(optionCode)
	if !code.Valid() {
		return service.CheckResult{Allowed: false}, fmt.Errorf("invalid option code: %s", optionCode)
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
			return service.CheckResult{}, fmt.Errorf("contract.CheckOption: HTTP %d: %s", resp.StatusCode(), strings.Join(resp.JSON400.Errors, "; "))
		case resp.JSON500 != nil:
			return service.CheckResult{}, fmt.Errorf("contract.CheckOption: HTTP %d: %s", resp.StatusCode(), strings.Join(resp.JSON500.Errors, "; "))
		default:
			return service.CheckResult{}, fmt.Errorf("contract.CheckOption: unexpected response, HTTP %d", resp.StatusCode())
		}
	}
	result := service.CheckResult{Allowed: resp.JSON200.Data.Allowed}
	if resp.JSON200.Data.Reason != nil {
		result.Reason = string(*resp.JSON200.Data.Reason)
	}
	return result, nil
}

func (c *Client) checkOption(ctx context.Context, request contractgen.CheckOptionJSONRequestBody) (*contractgen.CheckOptionClientResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, c.config.CheckTimeout)
	defer cancel()

	for attempt := 0; attempt < c.config.RetryAttempts; attempt++ {
		attemptCtx, cancelAttempt := context.WithTimeout(ctx, c.config.AttemptTimeout)
		response, err := c.generated.CheckOption(attemptCtx, request)
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
