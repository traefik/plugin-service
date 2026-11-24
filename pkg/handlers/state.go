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

// errNegativeStars is the message returned for a plugin with negative stars, whatever the endpoint.
const errNegativeStars = "stars must not be negative"

// PluginStateStorer is capable of reading and updating the plugin state used by the scraper.
type PluginStateStorer interface {
	ListStates(ctx context.Context) ([]db.PluginState, error)
	Patch(ctx context.Context, id string, patch db.PluginPatch) error
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

	id, err := getPathParam(req.URL)
	if err != nil {
		span.RecordError(err)
		JSONError(rw, http.StatusBadRequest, "Missing plugin id")

		return
	}

	logger := log.With().Str("plugin_id", id).Logger()

	patch, err := decodePatch(rw, req)
	if err != nil {
		span.RecordError(err)
		logger.Error().Err(err).Msg("Unable to decode plugin patch")

		if errors.As(err, new(*http.MaxBytesError)) {
			JSONError(rw, http.StatusRequestEntityTooLarge, err.Error())
		} else {
			JSONError(rw, http.StatusBadRequest, err.Error())
		}

		return
	}

	if patch.Stars != nil && *patch.Stars < 0 {
		JSONError(rw, http.StatusBadRequest, errNegativeStars)

		return
	}

	if err = h.store.Patch(ctx, id, patch); err != nil {
		span.RecordError(err)

		switch {
		case errors.Is(err, db.ErrEmptyPatch):
			JSONError(rw, http.StatusBadRequest, err.Error())
		case errors.As(err, &db.NotFoundError{}):
			NotFound(rw, req)
		default:
			logger.Error().Err(err).Msg("Error patching plugin")
			JSONInternalServerError(rw)
		}

		return
	}

	rw.WriteHeader(http.StatusNoContent)
}

// decodePatch decodes a body holding a single JSON object. An empty body is an empty patch.
func decodePatch(rw http.ResponseWriter, req *http.Request) (db.PluginPatch, error) {
	var patch db.PluginPatch

	dec := json.NewDecoder(http.MaxBytesReader(rw, req.Body, maxPatchSize))
	dec.DisallowUnknownFields()

	if err := dec.Decode(&patch); err != nil && !errors.Is(err, io.EOF) {
		return db.PluginPatch{}, err
	}

	// Anything after the object, even a lone "}" or "]", is rejected.
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return db.PluginPatch{}, errors.Join(errors.New("body must contain a single JSON object"), err)
	}

	return patch, nil
}
