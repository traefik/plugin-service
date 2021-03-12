package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strconv"

	"github.com/fauna/faunadb-go/v3/faunadb"
	"github.com/google/go-github/v32/github"
	"github.com/ldez/grignotin/goproxy"
	"github.com/rs/zerolog/log"
	"github.com/traefik/plugin-service/internal/token"
	"github.com/traefik/plugin-service/pkg/db"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

const (
	nextPageHeader = "X-Next-Page"
	defaultPerPage = 100
)

// PluginStorer is capable of storing plugins.
type PluginStorer interface {
	Get(ctx context.Context, id string) (db.Plugin, error)
	Delete(ctx context.Context, id string) error
	Create(context.Context, db.Plugin) (db.Plugin, error)
	List(context.Context, db.Pagination) ([]db.Plugin, string, error)
	GetByName(context.Context, string) (db.Plugin, error)
	SearchByName(context.Context, string, db.Pagination) ([]db.Plugin, string, error)
	Update(context.Context, string, db.Plugin) (db.Plugin, error)

	DeleteHash(ctx context.Context, id string) error
	CreateHash(ctx context.Context, module, version, hash string) (db.PluginHash, error)
	GetHashByName(ctx context.Context, module, version string) (db.PluginHash, error)
}

// Handlers a set of handlers.
type Handlers struct {
	store   PluginStorer
	goProxy *goproxy.Client
	gh      *github.Client
	token   *token.Client
	tracer  trace.Tracer
}

// New creates all HTTP handlers.
func New(store PluginStorer, goProxy *goproxy.Client, gh *github.Client, tokenClient *token.Client) Handlers {
	return Handlers{
		store:   store,
		goProxy: goProxy,
		gh:      gh,
		token:   tokenClient,
		tracer:  otel.Tracer("handler"),
	}
}

// Get gets a plugin.
func (h Handlers) Get(rw http.ResponseWriter, req *http.Request) {
	ctx, span := h.tracer.Start(req.Context(), "handler_get")
	defer span.End()

	rw.Header().Set("Content-Type", "application/json")

	id, err := getPathParam(req.URL)
	if err != nil {
		span.RecordError(err)
		JSONError(rw, http.StatusBadRequest, "missing plugin id")
		return
	}

	plugin, err := h.store.Get(ctx, id)
	if err != nil {
		span.RecordError(err)

		var notFoundError faunadb.NotFound
		if errors.As(err, &notFoundError) {
			JSONError(rw, http.StatusNotFound, "plugin not found")
			return
		}

		log.Error().Str("pluginID", id).Err(err).Msg("Error while fetch")

		JSONError(rw, http.StatusInternalServerError, "error")
		return
	}

	if err := json.NewEncoder(rw).Encode(plugin); err != nil {
		span.RecordError(err)
		log.Error().Str("pluginID", id).Err(err).Msg("failed to get plugin")
		JSONError(rw, http.StatusInternalServerError, "could not write response")
		return
	}
}

// List gets a list of plugins.
func (h Handlers) List(rw http.ResponseWriter, req *http.Request) {
	ctx, span := h.tracer.Start(req.Context(), "handler_list")
	defer span.End()

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
	plugins, next, err := h.store.List(ctx, db.Pagination{
		Start: start,
		Size:  defaultPerPage,
	})
	if err != nil {
		span.RecordError(err)
		log.Error().Err(err).Msg("Error fetching plugins")
		JSONError(rw, http.StatusNotFound, "could not fetch plugins")
		return
	}

	if len(plugins) == 0 {
		if err := json.NewEncoder(rw).Encode(make([]*db.Plugin, 0)); err != nil {
			span.RecordError(err)
			log.Error().Err(err).Msg("Error sending create response")
			JSONError(rw, http.StatusInternalServerError, "could not write response")
		}
		return
	}

	rw.Header().Set(nextPageHeader, next)

	if err := json.NewEncoder(rw).Encode(plugins); err != nil {
		span.RecordError(err)
		log.Error().Err(err).Msg("Error sending create response")
		JSONError(rw, http.StatusInternalServerError, "could not write response")
		return
	}
}

// Create creates a plugin.
func (h Handlers) Create(rw http.ResponseWriter, req *http.Request) {
	ctx, span := h.tracer.Start(req.Context(), "handler_create")
	defer span.End()

	rw.Header().Set("Content-Type", "application/json")

	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		span.RecordError(err)
		log.Error().Err(err).Msg("Error reading body for creation")
		JSONError(rw, http.StatusInternalServerError, err.Error())
		return
	}

	if len(body) == 0 {
		span.RecordError(err)
		log.Error().Err(err).Msg("Error decoding plugin for creation")
		JSONError(rw, http.StatusBadRequest, err.Error())
		return
	}

	pl := db.Plugin{}

	err = json.Unmarshal(body, &pl)
	if err != nil {
		span.RecordError(err)
		log.Error().Err(err).Msg("Error decoding plugin for creation")
		JSONError(rw, http.StatusBadRequest, err.Error())
		return
	}

	created, err := h.store.Create(ctx, pl)
	if err != nil {
		span.RecordError(err)
		log.Error().Str("moduleName", pl.Name).Err(err).Msg("Error persisting plugin")
		JSONError(rw, http.StatusInternalServerError, "could not persist data")
		return
	}

	rw.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(rw).Encode(created); err != nil {
		span.RecordError(err)
		log.Error().Str("moduleName", pl.Name).Err(err).Msg("Error sending create response")
		JSONError(rw, http.StatusInternalServerError, "could not write response")
		return
	}
}

// Update updates a plugin.
func (h Handlers) Update(rw http.ResponseWriter, req *http.Request) {
	ctx, span := h.tracer.Start(req.Context(), "handler_update")
	defer span.End()

	rw.Header().Set("Content-Type", "application/json")

	id, err := getPathParam(req.URL)
	if err != nil {
		span.RecordError(err)
		JSONError(rw, http.StatusBadRequest, "missing plugin id")
		return
	}

	input := db.Plugin{}
	err = json.NewDecoder(req.Body).Decode(&input)
	if err != nil {
		span.RecordError(err)
		log.Error().Err(err).Msg("Error reading body for update")
		JSONError(rw, http.StatusBadRequest, err.Error())
		return
	}

	pg, err := h.store.Update(ctx, id, input)
	if err != nil {
		span.RecordError(err)

		var notFoundError faunadb.NotFound
		if errors.As(err, &notFoundError) {
			span.RecordError(err)
			JSONError(rw, http.StatusNotFound, "plugin not found")
			return
		}

		log.Error().Str("pluginID", id).Err(err).Msg("Error updating plugin")

		JSONError(rw, http.StatusInternalServerError, "could not update plugin")
		return
	}

	rw.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(rw).Encode(pg); err != nil {
		span.RecordError(err)
		log.Error().Str("pluginID", id).Err(err).Msg("failed to marshal plugin")
		JSONError(rw, http.StatusInternalServerError, "could not write response")
		return
	}
}

// Delete deletes an instance info.
func (h Handlers) Delete(rw http.ResponseWriter, req *http.Request) {
	ctx, span := h.tracer.Start(req.Context(), "handler_delete")
	defer span.End()

	id, err := getPathParam(req.URL)
	if err != nil {
		span.RecordError(err)
		log.Error().Err(err).Msg("missing plugin id")
		JSONError(rw, http.StatusBadRequest, "missing plugin id")
		return
	}

	_, err = h.store.Get(ctx, id)
	if err != nil {
		span.RecordError(err)
		log.Error().Str("pluginID", id).Err(err).Msg("failed to get plugin information")
		NotFound(rw, req)
		return
	}

	err = h.store.Delete(ctx, id)
	if err != nil {
		span.RecordError(err)
		log.Error().Str("pluginID", id).Err(err).Msg("failed to delete the plugin info")
		JSONError(rw, http.StatusBadRequest, "failed to delete plugin info")
		return
	}

	err = h.store.DeleteHash(ctx, id)
	if err != nil {
		span.RecordError(err)

		var notFoundError faunadb.NotFound
		if !errors.As(err, &notFoundError) {
			log.Error().Str("pluginID", id).Err(err).Msg("failed to delete the plugin hash")
			JSONError(rw, http.StatusBadRequest, "failed to delete plugin hash")
			return
		}
	}
}

func (h Handlers) searchByName(rw http.ResponseWriter, req *http.Request) {
	ctx, span := h.tracer.Start(req.Context(), "handler_searchByName")
	defer span.End()

	rw.Header().Set("Content-Type", "application/json")

	query := unquote(req.FormValue("query"))

	start := req.URL.Query().Get("start")

	plugins, next, err := h.store.SearchByName(ctx, query, db.Pagination{
		Start: start,
		Size:  defaultPerPage,
	})
	if err != nil {
		span.RecordError(err)
		log.Error().Str("query", query).Err(err).Msg("unable to get plugins by name")
		JSONError(rw, http.StatusBadRequest, "unable to get plugins")
		return
	}

	if len(plugins) == 0 {
		if err := json.NewEncoder(rw).Encode(make([]*db.Plugin, 0)); err != nil {
			span.RecordError(err)
			log.Error().Str("query", query).Err(err).Msg("Error sending create response")
			JSONError(rw, http.StatusInternalServerError, "could not write response")
		}
		return
	}

	rw.Header().Set(nextPageHeader, next)

	if err := json.NewEncoder(rw).Encode(plugins); err != nil {
		span.RecordError(err)
		log.Error().Str("query", query).Err(err).Msg("Error sending create response")
		JSONError(rw, http.StatusInternalServerError, "could not write response")
		return
	}
}

func (h Handlers) getByName(rw http.ResponseWriter, req *http.Request) {
	ctx, span := h.tracer.Start(req.Context(), "handler_getByName")
	defer span.End()

	name := unquote(req.FormValue("name"))

	plugin, err := h.store.GetByName(ctx, name)
	if err != nil {
		span.RecordError(err)

		var notFoundError faunadb.NotFound
		if errors.As(err, &notFoundError) {
			log.Error().Str("pluginName", name).Msg("plugin not found")
			JSONError(rw, http.StatusNotFound, "plugin not found")
			return
		}

		log.Error().Str("pluginName", name).Err(err).Msg("Error while fetch")

		JSONError(rw, http.StatusInternalServerError, "error")
		return
	}

	if err := json.NewEncoder(rw).Encode([]*db.Plugin{&plugin}); err != nil {
		span.RecordError(err)
		log.Error().Str("pluginName", name).Err(err).Msg("failed to get plugin")
		JSONError(rw, http.StatusInternalServerError, "could not write response")
		return
	}
}

// NotFound a not found handler.
func NotFound(rw http.ResponseWriter, _ *http.Request) {
	JSONError(rw, http.StatusNotFound, http.StatusText(http.StatusNotFound))
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
