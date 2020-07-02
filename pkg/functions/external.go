package functions

import (
	"net/http"
	"os"

	"github.com/containous/plugin-service/pkg/db"
	"github.com/containous/plugin-service/pkg/handlers"
	"github.com/fauna/faunadb-go/faunadb"
	"github.com/julienschmidt/httprouter"
)

// External creates zeit function.
func External(rw http.ResponseWriter, req *http.Request) {
	cert := os.Getenv("PLAEN_JWT_CERT")

	dbSecret := os.Getenv("FAUNADB_SECRET")
	dbEndpoint := os.Getenv("FAUNADB_ENDPOINT")

	var options []faunadb.ClientConfig
	if dbEndpoint != "" {
		options = append(options, faunadb.Endpoint(dbEndpoint))
	}

	hdl := handlers.New(db.NewFaunaDB(faunadb.NewFaunaClient(dbSecret, options...)), nil, nil)

	router := httprouter.New()
	router.HandlerFunc(http.MethodGet, "/", hdl.List)
	router.HandlerFunc(http.MethodGet, "/:uuid", hdl.Get)

	router.PanicHandler = handlers.PanicHandler

	newJWTHandler(cert,
		"https://clients.plaen.io/",
		"https://sso.plaen.io/",
		map[string]check{"https://clients.plaen.io/user_id": {header: "X-User-Id"}},
		http.StripPrefix("/external", router),
	).ServeHTTP(rw, req)
}