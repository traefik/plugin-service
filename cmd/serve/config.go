package serve

import (
    "time"

    "github.com/traefik/plugin-service/pkg/db/mongodb"
    "github.com/traefik/plugin-service/pkg/tracer"
)

// Config holds the serve configuration.
type Config struct {
    Addr        string
    GitHubToken string

    TraceURL string

    MongoDB mongodb.Config
    Tracing tracer.Config
    GoProxy GoProxy

    TTL time.Duration
}

// GoProxy holds the go-proxy configuration.
type GoProxy struct {
    URL      string
    Username string
    Password string
}
