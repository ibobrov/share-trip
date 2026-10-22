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

const (
	RequestTimeout = 500 * time.Millisecond
	checkTimeout   = 1200 * time.Millisecond
	retryDelay     = 100 * time.Millisecond
)

type Client struct {
	generated *contractgen.ClientWithResponses
}

func NewClient(server string, httpClient *http.Client) (*Client, error) {
	generated, err := contractgen.NewClientWithResponses(server, contractgen.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("create contract client: %w", err)
	}
	return &Client{generated: generated}, nil
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
	ctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()

	for attempt := 0; attempt < 2; attempt++ {
		attemptCtx, cancelAttempt := context.WithTimeout(ctx, RequestTimeout)
		response, err := c.generated.CheckOption(attemptCtx, request)
		if err != nil {
			cancelAttempt()
			return nil, err
		}

		if response.StatusCode != http.StatusInternalServerError || attempt == 1 {
			parsed, err := contractgen.ParseCheckOptionClientResponse(response)
			cancelAttempt()
			return parsed, err
		}

		// Close the first response before retrying, even if its body is not valid JSON.
		_, _ = io.Copy(io.Discard, response.Body)
		_ = response.Body.Close()
		cancelAttempt()

		timer := time.NewTimer(retryDelay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	return nil, fmt.Errorf("contract check attempts exhausted")
}
