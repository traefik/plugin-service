package handlers

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strconv"

	"github.com/fauna/faunadb-go/faunadb"
	"github.com/google/go-github/v32/github"
	"github.com/ldez/grignotin/goproxy"
	"github.com/rs/zerolog/log"
	"github.com/traefik/plugin-service/internal/token"
	"github.com/traefik/plugin-service/pkg/db"
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
		gh:      gh,
		token:   tokenClient,
	}
}

// Get gets a plugin.
func (h Handlers) Get(rw http.ResponseWriter, req *http.Request) {
	rw.Header().Set("Content-Type", "application/json")

	id, err := getPathParam(req.URL)
	if err != nil {
		log.Error().Msg("missing plugin id")
		jsonError(rw, http.StatusBadRequest, "missing plugin id")
		return
	}

	logger := log.With().Str("pluginID", id).Logger()

	plugin, err := h.db.Get(id)
	if err != nil {
		var notFoundError faunadb.NotFound
		if errors.As(err, &notFoundError) {
			logger.Error().Msg("plugin not found")
			jsonError(rw, http.StatusNotFound, "plugin not found")
			return
		}

		logger.Error().Err(err).Msg("Error while fetch")

		jsonError(rw, http.StatusInternalServerError, "error")
		return
	}

	if err := json.NewEncoder(rw).Encode(plugin); err != nil {
		logger.Error().Err(err).Msg("failed to get plugin")
		jsonError(rw, http.StatusInternalServerError, "could not write response")
		return
	}
}

// List gets a list of plugins.
func (h Handlers) List(rw http.ResponseWriter, req *http.Request) {
	rw.Header().Set("Content-Type", "application/json")

	if value := req.FormValue("query"); value != "" {
		h.searchByName(rw, req)
		return
	}

	if value := req.FormValue("name"); value != "" {
		h.getByName(rw, req)
		return
	}

	start := req.URL.Query().Get("start")
	plugins, next, err := h.db.List(db.Pagination{
		Start: start,
		Size:  defaultPerPage,
	})
	if err != nil {
		log.Error().Err(err).Msg("Error fetching plugins")
		jsonError(rw, http.StatusNotFound, "could not fetch plugins")
		return
	}

	if len(plugins) == 0 {
		if err := json.NewEncoder(rw).Encode(make([]*db.Plugin, 0)); err != nil {
			log.Error().Err(err).Msg("Error sending create response")
			jsonError(rw, http.StatusInternalServerError, "could not write response")
		}
		return
	}

	rw.Header().Set(nextPageHeader, next)

	if err := json.NewEncoder(rw).Encode(plugins); err != nil {
		log.Error().Err(err).Msg("Error sending create response")
		jsonError(rw, http.StatusInternalServerError, "could not write response")
		return
	}
}

// Create creates a plugin.
func (h Handlers) Create(rw http.ResponseWriter, req *http.Request) {
	rw.Header().Set("Content-Type", "application/json")

	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.Error().Err(err).Msg("Error reading body for creation")
		jsonError(rw, http.StatusInternalServerError, err.Error())
		return
	}

	if len(body) == 0 {
		log.Error().Err(err).Msg("Error decoding plugin for creation")
		jsonError(rw, http.StatusBadRequest, err.Error())
		return
	}

	pl := db.Plugin{}

	err = json.Unmarshal(body, &pl)
	if err != nil {
		log.Error().Err(err).Msg("Error decoding plugin for creation")
		jsonError(rw, http.StatusBadRequest, err.Error())
		return
	}

	logger := log.Error().Str("moduleName", pl.Name)

	created, err := h.db.Create(pl)
	if err != nil {
		logger.Err(err).Msg("Error persisting plugin")
		jsonError(rw, http.StatusInternalServerError, "could not persist data")
		return
	}

	rw.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(rw).Encode(created); err != nil {
		logger.Err(err).Msg("Error sending create response")
		jsonError(rw, http.StatusInternalServerError, "could not write response")
		return
	}
}

// Update updates a plugin.
func (h Handlers) Update(rw http.ResponseWriter, req *http.Request) {
	rw.Header().Set("Content-Type", "application/json")

	id, err := getPathParam(req.URL)
	if err != nil {
		log.Error().Err(err).Msg("missing plugin id")
		jsonError(rw, http.StatusBadRequest, "missing plugin id")
		return
	}

	logger := log.With().Str("pluginID", id).Logger()

	input := db.Plugin{}
	err = json.NewDecoder(req.Body).Decode(&input)
	if err != nil {
		logger.Error().Err(err).Msg("Error reading body for update")
		jsonError(rw, http.StatusBadRequest, err.Error())
		return
	}

	pg, err := h.db.Update(id, input)
	if err != nil {
		var notFoundError faunadb.NotFound
		if errors.As(err, &notFoundError) {
			logger.Error().Msg("plugin not found")
			jsonError(rw, http.StatusNotFound, "plugin not found")
			return
		}

		logger.Error().Err(err).Msg("Error updating token")

		jsonError(rw, http.StatusInternalServerError, "could not update token")
		return
	}

	rw.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(rw).Encode(pg); err != nil {
		logger.Error().Err(err).Msg("failed to marshal token")
		jsonError(rw, http.StatusInternalServerError, "could not write response")
		return
	}
}

// Delete deletes an instance info.
func (h Handlers) Delete(rw http.ResponseWriter, req *http.Request) {
	id, err := getPathParam(req.URL)
	if err != nil {
		log.Error().Err(err).Msg("missing plugin id")
		jsonError(rw, http.StatusBadRequest, "missing plugin id")
		return
	}

	logger := log.With().Str("pluginID", id).Logger()

	_, err = h.db.Get(id)
	if err != nil {
		logger.Error().Err(err).Msg("failed to get plugin information")
		NotFound(rw, req)
		return
	}

	err = h.db.Delete(id)
	if err != nil {
		logger.Error().Err(err).Msg("failed to delete the plugin info")
		jsonError(rw, http.StatusBadRequest, "failed to delete plugin info")
		return
	}

	err = h.db.DeleteHash(id)
	if err != nil {
		var notFoundError faunadb.NotFound
		if !errors.As(err, &notFoundError) {
			logger.Error().Err(err).Msg("failed to delete the plugin hash")
			jsonError(rw, http.StatusBadRequest, "failed to delete plugin hash")
			return
		}
	}
}

func (h Handlers) searchByName(rw http.ResponseWriter, req *http.Request) {
	rw.Header().Set("Content-Type", "application/json")

	query := unquote(req.FormValue("query"))

	logger := log.With().Str("query", query).Logger()

	start := req.URL.Query().Get("start")

	plugins, next, err := h.db.SearchByName(query, db.Pagination{
		Start: start,
		Size:  defaultPerPage,
	})
	if err != nil {
		logger.Err(err).Msg("unable to get plugins by name")
		jsonError(rw, http.StatusBadRequest, "unable to get plugins")
		return
	}

	if len(plugins) == 0 {
		if err := json.NewEncoder(rw).Encode(make([]*db.Plugin, 0)); err != nil {
			log.Error().Err(err).Msg("Error sending create response")
			jsonError(rw, http.StatusInternalServerError, "could not write response")
		}
		return
	}

	rw.Header().Set(nextPageHeader, next)

	if err := json.NewEncoder(rw).Encode(plugins); err != nil {
		logger.Error().Err(err).Msg("Error sending create response")
		jsonError(rw, http.StatusInternalServerError, "could not write response")
		return
	}
}

func (h Handlers) getByName(rw http.ResponseWriter, req *http.Request) {
	name := unquote(req.FormValue("name"))

	logger := log.With().Str("pluginName", name).Logger()

	plugin, err := h.db.GetByName(name)
	if err != nil {
		var notFoundError faunadb.NotFound
		if errors.As(err, &notFoundError) {
			logger.Error().Msg("plugin not found")
			jsonError(rw, http.StatusNotFound, "plugin not found")
			return
		}

		logger.Error().Err(err).Msg("Error while fetch")

		jsonError(rw, http.StatusInternalServerError, "error")
		return
	}

	if err := json.NewEncoder(rw).Encode([]*db.Plugin{&plugin}); err != nil {
		logger.Error().Err(err).Msg("failed to get plugin")
		jsonError(rw, http.StatusInternalServerError, "could not write response")
		return
	}
}

// NotFound a not found handler.
func NotFound(rw http.ResponseWriter, _ *http.Request) {
	jsonError(rw, http.StatusNotFound, http.StatusText(http.StatusNotFound))
}

// PanicHandler handles panics.
func PanicHandler(rw http.ResponseWriter, req *http.Request, err interface{}) {
	log.Error().Str("method", req.Method).Interface("url", req.URL).Interface("err", err).
		Msg("Panic error executing request")
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

func unquote(value string) string {
	unquote, err := strconv.Unquote(value)
	if err != nil {
		return value
	}

	return unquote
}
