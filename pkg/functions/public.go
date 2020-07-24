package functions

import (
	"context"
	"encoding/base64"
	"net/http"
	"os"

	"github.com/containous/plugin-service/internal/token"
	"github.com/containous/plugin-service/pkg/db"
	"github.com/containous/plugin-service/pkg/handlers"
	"github.com/fauna/faunadb-go/faunadb"
	"github.com/google/go-github/v32/github"
	"github.com/gorilla/mux"
	"github.com/ldez/grignotin/goproxy"
	"github.com/rs/zerolog/log"
	"golang.org/x/oauth2"
)

// Public creates zeit function.
func Public(rw http.ResponseWriter, req *http.Request) {
	tokenBaseURL := os.Getenv("PILOT_TOKEN_URL")

	serviceAccessToken, err := base64.StdEncoding.DecodeString(os.Getenv("PILOT_SERVICES_ACCESS_TOKEN"))
	if err != nil {
		log.Error().Err(err).Msg("Failed to get PILOT_SERVICES_ACCESS_TOKEN")
		jsonError(rw, http.StatusInternalServerError, "internal error")
		return
	}

	dbSecret := os.Getenv("FAUNADB_SECRET")
	dbEndpoint := os.Getenv("FAUNADB_ENDPOINT")

	var options []faunadb.ClientConfig
	if dbEndpoint != "" {
		options = append(options, faunadb.Endpoint(dbEndpoint))
	}

	proxyURL := os.Getenv("PILOT_GO_PROXY_URL")
	proxyUsername := os.Getenv("PILOT_GO_PROXY_USERNAME")
	proxyPassword := os.Getenv("PILOT_GO_PROXY_PASSWORD")

	gpClient, err := newGoProxyClient(proxyURL, proxyUsername, proxyPassword)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create go proxy client")
		jsonError(rw, http.StatusInternalServerError, "internal error")
		return
	}

	ghToken := os.Getenv("PILOT_GITHUB_TOKEN")

	var ghClient *github.Client
	if ghToken != "" {
		ghClient = newGitHubClient(context.Background(), ghToken)
	}

	handler := handlers.New(db.NewFaunaDB(faunadb.NewFaunaClient(dbSecret, options...)), gpClient, ghClient, token.New(tokenBaseURL, string(serviceAccessToken)))

	r := mux.NewRouter()
	r.HandleFunc("/", handler.List)
	r.HandleFunc("/download/{all:.+}", handler.Download)
	r.HandleFunc("/validate/{all:.+}", handler.Validate)
	r.HandleFunc("/{uuid}", handler.Get)

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
