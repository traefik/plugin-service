package serve

import (
	"github.com/traefik/plugin-service/pkg/db/mongodb"
)

// Config holds the serve configuration.
type Config struct {
	MongoDB mongodb.Config
	Pilot   Pilot
	Tracing Tracing
	GoProxy GoProxy
}

// Pilot holds pilots configuration.
type Pilot struct {
	Host                string
	JWTCert             string
	TokenURL            string
	ServicesAccessToken string
	GitHubToken         string
}

// Tracing holds tracing configuration.
type Tracing struct {
	Endpoint    string
	Username    string
	Password    string
	Probability float64
}

// GoProxy holds the go-proxy configuration.
type GoProxy struct {
	URL      string
	Username string
	Password string
}
