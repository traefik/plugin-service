package functions

import (
	"context"
	"net/http"

	"github.com/caarlos0/env/v6"
	"github.com/fauna/faunadb-go/v3/faunadb"
	"github.com/google/go-github/v32/github"
	"github.com/gorilla/mux"
	"github.com/ldez/grignotin/goproxy"
	"github.com/rs/zerolog/log"
	"github.com/traefik/plugin-service/internal/token"
	"github.com/traefik/plugin-service/pkg/db"
	"github.com/traefik/plugin-service/pkg/handlers"
	"github.com/traefik/plugin-service/pkg/logger"
	"github.com/traefik/plugin-service/pkg/tracer"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"golang.org/x/oauth2"
)

// Public creates public function.
func Public(rw http.ResponseWriter, req *http.Request) {
	logger.Setup()

	cfg := config{}

	err := env.Parse(&cfg)
	if err != nil {
		log.Error().Err(err).Msg("Unable to parse env vars")
		handlers.JSONInternalServerError(rw)
		return
	}

	exporter, err := tracer.NewJaegerExporter(cfg.Tracing.Endpoint, cfg.Tracing.Username, cfg.Tracing.Password)
	if err != nil {
		log.Error().Err(err).Msg("Unable to configure new exporter.")
		handlers.JSONInternalServerError(rw)
		return
	}
	defer exporter.Flush()

	bsp := tracer.Setup(exporter, cfg.Tracing.Probability)
	defer func() { _ = bsp.Shutdown(req.Context()) }()

	var options []faunadb.ClientConfig
	if cfg.FaunaDB.Endpoint != "" {
		options = append(options, faunadb.Endpoint(cfg.FaunaDB.Endpoint))
	}

	options = append(options, faunadb.Observer(observer))

	gpClient, err := newGoProxyClient(cfg.Pilot.GoProxyURL, cfg.Pilot.GoProxyUsername, cfg.Pilot.GoProxyPassword)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create go proxy client")
		handlers.JSONInternalServerError(rw)
		return
	}

	var ghClient *github.Client
	if cfg.Pilot.GitHubToken != "" {
		ghClient = newGitHubClient(context.Background(), cfg.Pilot.GitHubToken)
	}

	handler := handlers.New(
		db.NewFaunaDB(faunadb.NewFaunaClient(cfg.FaunaDB.Secret, options...)),
		gpClient,
		ghClient,
		token.New(cfg.Pilot.TokenURL, string(cfg.Pilot.ServicesAccessToken)),
	)

	r := mux.NewRouter()
	r.Handle("/", otelhttp.NewHandler(http.HandlerFunc(handler.List), "public_list"))
	r.Handle("/download/{all:.+}", otelhttp.NewHandler(http.HandlerFunc(handler.Download), "public_download"))
	r.Handle("/validate/{all:.+}", otelhttp.NewHandler(http.HandlerFunc(handler.Validate), "public_validate"))
	r.Handle("/{uuid}", otelhttp.NewHandler(http.HandlerFunc(handler.Get), "public_get"))

	r.NotFoundHandler = http.HandlerFunc(handlers.NotFound)

	http.StripPrefix("/public", r).ServeHTTP(rw, req)
}

func newGoProxyClient(proxyURL, username, password string) (*goproxy.Client, error) {
	gpClient := goproxy.NewClient(proxyURL)

	if proxyURL != "" && username != "" && password != "" {
		tr, err := goproxy.NewBasicAuthTransport(username, password)
		if err != nil {
			return nil, err
		}

		gpClient.HTTPClient = tr.Client()
	}

	return gpClient, nil
}

func newGitHubClient(ctx context.Context, token string) *github.Client {
	if len(token) == 0 {
		return github.NewClient(nil)
	}

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	return github.NewClient(oauth2.NewClient(ctx, ts))
}
