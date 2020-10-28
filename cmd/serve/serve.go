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
	mux.HandleFunc("/", functions.External)
	mux.HandleFunc("/public/", functions.Public)
	mux.HandleFunc("/internal/", functions.Internal)
	mux.HandleFunc("/external/", functions.External)

	return http.ListenAndServe(context.String("host"), mux)
}

func bootstrap(token string, options []faunadb.ClientConfig) error {
	database := db.NewFaunaDB(faunadb.NewFaunaClient(token, options...), db.FaunaTracing{})
	return database.Bootstrap()
}

func setupEnvVars(token string, context *cli.Context) error {
	endpoint := context.String("endpoint")
	if endpoint != "" {
		if err := os.Setenv("FAUNADB_ENDPOINT", endpoint); err != nil {
			return err
		}
	}

	if err := os.Setenv("FAUNADB_SECRET", token); err != nil {
		return err
	}

	if err := os.Setenv("PILOT_TOKEN_URL", context.String("token-url")); err != nil {
		return err
	}

	if err := os.Setenv("PILOT_JWT_CERT", context.String("jwt-cert")); err != nil {
		return err
	}

	if err := os.Setenv("PILOT_SERVICES_ACCESS_TOKEN", context.String("services-access-token")); err != nil {
		return err
	}

	if err := os.Setenv("PILOT_GO_PROXY_URL", context.String("go-proxy-url")); err != nil {
		return err
	}

	if err := os.Setenv("PILOT_GO_PROXY_USERNAME", context.String("go-proxy-username")); err != nil {
		return err
	}

	if err := os.Setenv("PILOT_GO_PROXY_PASSWORD", context.String("go-proxy-password")); err != nil {
		return err
	}

	if err := os.Setenv("PILOT_GITHUB_TOKEN", context.String("github-token")); err != nil {
		return err
	}

	if err := os.Setenv("TRACING_ENDPOINT", context.String("tracing-endpoint")); err != nil {
		return err
	}

	if err := os.Setenv("TRACING_USERNAME", context.String("tracing-username")); err != nil {
		return err
	}

	if err := os.Setenv("TRACING_PASSWORD", context.String("tracing-password")); err != nil {
		return err
	}

	if err := os.Setenv("TRACING_PROBABILITY", context.String("tracing-probability")); err != nil {
		return err
	}

	return nil
}
