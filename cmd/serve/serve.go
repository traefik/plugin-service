package serve

import (
	"net/http"
	"os"

	"github.com/fauna/faunadb-go/v3/faunadb"
	"github.com/traefik/plugin-service/cmd/internal"
	"github.com/traefik/plugin-service/pkg/db"
	"github.com/traefik/plugin-service/pkg/functions"
	"github.com/urfave/cli/v2"
)

// Run executes user service.
func Run(context *cli.Context) error {
	endpoint := context.String("faunadb-endpoint")

	var options []faunadb.ClientConfig
	token, options, err := internal.GetDBClientParameters(endpoint, context.String("faunadb-secret"))
	if err != nil {
		return err
	}

	if err = setupEnvVars(token, context); err != nil {
		return err
	}

	if err = bootstrap(token, options); err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/public/", functions.Public)
	mux.HandleFunc("/internal/", functions.Internal)
	mux.HandleFunc("/external/", functions.External)

	return http.ListenAndServe(context.String("host"), mux)
}

func bootstrap(token string, options []faunadb.ClientConfig) error {
	database := db.NewFaunaDB(faunadb.NewFaunaClient(token, options...))
	return database.Bootstrap()
}

func setupEnvVars(token string, context *cli.Context) error {
	if err := os.Setenv("FAUNADB_SECRET", token); err != nil {
		return err
	}

	vars := map[string]string{
		"FAUNADB_ENDPOINT":            "endpoint",
		"PILOT_TOKEN_URL":             "token-url",
		"PILOT_JWT_CERT":              "jwt-cert",
		"PILOT_SERVICES_ACCESS_TOKEN": "services-access-token",
		"PILOT_GO_PROXY_URL":          "go-proxy-url",
		"PILOT_GO_PROXY_USERNAME":     "go-proxy-username",
		"PILOT_GO_PROXY_PASSWORD":     "go-proxy-password",
		"PILOT_GITHUB_TOKEN":          "github-token",
		"TRACING_ENDPOINT":            "tracing-endpoint",
		"TRACING_USERNAME":            "tracing-username",
		"TRACING_PASSWORD":            "tracing-password",
		"TRACING_PROBABILITY":         "tracing-probability",
	}

	for name, flag := range vars {
		if err := os.Setenv(name, context.String(flag)); err != nil {
			return err
		}
	}

	return nil
}
