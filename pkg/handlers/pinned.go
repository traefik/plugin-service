package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"
	"golang.org/x/mod/module"
)

// errNotPinned the Go module proxy does not know which commit the version was published from.
var errNotPinned = errors.New("no pinned commit")

// archiveRef returns the ref to download the GitHub zipball of moduleName@version from.
//
// It is the tag while the tag still points at the commit pinned by the Go module proxy at publication,
// and the pinned commit once the tag moved or was deleted.
// Requesting the tag keeps the archive bytes: the zipball of an annotated tag is named after the tag object, not the commit.
// The pinned commit of a moved annotated tag cannot give those bytes back, nor match a hash recorded after the move:
// a moved tag whose archive still matches the recorded hash is served, since validate accepts exactly those bytes.
// Without an answer from the Go module proxy, it falls back to the tag, as before pinning.
// When GitHub no longer has the pinned commit, the moved tag is served only if a hash is recorded:
// validate then rejects any other bytes, and no hash is ever recorded from code the proxy did not pin.
// recordedHash is the hash recorded for this version, empty if none.
func (h Handlers) archiveRef(ctx context.Context, owner, repoName, moduleName, version, recordedHash string) (string, error) {
	if h.pinProxyURL == "" {
		return version, nil
	}

	pinned, err := h.pinnedCommit(ctx, moduleName, version)
	if errors.Is(err, errNotPinned) {
		return version, nil
	}

	if err != nil {
		log.Warn().Err(err).Str("module_name", moduleName).Str("module_version", version).
			Msg("Failed to get the pinned commit, serving the tag")

		return version, nil
	}

	// A conditional request answered 304 is not counted in the GitHub rate limit.
	sha, resp, err := h.gh.Repositories.GetCommitSHA1(ctx, owner, repoName, version, pinned)

	unchanged := (resp != nil && resp.StatusCode == http.StatusNotModified) || (err == nil && sha == pinned)
	moved := err == nil
	deleted := resp != nil && (resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusUnprocessableEntity)

	switch {
	case unchanged:
		return version, nil
	case moved:
		return h.refAfterTagChange(ctx, owner, repoName, moduleName, version, pinned, sha, recordedHash)
	case deleted:
		return h.refAfterTagChange(ctx, owner, repoName, moduleName, version, pinned, "", recordedHash)
	default:
		return "", fmt.Errorf("resolve tag %s: %w", version, err)
	}
}

// refAfterTagChange returns the ref to serve once the tag moved to tagCommit, or was deleted when tagCommit is empty.
func (h Handlers) refAfterTagChange(ctx context.Context, owner, repoName, moduleName, version, pinned, tagCommit, recordedHash string) (string, error) {
	logger := log.With().Str("module_name", moduleName).Str("module_version", version).Str("pinned_commit", pinned).Logger()

	if tagCommit != "" && recordedHash != "" {
		matches, err := h.tagMatchesRecorded(ctx, owner, repoName, version, tagCommit, recordedHash)
		if err != nil {
			logger.Warn().Err(err).Str("tag_commit", tagCommit).Msg("Failed to check the moved tag against the recorded hash, serving as if it does not match")
		}

		if matches {
			logger.Info().Str("tag_commit", tagCommit).Msg("Tag moved after publication, serving the tag: its archive matches the recorded hash")

			return version, nil
		}
	}

	gone, err := h.commitGone(ctx, owner, repoName, pinned)
	if err != nil {
		return "", err
	}

	switch {
	case !gone && tagCommit != "":
		logger.Info().Str("tag_commit", tagCommit).Msg("Tag moved after publication, serving the pinned commit")

		return pinned, nil
	case !gone:
		logger.Info().Msg("Tag not found, serving the pinned commit")

		return pinned, nil
	case tagCommit != "" && recordedHash != "":
		logger.Warn().Str("tag_commit", tagCommit).Msg("Pinned commit gone from GitHub, serving the moved tag, checked against the recorded hash")

		return version, nil
	default:
		return "", fmt.Errorf("pinned commit %s of %s@%s is gone from GitHub", pinned, moduleName, version)
	}
}

// tagMatchesRecorded reports whether the zipball of the tag version, pointing at tagCommit, has the recorded hash.
func (h Handlers) tagMatchesRecorded(ctx context.Context, owner, repoName, version, tagCommit, recordedHash string) (bool, error) {
	// Cache the hash, not the answer: the recorded hash can be realigned.
	// The zipball of an annotated tag names the tag object, so two tags on one commit differ.
	key := owner + "/" + repoName + "@" + version + "@" + tagCommit

	if cached, ok := h.tagHashes.Load(key); ok {
		if sum, ok := cached.(string); ok {
			return sum == recordedHash, nil
		}
	}

	req, err := h.zipballRequest(ctx, owner, repoName, version)
	if err != nil {
		return false, fmt.Errorf("tag %s: %w", version, err)
	}

	hash := sha256.New()

	if _, err = h.gh.Do(ctx, req, hash); err != nil {
		return false, fmt.Errorf("download tag %s: %w", version, err)
	}

	sum := fmt.Sprintf("%x", hash.Sum(nil))

	h.tagHashes.Store(key, sum)

	return sum == recordedHash, nil
}

// commitGone reports whether GitHub no longer has the commit sha.
func (h Handlers) commitGone(ctx context.Context, owner, repoName, sha string) (bool, error) {
	// A conditional request answered 304 is not counted in the GitHub rate limit.
	_, resp, err := h.gh.Repositories.GetCommitSHA1(ctx, owner, repoName, sha, sha)

	switch {
	case err == nil, resp != nil && resp.StatusCode == http.StatusNotModified:
		return false, nil
	case resp != nil && (resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusUnprocessableEntity):
		return true, nil
	default:
		return false, fmt.Errorf("check commit %s: %w", sha, err)
	}
}

// pinnedCommit returns the commit the Go module proxy pinned when moduleName@version was published.
func (h Handlers) pinnedCommit(ctx context.Context, moduleName, version string) (string, error) {
	key := moduleName + "@" + version

	if cached, ok := h.pins.Load(key); ok {
		if pinned, ok := cached.(string); ok {
			return pinned, nil
		}
	}

	pinned, err := h.fetchPinnedCommit(ctx, moduleName, version)
	if err != nil {
		return "", err
	}

	h.pins.Store(key, pinned)

	return pinned, nil
}

func (h Handlers) fetchPinnedCommit(ctx context.Context, moduleName, version string) (string, error) {
	escModule, err := module.EscapePath(moduleName)
	if err != nil {
		return "", err
	}

	escVersion, err := module.EscapeVersion(version)
	if err != nil {
		return "", err
	}

	endpoint := fmt.Sprintf("%s/%s/@v/%s.info", strings.TrimSuffix(h.pinProxyURL, "/"), escModule, escVersion)

	//nolint:gosec // G704: the host is the configured Go module proxy.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return "", err
	}

	resp, err := h.httpClient.Do(req) //nolint:gosec // G704: same request.
	if err != nil {
		return "", fmt.Errorf("get %s: %w", endpoint, err)
	}

	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound, http.StatusGone:
		return "", errNotPinned
	default:
		return "", fmt.Errorf("get %s: status %d", endpoint, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var info struct {
		Origin *struct {
			Hash string `json:"Hash"` //nolint:tagliatelle // Go module proxy protocol.
		} `json:"Origin"` //nolint:tagliatelle // Go module proxy protocol.
	}

	if err = json.Unmarshal(body, &info); err != nil {
		return "", fmt.Errorf("decode %s: %w", endpoint, err)
	}

	if info.Origin == nil || info.Origin.Hash == "" {
		return "", errNotPinned
	}

	return info.Origin.Hash, nil
}
