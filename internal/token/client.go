package token

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// Client for the token service.
type Client struct {
	baseURL     string
	httpClient  *http.Client
	accessToken string
}

// New creates a token service client.
func New(baseURL, accessToken string) *Client {
	return &Client{
		baseURL:     baseURL,
		httpClient:  &http.Client{Timeout: 10 * time.Second, Transport: otelhttp.NewTransport(http.DefaultTransport)},
		accessToken: accessToken,
	}
}

// Check checks the token by calling the token service.
func (c Client) Check(ctx context.Context, token string) (*Token, error) {
	if token == "" {
		return nil, errors.New("empty token")
	}

	endpoint, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse base URL: %w", err)
	}

	query := endpoint.Query()
	query.Set("value", token)
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.accessToken != "" {
		req.Header.Add("Authorization", "Bearer "+c.accessToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call API: %w", err)
	}

	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode/100 != 2 {
		return nil, errors.New(string(body))
	}

	result := &Token{}
	err = json.Unmarshal(body, result)
	if err != nil {
		return nil, fmt.Errorf("failed to Unmarshal response: %w: %s", err, string(body))
	}

	return result, nil
}

// Get get the token by calling the token service.
func (c Client) Get(ctx context.Context, token string) (*Token, error) {
	if token == "" {
		return nil, errors.New("empty token")
	}

	baseURL, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse base URL: %w", err)
	}

	endpoint, err := baseURL.Parse(path.Join(baseURL.Path, token))
	if err != nil {
		return nil, fmt.Errorf("failed to parse endpoint URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.accessToken != "" {
		req.Header.Add("Authorization", "Bearer "+c.accessToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call API: %w", err)
	}

	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode/100 != 2 {
		return nil, errors.New(string(body))
	}

	result := &Token{}
	err = json.Unmarshal(body, result)
	if err != nil {
		return nil, fmt.Errorf("failed to Unmarshal response: %w: %s", err, string(body))
	}

	return result, nil
}
