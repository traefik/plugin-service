package handlers

import (
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
		log.Error().Err(err).Msg("Error reading body for blacklist creation")
		JSONError(rw, http.StatusBadRequest, err.Error())

		return
	}

	entry := db.BlacklistEntry{}
	if err = json.Unmarshal(body, &entry); err != nil {
		span.RecordError(err)
		log.Error().Err(err).Msg("Error decoding blacklist entry")
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
		logger.Error().Err(err).Msg("Error persisting blacklist entry")
		JSONInternalServerError(rw)

		return
	}

	rw.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(rw).Encode(created); err != nil {
		span.RecordError(err)
		logger.Error().Err(err).Msg("Error sending blacklist response")
		JSONInternalServerError(rw)

		return
	}
}

// DeleteFromBlacklist removes a repository from the blacklist.
// The repository is passed as a query parameter: ?repository=owner/repo.
func (h Handlers) DeleteFromBlacklist(rw http.ResponseWriter, req *http.Request) {
	ctx, span := h.tracer.Start(req.Context(), "handler_delete_from_blacklist")
	defer span.End()

	rw.Header().Set("Content-Type", "application/json")

	repository := req.URL.Query().Get("repository")
	if repository == "" {
		JSONError(rw, http.StatusBadRequest, "missing repository query parameter")

		return
	}

	logger := log.With().Str("repository", repository).Logger()

	err := h.store.DeleteBlacklist(ctx, repository)
	if err != nil {
		span.RecordError(err)

		if errors.As(err, &db.NotFoundError{}) {
			NotFound(rw, req)

			return
		}

		logger.Error().Err(err).Msg("Failed to delete blacklist entry")
		JSONInternalServerError(rw)

		return
	}

	rw.WriteHeader(http.StatusNoContent)
}
