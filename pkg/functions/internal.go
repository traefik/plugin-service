package functions

import (
	"net/http"

	"github.com/caarlos0/env/v6"
	"github.com/fauna/faunadb-go/v3/faunadb"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/julienschmidt/httprouter"
	"github.com/rs/zerolog/log"
	"github.com/traefik/plugin-service/pkg/db"
	"github.com/traefik/plugin-service/pkg/handlers"
	"github.com/traefik/plugin-service/pkg/jwt"
	"github.com/traefik/plugin-service/pkg/logger"
	"github.com/traefik/plugin-service/pkg/tracer"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// Internal creates internal function.
func Internal(rw http.ResponseWriter, req *http.Request) {
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

	retryClient := retryablehttp.NewClient()
	retryClient.Logger = &log.Logger
	retryClient.RetryMax = faunaRetryMax
	retryClient.RetryWaitMin = faunaRetryWaitMin
	retryClient.RetryWaitMax = faunaRetryWaitMax

	options = append(options, faunadb.HTTP(retryClient.StandardClient()), faunadb.Observer(observer))

	handler := handlers.New(
		db.NewFaunaDB(faunadb.NewFaunaClient(cfg.FaunaDB.Secret, options...)),
		nil,
		nil,
		nil,
	)

	router := httprouter.New()
	router.Handler(http.MethodGet, "/", otelhttp.NewHandler(http.HandlerFunc(handler.List), "internal_list"))
	router.Handler(http.MethodPost, "/", otelhttp.NewHandler(http.HandlerFunc(handler.Create), "internal_create"))
	router.Handler(http.MethodPut, "/:uuid", otelhttp.NewHandler(http.HandlerFunc(handler.Update), "internal_update"))
	router.Handler(http.MethodDelete, "/:uuid", otelhttp.NewHandler(http.HandlerFunc(handler.Delete), "internal_delete"))

	router.PanicHandler = handlers.PanicHandler

	jwt.NewHandler(cfg.Pilot.JWTCert,
		jwt.ServicesAudience,
		jwt.Issuer,
		map[string]jwt.Check{"sub": {Value: "Ie2dYtbQ5N5hRz4cNHZNKJ3WHrp62Mr7@clients"}},
		http.StripPrefix("/internal", router),
	).ServeHTTP(rw, req)
}
