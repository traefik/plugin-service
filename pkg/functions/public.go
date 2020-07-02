package functions

import (
	"encoding/base64"
	"net/http"
	"os"

	"github.com/containous/plugin-service/internal/token"
	"github.com/containous/plugin-service/pkg/db"
	"github.com/containous/plugin-service/pkg/handlers"
	"github.com/fauna/faunadb-go/faunadb"
	"github.com/julienschmidt/httprouter"
	"github.com/ldez/grignotin/goproxy"
)

// Public creates zeit function.
func Public(rw http.ResponseWriter, req *http.Request) {
	tokenBaseURL := os.Getenv("PLAEN_TOKEN_URL")

	serviceAccessToken, err := base64.StdEncoding.DecodeString(os.Getenv("PLAEN_SERVICES_ACCESS_TOKEN"))
	if err != nil {
		jsonError(rw, http.StatusInternalServerError, "internal error")
	}

	dbSecret := os.Getenv("FAUNADB_SECRET")
	dbEndpoint := os.Getenv("FAUNADB_ENDPOINT")

	var options []faunadb.ClientConfig
	if dbEndpoint != "" {
		options = append(options, faunadb.Endpoint(dbEndpoint))
	}

	handler := handlers.New(
		db.NewFaunaDB(faunadb.NewFaunaClient(dbSecret, options...)),
		goproxy.NewClient(""),
		token.New(tokenBaseURL, string(serviceAccessToken)),
	)

	router := httprouter.New()
	router.HandlerFunc(http.MethodGet, "/download", handler.Download)
	router.HandlerFunc(http.MethodGet, "/validate", handler.Validate)

	router.NotFound = http.HandlerFunc(handlers.NotFound)
	router.PanicHandler = handlers.PanicHandler

	http.StripPrefix("/public", router).ServeHTTP(rw, req)
}
