package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/rs/zerolog/log"
	"github.com/traefik/plugin-service/pkg/db"
)

// maxPatchSize is the maximum size of a patch body.
const maxPatchSize = 1 << 20

// PluginStateStorer is capable of reading and updating the plugin state used by the scraper.
type PluginStateStorer interface {
	ListStates(ctx context.Context) ([]db.PluginState, error)
	Patch(ctx context.Context, id string, patch db.PluginPatch) (db.Plugin, error)
}

// ListStates returns the state of all plugins, hidden and disabled plugins included, without pagination.
func (h Handlers) ListStates(rw http.ResponseWriter, req *http.Request) {
	ctx, span := h.tracer.Start(req.Context(), "handler_list_states")
	defer span.End()

	rw.Header().Set("Content-Type", "application/json")

	states, err := h.store.ListStates(ctx)
	if err != nil {
		span.RecordError(err)
		log.Error().Err(err).Msg("Error fetching plugin states")
		JSONInternalServerError(rw)

		return
	}

	if err := json.NewEncoder(rw).Encode(states); err != nil {
		span.RecordError(err)
		log.Error().Err(err).Msg("Failed to encode plugin states")
		JSONInternalServerError(rw)

		return
	}
}

// Patch updates only the given fields of a plugin (stars, hidden, tagsETag).
func (h Handlers) Patch(rw http.ResponseWriter, req *http.Request) {
	ctx, span := h.tracer.Start(req.Context(), "handler_patch")
	defer span.End()

	rw.Header().Set("Content-Type", "application/json")

	id, err := getPathParam(req.URL)
	if err != nil {
		span.RecordError(err)
		JSONError(rw, http.StatusBadRequest, "Missing plugin id")

		return
	}

	logger := log.With().Str("plugin_id", id).Logger()

	var patch db.PluginPatch

	dec := json.NewDecoder(http.MaxBytesReader(rw, req.Body, maxPatchSize))
	dec.DisallowUnknownFields()

	// An empty body is an empty patch.
	if err = dec.Decode(&patch); err != nil && !errors.Is(err, io.EOF) {
		span.RecordError(err)
		logger.Error().Err(err).Msg("Unable to decode plugin patch")
		JSONError(rw, http.StatusBadRequest, err.Error())

		return
	}

	if dec.More() {
		JSONError(rw, http.StatusBadRequest, "body must contain a single JSON object")

		return
	}

	if patch.IsEmpty() {
		JSONError(rw, http.StatusBadRequest, "nothing to update")

		return
	}

	if patch.Stars != nil && *patch.Stars < 0 {
		JSONError(rw, http.StatusBadRequest, "stars must not be negative")

		return
	}

	updated, err := h.store.Patch(ctx, id, patch)
	if err != nil {
		span.RecordError(err)

		if errors.As(err, &db.NotFoundError{}) {
			NotFound(rw, req)

			return
		}

		logger.Error().Err(err).Msg("Error patching plugin")
		JSONInternalServerError(rw)

		return
	}

	if err := json.NewEncoder(rw).Encode(updated); err != nil {
		span.RecordError(err)
		logger.Error().Err(err).Msg("Failed to encode patched plugin")
		JSONInternalServerError(rw)

		return
	}
}
