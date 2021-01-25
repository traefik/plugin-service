package healthcheck

import (
	"fmt"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
)

// Client is the healthcheck client.
type Client struct {
	httpClient *http.Client
	faunaPing  string
}

// New creates a new healthcheck client.
func New() *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 5 * time.Second},
		faunaPing:  "https://db.fauna.com/ping",
	}
}

// Live is the liveness handler.
func (c *Client) Live(rw http.ResponseWriter, req *http.Request) {
	rw.WriteHeader(http.StatusOK)
}

// Ready is the readiness handler.
func (c *Client) Ready(rw http.ResponseWriter, r *http.Request) {
	resp, err := c.httpClient.Get(c.faunaPing)
	if err != nil {
		log.Error().Err(err).Msg("failed to contact /ping on faunadb")
		http.Error(rw, err.Error(), http.StatusServiceUnavailable)
		return
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("faunaDB didn't send a valid HTTP response code: %d", resp.StatusCode)
		log.Error().Err(err).Msg("failed to contact /ping on faunadb")
		http.Error(rw, err.Error(), http.StatusServiceUnavailable)
		return
	}
}
