package functions

import (
	"net/http"
	"os"

	"github.com/containous/plugin-service/pkg/db"
	"github.com/containous/plugin-service/pkg/handlers"
	"github.com/fauna/faunadb-go/faunadb"
	"github.com/julienschmidt/httprouter"
)

// Internal creates zeit function.
func Internal(rw http.ResponseWriter, req *http.Request) {
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
	router.HandlerFunc(http.MethodPost, "/", hdl.Create)
	router.HandlerFunc(http.MethodPut, "/:uuid", hdl.Update)
	router.HandlerFunc(http.MethodDelete, "/:uuid", hdl.Delete)

	router.PanicHandler = handlers.PanicHandler

	newJWTHandler(cert,
		"https://services.pilot.traefik.io/",
		"https://sso.traefik.io/",
		map[string]check{"sub": {value: "Ie2dYtbQ5N5hRz4cNHZNKJ3WHrp62Mr7@clients"}},
		http.StripPrefix("/internal", router),
	).ServeHTTP(rw, req)
}
