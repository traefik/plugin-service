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

	"github.com/rs/zerolog/log"

	"github.com/fauna/faunadb-go/faunadb"
	"github.com/google/go-github/v32/github"
)

const (
	tokenHeader = "X-Token"
	hashHeader  = "X-Plugin-Hash"
)

// Download a plugin archive.
func (h Handlers) Download(rw http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		log.Info().Msgf("unsupported method: %s", req.Method)
		jsonErrorf(rw, http.StatusMethodNotAllowed, "unsupported method: %s", req.Method)
		return
	}

	tokenValue := req.Header.Get(tokenHeader)
	if tokenValue == "" {
		log.Error().Msg("missing token")
		jsonError(rw, http.StatusBadRequest, "missing token")
		return
	}

	_, err := h.token.Check(tokenValue)
	if err != nil {
		log.Error().Err(err).Msg("invalid token")
		jsonError(rw, http.StatusBadRequest, "invalid token")
		return
	}

	moduleName, version := path.Split(strings.TrimPrefix(req.URL.Path, "/download/"))
	moduleName = cleanModuleName(moduleName)

	_, err = h.db.GetByName(moduleName)
	if err != nil {
		var notFoundError faunadb.NotFound
		if errors.As(err, &notFoundError) {
			log.Info().Msgf("unknown plugin: %s@%s", moduleName, version)
			jsonErrorf(rw, http.StatusNotFound, "unknown plugin: %s@%s", moduleName, version)
			return
		}

		log.Error().Err(err).Msgf("failed to get plugin (%s@%s)", moduleName, version)
		jsonError(rw, http.StatusInternalServerError, "failed to get plugin")
		return
	}

	sum := req.Header.Get(hashHeader)
	if sum != "" {
		ph, errH := h.db.GetHashByName(moduleName, version)
		if errH != nil {
			var notFoundError faunadb.NotFound
			if !errors.As(errH, &notFoundError) {
				log.Error().Msgf("failed to get plugin hash: %s@%s", moduleName, version)
				jsonError(rw, http.StatusInternalServerError, "failed to get plugin")
				return
			}
		} else if ph.Hash == sum {
			rw.WriteHeader(http.StatusNotModified)
			return
		}

		log.Error().Msgf("Someone is trying to hack the archive: %v %v %v", moduleName, version, sum)
	}

	modFile, err := h.goProxy.GetModFile(moduleName, version)
	if err != nil {
		log.Error().Msg(err.Error())
		return
	}

	if h.gh != nil && len(modFile.Require) > 0 {
		h.downloadGitHub(moduleName, version)(rw, req)
		return
	}

	h.downloadGoProxy(moduleName, version)(rw, req)
}

func (h Handlers) downloadGoProxy(moduleName, version string) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		sources, err := h.goProxy.DownloadSources(moduleName, version)
		if err != nil {
			log.Error().Err(err).Msg("failed to download sources")
			jsonError(rw, http.StatusInternalServerError, "failed to get plugin")
			return
		}

		defer func() { _ = sources.Close() }()

		_, err = h.db.GetHashByName(moduleName, version)
		var notFoundError faunadb.NotFound
		if err != nil && !errors.As(err, &notFoundError) {
			log.Error().Err(err).Msgf("failed to get plugin hash: %s@%s", moduleName, version)
			jsonError(rw, http.StatusInternalServerError, "failed to get plugin")
			return
		}

		if err == nil {
			_, err = io.Copy(rw, sources)
			if err != nil {
				log.Error().Err(err).Msgf("failed to write response body (%s@%s)", moduleName, version)
				jsonError(rw, http.StatusInternalServerError, "failed to get plugin")
				return
			}

			return
		}

		raw, err := ioutil.ReadAll(sources)
		if err != nil {
			log.Error().Err(err).Msgf("failed to read response body (%s@%s)", moduleName, version)
			jsonError(rw, http.StatusInternalServerError, "failed to get plugin")
			return
		}

		hash := sha256.New()

		_, err = hash.Write(raw)
		if err != nil {
			log.Error().Err(err).Msgf("failed to compute hash (%s@%s)", moduleName, version)
			jsonError(rw, http.StatusInternalServerError, "failed to get plugin")
			return
		}

		sum := fmt.Sprintf("%x", hash.Sum(nil))

		_, err = h.db.CreateHash(moduleName, version, sum)
		if err != nil {
			log.Error().Err(err).Msgf("Error persisting plugin hash %s@%s", moduleName, version)
			jsonError(rw, http.StatusInternalServerError, "could not persist data")
			return
		}

		_, err = rw.Write(raw)
		if err != nil {
			log.Error().Err(err).Msgf("failed to write response body (%s@%s)", moduleName, version)
			jsonError(rw, http.StatusInternalServerError, "failed to get plugin")
			return
		}
	}
}

func (h Handlers) downloadGitHub(moduleName, version string) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		ctx := context.Background()

		request, err := h.getArchiveLinkRequest(ctx, moduleName, version)
		if err != nil {
			log.Error().Err(err).Msgf("failed to get archive link (%s@%s)", moduleName, version)
			jsonError(rw, http.StatusInternalServerError, "failed to get plugin")
			return
		}

		_, err = h.db.GetHashByName(moduleName, version)
		var notFoundError faunadb.NotFound
		if err != nil && !errors.As(err, &notFoundError) {
			log.Error().Err(err).Msgf("failed to get plugin hash: %s@%s", moduleName, version)
			jsonError(rw, http.StatusInternalServerError, "failed to get plugin")
			return
		}

		if err == nil {
			_, err = h.gh.Do(ctx, request, rw)
			if err != nil {
				log.Error().Err(err).Msgf("failed to write response body (%s@%s)", moduleName, version)
				jsonError(rw, http.StatusInternalServerError, "failed to get plugin")
				return
			}

			return
		}

		sources := bytes.NewBufferString("")

		_, err = h.gh.Do(ctx, request, sources)
		if err != nil {
			log.Error().Err(err).Msgf("failed to get archive content (%s@%s)", moduleName, version)
			jsonError(rw, http.StatusInternalServerError, "failed to get plugin")
			return
		}

		raw, err := ioutil.ReadAll(sources)
		if err != nil {
			log.Error().Err(err).Msgf("failed to read response body (%s@%s)", moduleName, version)
			jsonError(rw, http.StatusInternalServerError, "failed to get plugin")
			return
		}

		hash := sha256.New()

		_, err = hash.Write(raw)
		if err != nil {
			log.Error().Err(err).Msgf("failed to compute hash (%s@%s)", moduleName, version)
			jsonError(rw, http.StatusInternalServerError, "failed to get plugin")
			return
		}

		sum := fmt.Sprintf("%x", hash.Sum(nil))

		_, err = h.db.CreateHash(moduleName, version, sum)
		if err != nil {
			log.Error().Err(err).Msgf("Error persisting plugin hash %s@%s", moduleName, version)
			jsonError(rw, http.StatusInternalServerError, "could not persist data")
			return
		}

		_, err = rw.Write(raw)
		if err != nil {
			log.Error().Err(err).Msgf("failed to write response body (%s@%s)", moduleName, version)
			jsonError(rw, http.StatusInternalServerError, "failed to get plugin")
			return
		}
	}
}

func (h Handlers) getArchiveLinkRequest(ctx context.Context, moduleName, version string) (*http.Request, error) {
	opts := &github.RepositoryContentGetOptions{Ref: version}

	owner, repoName := path.Split(strings.TrimPrefix(moduleName, "github.com/"))
	owner = strings.TrimSuffix(owner, "/")

	link, _, err := h.gh.Repositories.GetArchiveLink(ctx, owner, repoName, github.Zipball, opts, true)
	if err != nil {
		return nil, fmt.Errorf("failed to get archive link: %w", err)
	}

	return http.NewRequest(http.MethodGet, link.String(), nil)
}

// Validate validates a plugin archive.
func (h Handlers) Validate(rw http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		log.Info().Msgf("unsupported method: %s", req.Method)
		jsonErrorf(rw, http.StatusMethodNotAllowed, "unsupported method: %s", req.Method)
		return
	}

	tokenValue := req.Header.Get(tokenHeader)
	if tokenValue == "" {
		log.Error().Msg("missing token")
		jsonError(rw, http.StatusBadRequest, "missing token")
		return
	}

	_, err := h.token.Check(tokenValue)
	if err != nil {
		log.Error().Err(err).Msgf("invalid token")
		jsonError(rw, http.StatusBadRequest, "invalid token")
		return
	}

	moduleName, version := path.Split(strings.TrimPrefix(req.URL.Path, "/validate/"))
	moduleName = strings.TrimSuffix(strings.TrimPrefix(moduleName, "/"), "/")

	headerSum := req.Header.Get(hashHeader)
	ph, err := h.db.GetHashByName(moduleName, version)
	if err != nil {
		var notFoundError faunadb.NotFound
		if errors.As(err, &notFoundError) {
			log.Info().Msgf("plugin not found: %s@%s", moduleName, version)
			jsonError(rw, http.StatusNotFound, "plugin not found")
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
