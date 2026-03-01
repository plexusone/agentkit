// Package http provides HTTP client utilities for inter-agent communication.
package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// PostJSON makes a POST request with JSON payload and decodes the JSON response.
func PostJSON(ctx context.Context, client *http.Client, url string, request interface{}, response interface{}) error {
	reqData, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(httpReq) //nolint:gosec // G704: URL provided by SDK user
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s - %s", resp.StatusCode, resp.Status, string(body))
	}

	if err := json.NewDecoder(resp.Body).Decode(response); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	return nil
}

// GetJSON makes a GET request and decodes the JSON response.
func GetJSON(ctx context.Context, client *http.Client, url string, response interface{}) error {
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Accept", "application/json")

	resp, err := client.Do(httpReq) //nolint:gosec // G704: URL provided by SDK user
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s - %s", resp.StatusCode, resp.Status, string(body))
	}

	if err := json.NewDecoder(resp.Body).Decode(response); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	return nil
}

// HealthCheck checks if a service is healthy by calling its /health endpoint.
func HealthCheck(ctx context.Context, client *http.Client, baseURL string) error {
	url := baseURL + "/health"
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(httpReq) //nolint:gosec // G704: URL provided by SDK user
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("service unhealthy: HTTP %d", resp.StatusCode)
	}

	return nil
}
