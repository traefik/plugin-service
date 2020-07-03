package handlers

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"regexp"

	"github.com/containous/plugin-service/internal/token"
	"github.com/containous/plugin-service/pkg/db"
	"github.com/fauna/faunadb-go/faunadb"
	"github.com/google/go-github/v32/github"
	"github.com/ldez/grignotin/goproxy"
)

const (
	nextPageHeader = "X-Next-Page"
	defaultPerPage = 100
)

// Handlers a set of handlers.
type Handlers struct {
	db      db.PluginDB
	goProxy *goproxy.Client
	gh      *github.Client
	token   *token.Client
}

// New creates all HTTP handlers.
func New(db db.PluginDB, goProxy *goproxy.Client, gh *github.Client, tokenClient *token.Client) Handlers {
	return Handlers{
		db:      db,
		goProxy: goProxy,
		token:   tokenClient,
	}
}

// Get gets a plugin.
func (h Handlers) Get(rw http.ResponseWriter, req *http.Request) {
	rw.Header().Set("Content-Type", "application/json")

	id, err := getPathParam(req.URL)
	if err != nil {
		log.Println("missing plugin id")
		jsonError(rw, http.StatusBadRequest, "missing plugin id")
		return
	}

	plugin, err := h.db.Get(id)
	if err != nil {
		var notFoundError faunadb.NotFound
		if errors.As(err, &notFoundError) {
			log.Printf("plugin not found: %s", id)
			jsonError(rw, http.StatusNotFound, "plugin not found")
			return
		}

		log.Printf("Error while fetch: %v", err)

		jsonError(rw, http.StatusInternalServerError, "error")
		return
	}

	if err := json.NewEncoder(rw).Encode(plugin); err != nil {
		log.Printf("failed to get plugin: %v", err)
		jsonError(rw, http.StatusInternalServerError, "could not write response")
		return
	}
}

// List gets a list of plugins.
func (h Handlers) List(rw http.ResponseWriter, req *http.Request) {
	rw.Header().Set("Content-Type", "application/json")

	name := req.URL.Query().Get("name")
	if name != "" {
		plugin, err := h.db.GetByName(name)
		if err != nil {
			var notFoundError faunadb.NotFound
			if errors.As(err, &notFoundError) {
				log.Printf("plugin not found: %s", name)
				jsonError(rw, http.StatusNotFound, "plugin not found")
				return
			}

			log.Printf("Error while fetch: %v", err)

			jsonError(rw, http.StatusInternalServerError, "error")
			return
		}

		if err := json.NewEncoder(rw).Encode([]*db.Plugin{&plugin}); err != nil {
			log.Printf("failed to get plugin: %v", err)
			jsonError(rw, http.StatusInternalServerError, "could not write response")
			return
		}

		return
	}

	start := req.URL.Query().Get("start")
	plugins, next, err := h.db.List(db.Pagination{
		Start: start,
		Size:  defaultPerPage,
	})

	if err != nil {
		log.Printf("Error fetching plugins: %v", err)
		jsonError(rw, http.StatusNotFound, "could not fetch plugins")
		return
	}

	if len(plugins) == 0 {
		if err := json.NewEncoder(rw).Encode(make([]*db.Plugin, 0)); err != nil {
			log.Printf("Error sending create response: %v", err)
			jsonError(rw, http.StatusInternalServerError, "could not write response")
		}
		return
	}

	rw.Header().Set(nextPageHeader, next)

	if err := json.NewEncoder(rw).Encode(plugins); err != nil {
		log.Printf("Error sending create response: %v", err)
		jsonError(rw, http.StatusInternalServerError, "could not write response")
		return
	}
}

// Create creates a plugin.
func (h Handlers) Create(rw http.ResponseWriter, req *http.Request) {
	rw.Header().Set("Content-Type", "application/json")

	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.Printf("Error reading body for creation: %v", err)
		jsonError(rw, http.StatusInternalServerError, err.Error())
		return
	}

	if len(body) == 0 {
		log.Printf("Error decoding plugin for creation: %v", err)
		jsonError(rw, http.StatusBadRequest, err.Error())
		return
	}

	pl := db.Plugin{}

	err = json.Unmarshal(body, &pl)
	if err != nil {
		log.Printf("Error decoding plugin for creation: %v", err)
		jsonError(rw, http.StatusBadRequest, err.Error())
		return
	}

	created, err := h.db.Create(pl)
	if err != nil {
		log.Printf("Error persisting plugin %s: %v", pl.Name, err)
		jsonError(rw, http.StatusInternalServerError, "could not persist data")
		return
	}

	rw.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(rw).Encode(created); err != nil {
		log.Printf("Error sending create response: %v", err)
		jsonError(rw, http.StatusInternalServerError, "could not write response")
		return
	}
}

// Update updates a plugin.
func (h Handlers) Update(rw http.ResponseWriter, req *http.Request) {
	rw.Header().Set("Content-Type", "application/json")

	id, err := getPathParam(req.URL)
	if err != nil {
		jsonError(rw, http.StatusBadRequest, "missing token id")
		return
	}

	input := db.Plugin{}
	err = json.NewDecoder(req.Body).Decode(&input)
	if err != nil {
		log.Printf("Error reading body for update: %v", err)
		jsonError(rw, http.StatusBadRequest, err.Error())
		return
	}

	pg, err := h.db.Update(id, input)
	if err != nil {
		var notFoundError faunadb.NotFound
		if errors.As(err, &notFoundError) {
			log.Printf("plugin not found: %s", id)
			jsonError(rw, http.StatusNotFound, "plugin not found")
			return
		}

		log.Printf("Error updating token: %v", err)

		jsonError(rw, http.StatusInternalServerError, "could not update token")
		return
	}

	rw.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(rw).Encode(pg); err != nil {
		log.Printf("failed to marshal token: %v", err)
		jsonError(rw, http.StatusInternalServerError, "could not write response")
		return
	}
}

// Delete deletes an instance info.
func (h Handlers) Delete(rw http.ResponseWriter, req *http.Request) {
	id, err := getPathParam(req.URL)
	if err != nil {
		jsonError(rw, http.StatusBadRequest, "missing instance info id")
		return
	}

	_, err = h.db.Get(id)
	if err != nil {
		log.Printf("failed to get plugin information: %v", err)
		NotFound(rw, req)
		return
	}

	err = h.db.Delete(id)
	if err != nil {
		log.Printf("failed to delete the plugin info: %v", err)
		jsonError(rw, http.StatusBadRequest, err.Error())
		return
	}

	err = h.db.DeleteHash(id)
	if err != nil {
		var notFoundError faunadb.NotFound
		if !errors.As(err, &notFoundError) {
			log.Printf("failed to delete the plugin hash: %v", err)
			jsonError(rw, http.StatusBadRequest, err.Error())
			return
		}
	}
}

// NotFound a not found handler.
func NotFound(rw http.ResponseWriter, _ *http.Request) {
	jsonError(rw, http.StatusNotFound, http.StatusText(http.StatusNotFound))
}

// PanicHandler handles panics.
func PanicHandler(rw http.ResponseWriter, req *http.Request, err interface{}) {
	log.Printf("Panic error executing request %s %s: %v", req.Method, req.URL, err)
	jsonError(rw, http.StatusInternalServerError, "panic")
}

func getPathParam(uri *url.URL) (string, error) {
	exp := regexp.MustCompile(`^/([\w-]+)/?$`)
	parts := exp.FindStringSubmatch(uri.Path)

	if len(parts) != 2 {
		return "", errors.New("missing id")
	}

	return parts[1], nil
}
