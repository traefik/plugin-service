package serve

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/julienschmidt/httprouter"
	"github.com/traefik/hub-trace-kpi/trace"
	"github.com/traefik/plugin-service/cmd/internal"
	"github.com/traefik/plugin-service/pkg/handlers"
	"github.com/traefik/plugin-service/pkg/healthcheck"
	"github.com/traefik/plugin-service/pkg/tracer"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

func run(ctx context.Context, cfg Config) error {
	stopTracer, err := setupTracing(ctx, cfg.Tracing, cfg.TraceURL)
	if err != nil {
		return fmt.Errorf("setup tracing provider: %w", err)
	}
	defer stopTracer()

	store, tearDown, err := internal.CreateMongoClient(ctx, cfg.MongoDB)
	if err != nil {
		return fmt.Errorf("unable to create MongoDB client: %w", err)
	}
	defer tearDown()

	if err = store.Bootstrap(); err != nil {
		return fmt.Errorf("unable to bootstrap database: %w", err)
	}

	var gpClient handlers.GoproxyPluginClient
	gpClient, err = handlers.NewGoproxyClient(cfg.GoProxy.URL, cfg.GoProxy.Username, cfg.GoProxy.Password)
	if err != nil {
		return fmt.Errorf("unable to create go proxy client: %w", err)
	}

	var ghClient handlers.GithubPluginClient
	if cfg.GitHubToken != "" {
		ghClient = handlers.NewGithubClient(context.Background(), cfg.GitHubToken)
	}

	handler := handlers.New(store, gpClient, ghClient, cfg.TTL)

	healthChecker := healthcheck.Client{DB: store}

	r := http.NewServeMux()

	r.Handle("/public/", buildPublicRouter(handler))
	r.Handle("/internal/", buildInternalRouter(handler))
	r.Handle("/external/", buildExternalRouter(handler))
	r.HandleFunc("/live", healthChecker.Live)
	r.HandleFunc("/ready", healthChecker.Ready)

	return http.ListenAndServe(cfg.Addr, r)
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

func buildInternalRouter(handler handlers.Handlers) http.Handler {
	r := httprouter.New()

	r.Handler(http.MethodGet, "/", otelhttp.NewHandler(http.HandlerFunc(handler.List), "internal_list"))
	r.Handler(http.MethodPost, "/", otelhttp.NewHandler(http.HandlerFunc(handler.Create), "internal_create"))
	r.Handler(http.MethodPut, "/:uuid", otelhttp.NewHandler(http.HandlerFunc(handler.Update), "internal_update"))
	r.Handler(http.MethodDelete, "/:uuid", otelhttp.NewHandler(http.HandlerFunc(handler.Delete), "internal_delete"))

	r.NotFound = http.HandlerFunc(handlers.NotFound)
	r.PanicHandler = handlers.PanicHandler

	return http.StripPrefix("/internal", r)
}

func buildExternalRouter(handler handlers.Handlers) http.Handler {
	r := httprouter.New()

	r.Handler(http.MethodGet, "/", otelhttp.NewHandler(http.HandlerFunc(handler.List), "external_list"))
	r.Handler(http.MethodGet, "/:uuid", otelhttp.NewHandler(http.HandlerFunc(handler.Get), "external_get"))

	r.NotFound = http.HandlerFunc(handlers.NotFound)
	r.PanicHandler = handlers.PanicHandler

	return http.StripPrefix("/external", r)
}

func setupTracing(ctx context.Context, cfg tracer.Config, traceServiceURL string) (func(), error) {
	tracePropagator := propagation.NewCompositeTextMapPropagator(propagation.TraceContext{})
	traceProvider, err := tracer.NewOTLPProvider(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("setup tracing provider: %w", err)
	}

	otel.SetTracerProvider(traceProvider)
	otel.SetTextMapPropagator(tracePropagator)

	if traceServiceURL != "" {
		provider := trace.NewProvider("plugin-service", traceProvider, http.DefaultClient, traceServiceURL, 2)
		provider.StartWorkers(ctx)

		otel.SetTracerProvider(provider)
	}

	return func() {
		_ = traceProvider.Stop(ctx)
	}, nil
}
