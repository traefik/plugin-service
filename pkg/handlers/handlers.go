package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"

	"github.com/google/go-github/v35/github"
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
		JSONError(rw, http.StatusBadRequest, "Missing plugin id")
		return
	}

	logger := log.With().Str("plugin_id", id).Logger()

	plugin, err := h.store.Get(ctx, id)
	if err != nil {
		span.RecordError(err)

		if errors.As(err, &db.NotFoundError{}) {
			NotFound(rw, req)
			return
		}

		logger.Error().Err(err).Msg("Error while trying to get plugin")
		JSONInternalServerError(rw)
		return
	}

	if err := json.NewEncoder(rw).Encode(plugin); err != nil {
		span.RecordError(err)
		logger.Error().Err(err).Msg("Failed to get plugin")
		JSONInternalServerError(rw)
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

	logger := log.With().Str("search_start", start).Logger()

	plugins, next, err := h.store.List(ctx, db.Pagination{
		Start: start,
		Size:  defaultPerPage,
	})
	if err != nil {
		span.RecordError(err)
		logger.Error().Err(err).Msg("Error fetching plugins")
		NotFound(rw, req)
		return
	}

	if len(plugins) == 0 {
		if err := json.NewEncoder(rw).Encode(make([]*db.Plugin, 0)); err != nil {
			span.RecordError(err)
			logger.Error().Err(err).Msg("Failed to encode response")
			JSONInternalServerError(rw)
		}
		return
	}

	rw.Header().Set(nextPageHeader, next)

	if err := json.NewEncoder(rw).Encode(plugins); err != nil {
		span.RecordError(err)
		logger.Error().Err(err).Msg("Failed to encode response")
		JSONInternalServerError(rw)
		return
	}
}

// Create creates a plugin.
func (h Handlers) Create(rw http.ResponseWriter, req *http.Request) {
	ctx, span := h.tracer.Start(req.Context(), "handler_create")
	defer span.End()

	rw.Header().Set("Content-Type", "application/json")

	body, err := io.ReadAll(req.Body)
	if err != nil {
		span.RecordError(err)
		log.Error().Err(err).Msg("Error reading body for creation")
		JSONError(rw, http.StatusBadRequest, err.Error())
		return
	}

	if len(body) == 0 {
		err = errors.New("empty body")
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

	logger := log.With().Str("module_name", pl.Name).Logger()

	created, err := h.store.Create(ctx, pl)
	if err != nil {
		span.RecordError(err)
		logger.Error().Err(err).Msg("Error persisting plugin")
		JSONError(rw, http.StatusInternalServerError, "Could not persist data")
		return
	}

	rw.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(rw).Encode(created); err != nil {
		span.RecordError(err)
		logger.Error().Err(err).Msg("Error sending create response")
		JSONInternalServerError(rw)
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
		log.Error().Err(err).Msg("Missing plugin id")
		JSONError(rw, http.StatusBadRequest, "Missing plugin id")
		return
	}

	logger := log.With().Str("plugin_id", id).Logger()

	input := db.Plugin{}
	err = json.NewDecoder(req.Body).Decode(&input)
	if err != nil {
		span.RecordError(err)
		logger.Error().Err(err).Msg("Error reading body for update")
		JSONError(rw, http.StatusBadRequest, err.Error())
		return
	}

	pg, err := h.store.Update(ctx, id, input)
	if err != nil {
		span.RecordError(err)

		if errors.As(err, &db.NotFoundError{}) {
			span.RecordError(err)
			log.Error().Err(err).Msg("Plugin not found")
			NotFound(rw, req)
			return
		}

		logger.Error().Err(err).Msg("Error updating plugin")
		JSONInternalServerError(rw)
		return
	}

	rw.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(rw).Encode(pg); err != nil {
		span.RecordError(err)
		logger.Error().Err(err).Msg("Failed to marshal plugin")
		JSONInternalServerError(rw)
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
		log.Warn().Err(err).Msg("Missing plugin id")
		JSONError(rw, http.StatusBadRequest, "Missing plugin id")
		return
	}

	logger := log.With().Str("plugin_id", id).Logger()

	_, err = h.store.Get(ctx, id)
	if err != nil {
		span.RecordError(err)
		logger.Error().Err(err).Msg("Failed to get plugin information")
		NotFound(rw, req)
		return
	}

	err = h.store.Delete(ctx, id)
	if err != nil {
		span.RecordError(err)
		logger.Error().Err(err).Msg("Failed to delete the plugin info")
		JSONError(rw, http.StatusInternalServerError, "Failed to delete plugin info")
		return
	}
}

func (h Handlers) searchByName(rw http.ResponseWriter, req *http.Request) {
	ctx, span := h.tracer.Start(req.Context(), "handler_searchByName")
	defer span.End()

	rw.Header().Set("Content-Type", "application/json")

	query := unquote(req.FormValue("query"))
	start := req.URL.Query().Get("start")

	logger := log.With().Str("search_query", query).Str("search_start", start).Logger()

	plugins, next, err := h.store.SearchByName(ctx, query, db.Pagination{
		Start: start,
		Size:  defaultPerPage,
	})
	if err != nil {
		span.RecordError(err)
		logger.Error().Err(err).Msg("Unable to get plugins by name")
		JSONError(rw, http.StatusBadRequest, "Unable to get plugins")
		return
	}

	if len(plugins) == 0 {
		if err := json.NewEncoder(rw).Encode(make([]*db.Plugin, 0)); err != nil {
			span.RecordError(err)
			logger.Error().Err(err).Msg("Error sending create response")
			JSONInternalServerError(rw)
		}
		return
	}

	rw.Header().Set(nextPageHeader, next)

	if err := json.NewEncoder(rw).Encode(plugins); err != nil {
		span.RecordError(err)
		logger.Error().Err(err).Msg("Error sending create response")
		JSONInternalServerError(rw)
		return
	}
}

func (h Handlers) getByName(rw http.ResponseWriter, req *http.Request) {
	ctx, span := h.tracer.Start(req.Context(), "handler_getByName")
	defer span.End()

	name := unquote(req.FormValue("name"))

	logger := log.With().Str("module_name", name).Logger()

	plugin, err := h.store.GetByName(ctx, name)
	if err != nil {
		span.RecordError(err)

		if errors.As(err, &db.NotFoundError{}) {
			logger.Error().Msg("plugin not found")
			NotFound(rw, req)
			return
		}

		logger.Error().Err(err).Msg("Error while fetch")
		JSONInternalServerError(rw)
		return
	}

	if err := json.NewEncoder(rw).Encode([]*db.Plugin{&plugin}); err != nil {
		span.RecordError(err)
		logger.Error().Err(err).Msg("Failed to encode response")
		JSONInternalServerError(rw)
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
