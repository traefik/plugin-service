package functions

import (
	"net/http"
	"os"

	"github.com/fauna/faunadb-go/faunadb"
	"github.com/julienschmidt/httprouter"
	"github.com/traefik/plugin-service/pkg/db"
	"github.com/traefik/plugin-service/pkg/handlers"
	"github.com/traefik/plugin-service/pkg/logger"
)

// External creates zeit function.
func External(rw http.ResponseWriter, req *http.Request) {
	logger.Setup()

	cert := os.Getenv("PILOT_JWT_CERT")

	dbSecret := os.Getenv("FAUNADB_SECRET")
	dbEndpoint := os.Getenv("FAUNADB_ENDPOINT")

	var options []faunadb.ClientConfig
	if dbEndpoint != "" {
		options = append(options, faunadb.Endpoint(dbEndpoint))
	}

	hdl := handlers.New(db.NewFaunaDB(faunadb.NewFaunaClient(dbSecret, options...)), nil, nil, nil)

	router := httprouter.New()
	router.HandlerFunc(http.MethodGet, "/", hdl.List)
	router.HandlerFunc(http.MethodGet, "/:uuid", hdl.Get)

	router.PanicHandler = handlers.PanicHandler

	newJWTHandler(cert,
		"https://clients.pilot.traefik.io/",
		"https://sso.traefik.io/",
		map[string]check{"https://clients.pilot.traefik.io/uuid": {header: "X-User-Id"}},
		http.StripPrefix("/external", router),
	).ServeHTTP(rw, req)
}
