package functions

import (
	"encoding/base64"
)

type faunaDB struct {
	Endpoint string `env:"FAUNADB_ENDPOINT"`
	Secret   string `env:"FAUNADB_SECRET,required"`
}

type pilot struct {
	TokenURL            string       `env:"PILOT_TOKEN_URL,required"`
	JWTCert             string       `env:"PILOT_JWT_CERT,required"`
	ServicesAccessToken base64String `env:"PILOT_SERVICES_ACCESS_TOKEN,required"`
	GoProxyURL          string       `env:"PILOT_GO_PROXY_URL,required"`
	GoProxyUsername     string       `env:"PILOT_GO_PROXY_USERNAME,required"`
	GoProxyPassword     string       `env:"PILOT_GO_PROXY_PASSWORD,required"`
	GitHubToken         string       `env:"PILOT_GITHUB_TOKEN,required"`
}

type tracing struct {
	Endpoint    string  `env:"TRACING_ENDPOINT" envDefault:"https://collector.infra.traefiklabs.tech"`
	Username    string  `env:"TRACING_USERNAME" envDefault:"jaeger"`
	Password    string  `env:"TRACING_PASSWORD" envDefault:"jaeger"`
	Probability float64 `env:"TRACING_PROBABILITY" envDefault:"0.1"`
}

// config represents the configuration of this application.
type config struct {
	FaunaDB faunaDB
	Pilot   pilot
	Tracing tracing
}

// base64String implements encoding.TextUnmarshaler.
type base64String string

func (b *base64String) UnmarshalText(data []byte) error {
	value, err := base64.StdEncoding.DecodeString(string(data))
	if err != nil {
		return err
	}

	*b = base64String(value)

	return nil
}
