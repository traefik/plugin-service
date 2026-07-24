package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"

	"github.com/rs/zerolog/log"
	"github.com/traefik/plugin-service/pkg/db"
)

// repositoryRegexp matches a GitHub full name: owner/repo (exactly one slash, no spaces).
var repositoryRegexp = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)

// BlacklistStorer is capable of storing the plugin blacklist.
type BlacklistStorer interface {
	ListBlacklist(ctx context.Context) ([]db.BlacklistEntry, error)
	UpsertBlacklist(ctx context.Context, entry db.BlacklistEntry) (db.BlacklistEntry, error)
	DeleteBlacklist(ctx context.Context, repository string) error
}

// ListBlacklist lists the blacklisted repositories.
func (h Handlers) ListBlacklist(rw http.ResponseWriter, req *http.Request) {
	ctx, span := h.tracer.Start(req.Context(), "handler_list_blacklist")
	defer span.End()

	rw.Header().Set("Content-Type", "application/json")

	entries, err := h.store.ListBlacklist(ctx)
	if err != nil {
		span.RecordError(err)
		log.Error().Err(err).Msg("Error fetching blacklist")
		JSONInternalServerError(rw)

		return
	}

	if err := json.NewEncoder(rw).Encode(entries); err != nil {
		span.RecordError(err)
		log.Error().Err(err).Msg("Failed to encode blacklist response")
		JSONInternalServerError(rw)

		return
	}
}

// AddToBlacklist adds (or updates) a repository in the blacklist.
func (h Handlers) AddToBlacklist(rw http.ResponseWriter, req *http.Request) {
	ctx, span := h.tracer.Start(req.Context(), "handler_add_to_blacklist")
	defer span.End()

	rw.Header().Set("Content-Type", "application/json")

	body, err := io.ReadAll(req.Body)
	if err != nil {
		span.RecordError(err)
		log.Error().Err(err).Msg("Unable to read body for adding entry in the blacklist")
		JSONError(rw, http.StatusBadRequest, err.Error())

		return
	}

	var entry db.BlacklistEntry
	if err = json.Unmarshal(body, &entry); err != nil {
		span.RecordError(err)
		log.Error().Err(err).Msg("Unable to decode blacklist entry")
		JSONError(rw, http.StatusBadRequest, err.Error())

		return
	}

	if !repositoryRegexp.MatchString(entry.Repository) {
		JSONError(rw, http.StatusBadRequest, "invalid repository, expected owner/repo")

		return
	}

	logger := log.With().Str("repository", entry.Repository).Logger()

	created, err := h.store.UpsertBlacklist(ctx, entry)
	if err != nil {
		span.RecordError(err)
		logger.Error().Err(err).Msg("Unable to persist blacklist entry")
		JSONInternalServerError(rw)

		return
	}

	if err := json.NewEncoder(rw).Encode(created); err != nil {
		span.RecordError(err)
		logger.Error().Err(err).Msg("Unable to send blacklist response")
		JSONInternalServerError(rw)

		return
	}
}

// DeleteFromBlacklist removes a repository from the blacklist.
// The repository is passed in the path: DELETE /internal/blacklist/{owner}/{repo}.
func (h Handlers) DeleteFromBlacklist(rw http.ResponseWriter, req *http.Request) {
	ctx, span := h.tracer.Start(req.Context(), "handler_delete_from_blacklist")
	defer span.End()

	repository := req.PathValue("owner") + "/" + req.PathValue("repo")

	logger := log.With().Str("repository", repository).Logger()

	err := h.store.DeleteBlacklist(ctx, repository)
	if err != nil && !errors.As(err, &db.NotFoundError{}) {
		span.RecordError(err)
		logger.Error().Err(err).Msg("Failed to delete blacklist entry")
		JSONInternalServerError(rw)

		return
	}

	rw.WriteHeader(http.StatusNoContent)
}
