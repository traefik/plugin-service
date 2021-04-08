package serve

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/go-github/v32/github"
	"github.com/gorilla/mux"
	"github.com/julienschmidt/httprouter"
	"github.com/ldez/grignotin/goproxy"
	"github.com/traefik/plugin-service/cmd/internal"
	"github.com/traefik/plugin-service/internal/token"
	"github.com/traefik/plugin-service/pkg/handlers"
	"github.com/traefik/plugin-service/pkg/healthcheck"
	"github.com/traefik/plugin-service/pkg/jwt"
	"github.com/traefik/plugin-service/pkg/tracer"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"golang.org/x/oauth2"
)

func run(ctx context.Context, cfg Config) error {
	exporter, err := tracer.NewJaegerExporter(cfg.Tracing.Endpoint, cfg.Tracing.Username, cfg.Tracing.Password)
	if err != nil {
		return fmt.Errorf("unable to configure exporter: %w", err)
	}

	defer exporter.Flush()

	bsp := tracer.Setup(exporter, cfg.Tracing.Probability)
	defer func() { _ = bsp.Shutdown(ctx) }()

	store, tearDown, err := internal.CreateMongoClient(ctx, cfg.MongoDB)
	if err != nil {
		return fmt.Errorf("unable to create MongoDB client: %w", err)
	}
	defer tearDown()

	if err = store.Bootstrap(); err != nil {
		return fmt.Errorf("unable to bootstrap database: %w", err)
	}

	gpClient, err := newGoProxyClient(cfg.GoProxy)
	if err != nil {
		return fmt.Errorf("unable to create go proxy client: %w", err)
	}

	var ghClient *github.Client
	if cfg.Pilot.GitHubToken != "" {
		ghClient = newGitHubClient(context.Background(), cfg.Pilot.GitHubToken)
	}

	handler := handlers.New(
		store,
		gpClient,
		ghClient,
		token.New(cfg.Pilot.TokenURL, cfg.Pilot.ServicesAccessToken),
	)

	healthChecker := healthcheck.Client{DB: store}

	r := http.NewServeMux()

	r.Handle("/public/", buildPublicRouter(handler))
	r.Handle("/internal/", buildInternalRouter(handler, cfg.Pilot))
	r.Handle("/external/", buildExternalRouter(handler, cfg.Pilot))
	r.HandleFunc("/live", healthChecker.Live)
	r.HandleFunc("/ready", healthChecker.Ready)

	return http.ListenAndServe(cfg.Pilot.Host, r)
}

func buildPublicRouter(handler handlers.Handlers) http.Handler {
	r := mux.NewRouter()

	r.Handle("/", otelhttp.NewHandler(http.HandlerFunc(handler.List), "public_list"))
	r.Handle("/download/{all:.+}", otelhttp.NewHandler(http.HandlerFunc(handler.Download), "public_download"))
	r.Handle("/validate/{all:.+}", otelhttp.NewHandler(http.HandlerFunc(handler.Validate), "public_validate"))
	r.Handle("/{uuid}", otelhttp.NewHandler(http.HandlerFunc(handler.Get), "public_get"))

	r.NotFoundHandler = http.HandlerFunc(handlers.NotFound)

	return http.StripPrefix("/public", r)
}

func buildInternalRouter(handler handlers.Handlers, cfg Pilot) http.Handler {
	r := httprouter.New()

	r.Handler(http.MethodGet, "/", otelhttp.NewHandler(http.HandlerFunc(handler.List), "internal_list"))
	r.Handler(http.MethodPost, "/", otelhttp.NewHandler(http.HandlerFunc(handler.Create), "internal_create"))
	r.Handler(http.MethodPut, "/:uuid", otelhttp.NewHandler(http.HandlerFunc(handler.Update), "internal_update"))
	r.Handler(http.MethodDelete, "/:uuid", otelhttp.NewHandler(http.HandlerFunc(handler.Delete), "internal_delete"))

	r.NotFound = http.HandlerFunc(handlers.NotFound)
	r.PanicHandler = handlers.PanicHandler

	return jwt.NewHandler(cfg.JWTCert,
		jwt.ServicesAudience,
		jwt.Issuer,
		map[string]jwt.Check{"sub": {Value: "Ie2dYtbQ5N5hRz4cNHZNKJ3WHrp62Mr7@clients"}},
		http.StripPrefix("/internal", r),
	)
}

func buildExternalRouter(handler handlers.Handlers, cfg Pilot) http.Handler {
	r := httprouter.New()

	r.Handler(http.MethodGet, "/", otelhttp.NewHandler(http.HandlerFunc(handler.List), "external_list"))
	r.Handler(http.MethodGet, "/:uuid", otelhttp.NewHandler(http.HandlerFunc(handler.Get), "external_get"))

	r.NotFound = http.HandlerFunc(handlers.NotFound)
	r.PanicHandler = handlers.PanicHandler

	return jwt.NewHandler(cfg.JWTCert,
		jwt.ClientsAudience,
		jwt.Issuer,
		map[string]jwt.Check{
			jwt.UserIDClaim:         {},
			jwt.OrganizationIDClaim: {},
		},
		http.StripPrefix("/external", r),
	)
}

func newGoProxyClient(cfg GoProxy) (*goproxy.Client, error) {
	gpClient := goproxy.NewClient(cfg.URL)

	if cfg.URL != "" && cfg.Username != "" && cfg.Password != "" {
		tr, err := goproxy.NewBasicAuthTransport(cfg.Username, cfg.Password)
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
