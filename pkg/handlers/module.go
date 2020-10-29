package handlers

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"path"
	"strings"

	"github.com/fauna/faunadb-go/v3/faunadb"
	"github.com/google/go-github/v32/github"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel/label"
)

const (
	tokenHeader = "X-Token"
	hashHeader  = "X-Plugin-Hash"
)

// Download a plugin archive.
func (h Handlers) Download(rw http.ResponseWriter, req *http.Request) {
	ctx, span := h.tracer.Start(req.Context(), "handler_download")
	defer span.End()

	if req.Method != http.MethodGet {
		span.RecordError(ctx, fmt.Errorf("unsupported method: %s", req.Method))
		log.Error().Msgf("unsupported method: %s", req.Method)
		jsonErrorf(rw, http.StatusMethodNotAllowed, "unsupported method: %s", req.Method)
		return
	}

	moduleName, version := path.Split(strings.TrimPrefix(req.URL.Path, "/download/"))
	moduleName = cleanModuleName(moduleName)

	logger := log.With().Str("moduleName", moduleName).Str("moduleVersion", version).Logger()

	tokenValue := req.Header.Get(tokenHeader)
	if tokenValue == "" {
		span.RecordError(ctx, errors.New("missing token"))
		logger.Error().Msg("missing token")
		jsonError(rw, http.StatusBadRequest, "missing token")
		return
	}

	_, err := h.token.Check(ctx, tokenValue)
	if err != nil {
		span.RecordError(ctx, err)
		logger.Error().Err(err).Msg("invalid token")
		jsonError(rw, http.StatusBadRequest, "invalid token")
		return
	}

	_, err = h.db.GetByName(ctx, moduleName)
	if err != nil {
		span.RecordError(ctx, err)
		var notFoundError faunadb.NotFound
		if errors.As(err, &notFoundError) {
			logger.Error().Msg("unknown plugin")
			jsonErrorf(rw, http.StatusNotFound, "unknown plugin: %s@%s", moduleName, version)
			return
		}

		logger.Error().Err(err).Msg("failed to get plugin")
		jsonErrorf(rw, http.StatusInternalServerError, "failed to get plugin %s@%s", moduleName, version)
		return
	}

	sum := req.Header.Get(hashHeader)
	if sum != "" {
		ph, errH := h.db.GetHashByName(ctx, moduleName, version)
		if errH != nil {
			span.RecordError(ctx, errH)
			var notFoundError faunadb.NotFound
			if !errors.As(errH, &notFoundError) {
				logger.Error().Err(errH).Msg("failed to get plugin hash")
				jsonErrorf(rw, http.StatusInternalServerError, "failed to get plugin %s@%s", moduleName, version)
				return
			}
		} else if ph.Hash == sum {
			rw.WriteHeader(http.StatusNotModified)
			return
		}
		attributes := []label.KeyValue{
			{Key: label.Key("module.tokenValue"), Value: label.StringValue(tokenValue)},
			{Key: label.Key("module.moduleName"), Value: label.StringValue(moduleName)},
			{Key: label.Key("module.version"), Value: label.StringValue(version)},
			{Key: label.Key("module.sum"), Value: label.StringValue(sum)},
		}
		span.AddEvent(ctx, "module.download", attributes...)
		logger.Error().Msgf("Someone is trying to hack the archive: %v", sum)
	}

	modFile, err := h.goProxy.GetModFile(moduleName, version)
	if err != nil {
		span.RecordError(ctx, err)
		logger.Error().Err(err).Msg("Failed to get module file")
		return
	}

	if h.gh != nil && len(modFile.Require) > 0 {
		h.downloadGitHub(ctx, moduleName, version)(rw, req)
		return
	}

	h.downloadGoProxy(ctx, moduleName, version)(rw, req)
}

func (h Handlers) downloadGoProxy(ctx context.Context, moduleName, version string) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		ctx, span := h.tracer.Start(ctx, "handler_downloadGoProxy")
		defer span.End()

		logger := log.With().Str("moduleName", moduleName).Str("moduleVersion", version).Logger()

		sources, err := h.goProxy.DownloadSources(moduleName, version)
		if err != nil {
			span.RecordError(ctx, err)
			logger.Error().Err(err).Msg("failed to download sources")
			jsonErrorf(rw, http.StatusInternalServerError, "failed to get plugin %s@%s", moduleName, version)
			return
		}

		defer func() { _ = sources.Close() }()

		_, err = h.db.GetHashByName(ctx, moduleName, version)
		var notFoundError faunadb.NotFound
		if err != nil && !errors.As(err, &notFoundError) {
			span.RecordError(ctx, err)
			logger.Error().Err(err).Msg("failed to get plugin hash")
			jsonErrorf(rw, http.StatusInternalServerError, "failed to get plugin %s@%s", moduleName, version)
			return
		}

		if err == nil {
			_, err = io.Copy(rw, sources)
			if err != nil {
				span.RecordError(ctx, err)
				logger.Error().Err(err).Msg("failed to write response body")
				jsonErrorf(rw, http.StatusInternalServerError, "failed to get plugin %s@%s", moduleName, version)
				return
			}

			return
		}

		raw, err := ioutil.ReadAll(sources)
		if err != nil {
			span.RecordError(ctx, err)
			logger.Error().Err(err).Msg("failed to read response body")
			jsonErrorf(rw, http.StatusInternalServerError, "failed to get plugin %s@%s", moduleName, version)
			return
		}

		hash := sha256.New()

		_, err = hash.Write(raw)
		if err != nil {
			span.RecordError(ctx, err)
			logger.Error().Err(err).Msg("failed to compute hash")
			jsonErrorf(rw, http.StatusInternalServerError, "failed to get plugin %s@%s", moduleName, version)
			return
		}

		sum := fmt.Sprintf("%x", hash.Sum(nil))

		_, err = h.db.CreateHash(ctx, moduleName, version, sum)
		if err != nil {
			span.RecordError(ctx, err)
			logger.Error().Err(err).Msg("Error persisting plugin hash")
			jsonErrorf(rw, http.StatusInternalServerError, "could not persist data: %s@%s", moduleName, version)
			return
		}

		_, err = rw.Write(raw)
		if err != nil {
			span.RecordError(ctx, err)
			logger.Error().Err(err).Msg("failed to write response body")
			jsonErrorf(rw, http.StatusInternalServerError, "failed to get plugin %s@%s", moduleName, version)
			return
		}
	}
}

func (h Handlers) downloadGitHub(ctx context.Context, moduleName, version string) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		ctx, span := h.tracer.Start(ctx, "handler_downloadGitHub")
		defer span.End()

		logger := log.Error().Str("moduleName", moduleName).Str("moduleVersion", version)

		request, err := h.getArchiveLinkRequest(ctx, moduleName, version)
		if err != nil {
			span.RecordError(ctx, err)
			logger.Err(err).Msg("failed to get archive link")
			jsonErrorf(rw, http.StatusInternalServerError, "failed to get plugin %s@%s", moduleName, version)
			return
		}

		_, err = h.db.GetHashByName(ctx, moduleName, version)
		var notFoundError faunadb.NotFound
		if err != nil && !errors.As(err, &notFoundError) {
			span.RecordError(ctx, err)
			logger.Err(err).Msg("failed to get plugin hash")
			jsonErrorf(rw, http.StatusInternalServerError, "failed to get plugin %s@%s", moduleName, version)
			return
		}

		if err == nil {
			_, err = h.gh.Do(ctx, request, rw)
			if err != nil {
				span.RecordError(ctx, err)
				logger.Err(err).Msg("failed to write response body")
				jsonErrorf(rw, http.StatusInternalServerError, "failed to get plugin %s@%s", moduleName, version)
				return
			}

			return
		}

		sources := bytes.NewBufferString("")

		_, err = h.gh.Do(ctx, request, sources)
		if err != nil {
			span.RecordError(ctx, err)
			logger.Err(err).Msg("failed to get archive content")
			jsonErrorf(rw, http.StatusInternalServerError, "failed to get plugin %s@%s", moduleName, version)
			return
		}

		raw, err := ioutil.ReadAll(sources)
		if err != nil {
			span.RecordError(ctx, err)
			logger.Err(err).Msg("failed to read response body")
			jsonErrorf(rw, http.StatusInternalServerError, "failed to get plugin %s@%s", moduleName, version)
			return
		}

		hash := sha256.New()

		_, err = hash.Write(raw)
		if err != nil {
			span.RecordError(ctx, err)
			logger.Err(err).Msg("failed to compute hash")
			jsonErrorf(rw, http.StatusInternalServerError, "failed to get plugin %s@%s", moduleName, version)
			return
		}

		sum := fmt.Sprintf("%x", hash.Sum(nil))

		_, err = h.db.CreateHash(ctx, moduleName, version, sum)
		if err != nil {
			span.RecordError(ctx, err)
			logger.Err(err).Msg("Error persisting plugin hash")
			jsonErrorf(rw, http.StatusInternalServerError, "failed to get plugin %s@%s", moduleName, version)
			return
		}

		_, err = rw.Write(raw)
		if err != nil {
			span.RecordError(ctx, err)
			logger.Err(err).Msg("failed to write response body")
			jsonErrorf(rw, http.StatusInternalServerError, "failed to get plugin %s@%s", moduleName, version)
			return
		}
	}
}

func (h Handlers) getArchiveLinkRequest(ctx context.Context, moduleName, version string) (*http.Request, error) {
	ctx, span := h.tracer.Start(ctx, "handler_getArchiveLinkRequest")
	defer span.End()

	opts := &github.RepositoryContentGetOptions{Ref: version}

	owner, repoName := path.Split(strings.TrimPrefix(moduleName, "github.com/"))
	owner = strings.TrimSuffix(owner, "/")

	link, _, err := h.gh.Repositories.GetArchiveLink(ctx, owner, repoName, github.Zipball, opts, true)
	if err != nil {
		span.RecordError(ctx, err)
		return nil, fmt.Errorf("failed to get archive link: %w", err)
	}

	return http.NewRequestWithContext(ctx, http.MethodGet, link.String(), nil)
}

// Validate validates a plugin archive.
func (h Handlers) Validate(rw http.ResponseWriter, req *http.Request) {
	ctx, span := h.tracer.Start(req.Context(), "handler_getArchiveLinkRequest")
	defer span.End()

	if req.Method != http.MethodGet {
		span.RecordError(ctx, fmt.Errorf("unsupported method: %s", req.Method))
		log.Error().Msgf("unsupported method: %s", req.Method)
		jsonErrorf(rw, http.StatusMethodNotAllowed, "unsupported method: %s", req.Method)
		return
	}

	moduleName, version := path.Split(strings.TrimPrefix(req.URL.Path, "/validate/"))
	moduleName = cleanModuleName(moduleName)

	logger := log.With().Str("moduleName", moduleName).Str("moduleVersion", version).Logger()

	tokenValue := req.Header.Get(tokenHeader)
	if tokenValue == "" {
		span.RecordError(ctx, errors.New("missing token"))
		logger.Error().Msg("missing token")
		jsonError(rw, http.StatusBadRequest, "missing token")
		return
	}

	_, err := h.token.Check(context.Background(), tokenValue)
	if err != nil {
		span.RecordError(ctx, errors.New("invalid token"))
		logger.Error().Err(err).Msg("invalid token")
		jsonError(rw, http.StatusBadRequest, "invalid token")
		return
	}

	headerSum := req.Header.Get(hashHeader)
	ph, err := h.db.GetHashByName(context.Background(), moduleName, version)
	if err != nil {
		var notFoundError faunadb.NotFound
		if errors.As(err, &notFoundError) {
			span.RecordError(ctx, fmt.Errorf("plugin not found %s@%s", moduleName, version))
			logger.Error().Msg("plugin not found")
			jsonErrorf(rw, http.StatusNotFound, "plugin not found %s@%s", moduleName, version)
			return
		}

		jsonError(rw, http.StatusInternalServerError, err.Error())
		return
	}

	if ph.Hash == headerSum {
		rw.WriteHeader(http.StatusOK)
		return
	}

	rw.WriteHeader(http.StatusNotFound)
}

func cleanModuleName(moduleName string) string {
	return strings.TrimSuffix(strings.TrimPrefix(moduleName, "/"), "/")
}
