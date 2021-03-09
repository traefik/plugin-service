package token

import (
	"context"
)

// Client for the token service.
type Client struct{}

// New creates a token service client.
func New(_, _ string) *Client {
	return &Client{}
}

// Check checks the token by calling the token service.
func (c Client) Check(_ context.Context, _ string) (*Token, error) {
	return &Token{}, nil
}

// Get get the token by calling the token service.
func (c Client) Get(_ context.Context, _ string) (*Token, error) {
	return &Token{}, nil
}
