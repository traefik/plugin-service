package functions

import (
	"net/http"

	"github.com/caarlos0/env/v6"
	"github.com/fauna/faunadb-go/v3/faunadb"
	"github.com/julienschmidt/httprouter"
	"github.com/rs/zerolog/log"
	"github.com/traefik/plugin-service/pkg/db"
	"github.com/traefik/plugin-service/pkg/handlers"
	"github.com/traefik/plugin-service/pkg/logger"
	"github.com/traefik/plugin-service/pkg/tracer"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// External creates zeit function.
func External(rw http.ResponseWriter, req *http.Request) {
	logger.Setup()

	cfg := config{}

	err := env.Parse(&cfg)
	if err != nil {
		log.Error().Err(err).Msg("Unable to parse env vars")
		jsonError(rw, http.StatusInternalServerError, "internal server error")
		return
	}

	exporter, err := tracer.NewJaegerExporter(req, cfg.Tracing.Endpoint, cfg.Tracing.Username, cfg.Tracing.Password)
	if err != nil {
		log.Error().Err(err).Msg("Unable to configure new exporter.")
		jsonError(rw, http.StatusInternalServerError, "internal server error")
		return
	}
	defer exporter.Flush()

	bsp := tracer.Setup(exporter, cfg.Tracing.Probability)
	defer bsp.Shutdown()

	var options []faunadb.ClientConfig
	if cfg.FaunaDB.Endpoint != "" {
		options = append(options, faunadb.Endpoint(cfg.FaunaDB.Endpoint))
	}

	options = append(options, faunadb.Observer(observer))

	handler := handlers.New(
		db.NewFaunaDB(faunadb.NewFaunaClient(cfg.FaunaDB.Secret, options...), db.FaunaTracing{
			Endpoint: cfg.Tracing.Endpoint,
			Username: cfg.Tracing.Username,
			Password: cfg.Tracing.Password,
		}),
		nil,
		nil,
		nil,
	)

	router := httprouter.New()
	router.Handler(http.MethodGet, "/", otelhttp.NewHandler(http.HandlerFunc(handler.List), "external_list"))
	router.Handler(http.MethodGet, "/:uuid", otelhttp.NewHandler(http.HandlerFunc(handler.Get), "external_get"))

	router.PanicHandler = handlers.PanicHandler

	newJWTHandler(cfg.Pilot.JWTCert,
		"https://clients.pilot.traefik.io/",
		"https://sso.traefik.io/",
		map[string]check{"https://clients.pilot.traefik.io/uuid": {header: "X-User-Id"}},
		http.StripPrefix("/external", router),
	).ServeHTTP(rw, req)
}
