package healthcheck

import (
	"context"
	"net/http"

	"github.com/rs/zerolog/log"
)

// Pinger is capable of pinging.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Client is the healthcheck client.
type Client struct {
	DB Pinger
}

// Live is the liveness handler.
func (c *Client) Live(rw http.ResponseWriter, _ *http.Request) {
	rw.WriteHeader(http.StatusOK)
}

// Ready is the readiness handler.
func (c *Client) Ready(rw http.ResponseWriter, req *http.Request) {
	err := c.DB.Ping(req.Context())
	if err != nil {
		log.Error().Err(err).Msg("failed to ping database")
		http.Error(rw, err.Error(), http.StatusServiceUnavailable)

		return
	}
}
