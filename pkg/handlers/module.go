package handlers

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strings"

	"github.com/google/go-github/v57/github"
	"github.com/rs/zerolog/log"
	ttrace "github.com/traefik/hub-trace-kpi/trace"
	"github.com/traefik/plugin-service/pkg/db"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const (
	hashHeader = "X-Plugin-Hash"
)

// Download a plugin archive.
func (h Handlers) Download(rw http.ResponseWriter, req *http.Request) {
	ctx, span := h.tracer.Start(req.Context(), "handler_download")
	defer span.End()

	if req.Method != http.MethodGet {
		span.RecordError(fmt.Errorf("unsupported method: %s", req.Method))
		log.Error().Msgf("Unsupported method: %s", req.Method)
		JSONErrorf(rw, http.StatusMethodNotAllowed, "unsupported method: %s", req.Method)
		return
	}

	pluginName, version := extractPluginInfo(req.URL, "/download/")

	attributes := []attribute.KeyValue{
		attribute.String("module.moduleName", pluginName),
		attribute.String("module.version", version),
		attribute.String("module.ip", getUserIP(req)),
	}

	logger := log.With().Str("plugin_name", pluginName).Str("plugin_version", version).Logger()

	plugin, err := h.store.GetByName(ctx, pluginName, false)
	if err != nil {
		span.RecordError(err)
		if errors.As(err, &db.NotFoundError{}) {
			logger.Warn().Err(err).Msg("Unknown plugin")
			JSONErrorf(rw, http.StatusNotFound, "Unknown plugin: %s@%s", pluginName, version)
			return
		}

		logger.Error().Err(err).Msg("Failed to get plugin")
		JSONErrorf(rw, http.StatusInternalServerError, "Failed to get plugin %s@%s", pluginName, version)
		return
	}

	ttrace.Captured(span)

	sum := req.Header.Get(hashHeader)
	if sum != "" {
		attributes = append(attributes, attribute.String("module.sum", sum))
		ph, errH := h.store.GetHashByName(ctx, pluginName, version)
		if errH != nil {
			span.RecordError(errH)
			if !errors.As(errH, &db.NotFoundError{}) {
				logger.Error().Err(errH).Msg("Failed to get plugin hash")
				JSONErrorf(rw, http.StatusInternalServerError, "Failed to get plugin %s@%s", pluginName, version)
				return
			}
		} else if ph.Hash == sum {
			rw.WriteHeader(http.StatusNotModified)
			return
		}

		span.AddEvent("module.download", trace.WithAttributes(attributes...))
		logger.Error().Msgf("Someone is trying to hack the archive: %v", sum)
	}

	span.SetAttributes(attributes...)

	switch strings.ToLower(plugin.Runtime) {
	case "wasm":
		// WASM plugins
		if h.gh == nil {
			logger.Error().Msg("Failed to get plugin: missing GitHub client.")
			JSONErrorf(rw, http.StatusInternalServerError, "Failed to get plugin %s@%s", pluginName, version)
			return
		}

		h.downloadGitHub(ctx, pluginName, version, true)(rw, req)
		return

	default:
		// Yaegi plugins
		modFile, err := h.goProxy.GetModFile(pluginName, version)
		if err != nil {
			span.RecordError(err)
			logger.Error().Err(err).Msg("Failed to get module file")
			JSONErrorf(rw, http.StatusInternalServerError, "Failed to get plugin %s@%s", pluginName, version)
			return
		}

		// Uses GitHub when there are dependencies because Go proxy archives don't contain vendor folder.
		if h.gh != nil && len(modFile.Require) > 0 {
			h.downloadGitHub(ctx, pluginName, version, false)(rw, req)
			return
		}

		h.downloadGoProxy(ctx, pluginName, version)(rw, req)
	}
}

func getUserIP(req *http.Request) string {
	ip, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		ip = req.RemoteAddr
	}

	xff := req.Header.Get("X-Forwarded-For")
	if xff != "" {
		ip = strings.Trim(strings.Split(xff, ",")[0], " ")
	}
	return ip
}

func (h Handlers) downloadGoProxy(ctx context.Context, moduleName, version string) http.HandlerFunc {
	return func(rw http.ResponseWriter, _ *http.Request) {
		var span trace.Span
		ctx, span = h.tracer.Start(ctx, "handler_downloadGoProxy")
		defer span.End()

		logger := log.With().Str("module_name", moduleName).Str("module_version", version).Logger()

		sources, err := h.goProxy.DownloadSources(moduleName, version)
		if err != nil {
			span.RecordError(err)
			logger.Error().Err(err).Msg("Failed to download sources")
			JSONErrorf(rw, http.StatusInternalServerError, "Failed to get plugin %s@%s", moduleName, version)
			return
		}

		defer func() { _ = sources.Close() }()

		_, err = h.store.GetHashByName(ctx, moduleName, version)
		if err != nil && !errors.As(err, &db.NotFoundError{}) {
			span.RecordError(err)
			logger.Error().Err(err).Msg("Failed to get plugin hash")
			JSONErrorf(rw, http.StatusInternalServerError, "Failed to get plugin %s@%s", moduleName, version)
			return
		}

		if err == nil {
			_, err = io.Copy(rw, sources)
			if err != nil {
				span.RecordError(err)
				logger.Error().Err(err).Msg("Failed to write response body")
				JSONErrorf(rw, http.StatusInternalServerError, "Failed to get plugin %s@%s", moduleName, version)
				return
			}

			return
		}

		raw, err := io.ReadAll(sources)
		if err != nil {
			span.RecordError(err)
			logger.Error().Err(err).Msg("Failed to read response body")
			JSONErrorf(rw, http.StatusInternalServerError, "Failed to get plugin %s@%s", moduleName, version)
			return
		}

		hash := sha256.New()

		_, err = hash.Write(raw)
		if err != nil {
			span.RecordError(err)
			logger.Error().Err(err).Msg("Failed to compute hash")
			JSONErrorf(rw, http.StatusInternalServerError, "Failed to get plugin %s@%s", moduleName, version)
			return
		}

		sum := fmt.Sprintf("%x", hash.Sum(nil))

		_, err = h.store.CreateHash(ctx, moduleName, version, sum)
		if err != nil {
			span.RecordError(err)
			logger.Error().Err(err).Msg("Error persisting plugin hash")
			JSONErrorf(rw, http.StatusInternalServerError, "Could not persist data: %s@%s", moduleName, version)
			return
		}

		_, err = rw.Write(raw)
		if err != nil {
			span.RecordError(err)
			logger.Error().Err(err).Msg("failed to write response body")
			JSONErrorf(rw, http.StatusInternalServerError, "failed to get plugin %s@%s", moduleName, version)
			return
		}
	}
}

func (h Handlers) downloadGitHub(ctx context.Context, moduleName, version string, fromAssets bool) http.HandlerFunc {
	return func(rw http.ResponseWriter, _ *http.Request) {
		var span trace.Span
		ctx, span = h.tracer.Start(ctx, "handler_downloadGitHub")
		defer span.End()

		logger := log.With().Str("module_name", moduleName).Str("module_version", version).Logger()

		var request *http.Request
		var err error
		if fromAssets {
			request, err = h.getAssetLinkRequest(ctx, moduleName, version)
		} else {
			request, err = h.getArchiveLinkRequest(ctx, moduleName, version)
		}
		if err != nil {
			span.RecordError(err)
			logger.Error().Err(err).Msg("Failed to get archive link")
			JSONErrorf(rw, http.StatusInternalServerError, "Failed to get plugin %s@%s", moduleName, version)
			return
		}

		_, err = h.store.GetHashByName(ctx, moduleName, version)
		if err != nil && !errors.As(err, &db.NotFoundError{}) {
			span.RecordError(err)
			logger.Error().Err(err).Msg("Failed to get plugin hash")
			JSONErrorf(rw, http.StatusInternalServerError, "Failed to get plugin %s@%s", moduleName, version)
			return
		}

		if err == nil {
			_, err = h.gh.Do(ctx, request, rw)
			if err != nil {
				span.RecordError(err)
				logger.Error().Err(err).Msg("Failed to write response body")
				JSONErrorf(rw, http.StatusInternalServerError, "Failed to get plugin %s@%s", moduleName, version)
				return
			}

			return
		}

		sources := bytes.NewBufferString("")

		_, err = h.gh.Do(ctx, request, sources)
		if err != nil {
			span.RecordError(err)
			logger.Error().Err(err).Msg("Failed to get archive content")
			JSONErrorf(rw, http.StatusInternalServerError, "Failed to get plugin %s@%s", moduleName, version)
			return
		}

		raw, err := io.ReadAll(sources)
		if err != nil {
			span.RecordError(err)
			logger.Error().Err(err).Msg("Failed to read response body")
			JSONErrorf(rw, http.StatusInternalServerError, "Failed to get plugin %s@%s", moduleName, version)
			return
		}

		hash := sha256.New()

		_, err = hash.Write(raw)
		if err != nil {
			span.RecordError(err)
			logger.Error().Err(err).Msg("Failed to compute hash")
			JSONErrorf(rw, http.StatusInternalServerError, "Failed to get plugin %s@%s", moduleName, version)
			return
		}

		sum := fmt.Sprintf("%x", hash.Sum(nil))

		_, err = h.store.CreateHash(ctx, moduleName, version, sum)
		if err != nil {
			span.RecordError(err)
			logger.Error().Err(err).Msg("Error persisting plugin hash")
			JSONErrorf(rw, http.StatusInternalServerError, "Failed to get plugin %s@%s", moduleName, version)
			return
		}

		_, err = rw.Write(raw)
		if err != nil {
			span.RecordError(err)
			logger.Error().Err(err).Msg("Failed to write response body")
			JSONErrorf(rw, http.StatusInternalServerError, "Failed to get plugin %s@%s", moduleName, version)
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

	link, _, err := h.gh.Repositories.GetArchiveLink(ctx, owner, repoName, github.Zipball, opts, 3)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to get archive link: %w", err)
	}

	return http.NewRequestWithContext(ctx, http.MethodGet, link.String(), http.NoBody)
}

func (h Handlers) getAssetLinkRequest(ctx context.Context, moduleName, version string) (*http.Request, error) {
	ctx, span := h.tracer.Start(ctx, "handler_getAssetLinkRequest")
	defer span.End()

	owner, repoName := path.Split(strings.TrimPrefix(moduleName, "github.com/"))
	owner = strings.TrimSuffix(owner, "/")

	release, _, err := h.gh.Repositories.GetReleaseByTag(ctx, owner, repoName, version)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to get release: %w", err)
	}

	assets := map[*github.ReleaseAsset]struct{}{}
	for _, asset := range release.Assets {
		if filepath.Ext(asset.GetName()) == ".zip" {
			assets[asset] = struct{}{}
		}
	}

	if len(assets) > 1 {
		return nil, fmt.Errorf("too many zip archive (%d)", len(assets))
	}

	for asset := range assets {
		return http.NewRequestWithContext(ctx, http.MethodGet, asset.GetBrowserDownloadURL(), http.NoBody)
	}
	return nil, errors.New("zip archive not found")
}

// Validate validates a plugin archive.
func (h Handlers) Validate(rw http.ResponseWriter, req *http.Request) {
	ctx, span := h.tracer.Start(req.Context(), "handler_getArchiveLinkRequest")
	defer span.End()

	if req.Method != http.MethodGet {
		span.RecordError(fmt.Errorf("unsupported method: %s", req.Method))
		log.Warn().Msgf("Unsupported method: %s", req.Method)
		JSONErrorf(rw, http.StatusMethodNotAllowed, "Unsupported method: %s", req.Method)
		return
	}

	moduleName, version := extractPluginInfo(req.URL, "/validate/")

	logger := log.With().Str("module_name", moduleName).Str("module_version", version).Logger()

	headerSum := req.Header.Get(hashHeader)
	ph, err := h.store.GetHashByName(ctx, moduleName, version)
	if err != nil {
		if errors.As(err, &db.NotFoundError{}) {
			span.RecordError(fmt.Errorf("plugin not found %s@%s", moduleName, version))
			logger.Warn().Err(err).Msg("Plugin not found")
			JSONErrorf(rw, http.StatusNotFound, "Plugin not found %s@%s", moduleName, version)
			return
		}

		span.RecordError(err)
		logger.Error().Err(err).Msg("Error while fetch")
		JSONInternalServerError(rw)
		return
	}

	if ph.Hash == headerSum {
		rw.WriteHeader(http.StatusOK)
		return
	}

	rw.WriteHeader(http.StatusNotFound)
}

func extractPluginInfo(endpoint *url.URL, sep string) (string, string) {
	_, after, _ := strings.Cut(strings.TrimSuffix(endpoint.Path, "/"), sep)
	moduleName, version := path.Split(after)

	return cleanModuleName(moduleName), version
}

func cleanModuleName(moduleName string) string {
	return strings.TrimSuffix(strings.TrimPrefix(moduleName, "/"), "/")
}
