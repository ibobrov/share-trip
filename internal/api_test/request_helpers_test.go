package api_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func sendJSONRequest[T any](
	t *testing.T,
	method string,
	url string,
	payload any,
	expectedStatus int,
) T {
	t.Helper()

	var requestBody io.Reader

	if payload != nil {
		body, err := json.Marshal(payload)
		require.NoError(t, err)

		requestBody = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, url, requestBody)
	require.NoError(t, err)

	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	resp, err := testApp.Test(req, -1)
	require.NoError(t, err)

	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Errorf("close response body: %v", err)
		}
	}()

	respBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	require.Equal(
		t,
		expectedStatus,
		resp.StatusCode,
		"response: %s",
		string(respBody),
	)

	var response T
	require.NoError(
		t,
		json.Unmarshal(respBody, &response),
		"response: %s",
		string(respBody),
	)

	return response
}
