package handlers

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"

	"github.com/google/go-github/v74/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArchiveRef(t *testing.T) {
	proxy := http.NewServeMux()
	proxy.HandleFunc("/github.com/o/unknown/@v/v1.0.0.info", func(rw http.ResponseWriter, _ *http.Request) {
		rw.WriteHeader(http.StatusNotFound)
	})
	proxy.HandleFunc("/github.com/o/noorigin/@v/v1.0.0.info", func(rw http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(rw, `{"Version":"v1.0.0"}`)
	})
	proxy.HandleFunc("/github.com/o/proxydown/@v/v1.0.0.info", func(rw http.ResponseWriter, _ *http.Request) {
		rw.WriteHeader(http.StatusBadGateway)
	})
	proxy.HandleFunc("/github.com/o/proxygone/@v/v1.0.0.info", func(rw http.ResponseWriter, _ *http.Request) {
		rw.WriteHeader(http.StatusGone)
	})

	for _, repo := range []string{"stable", "moved", "deleted", "notfound", "ghdown", "movedgone", "deletedgone", "checkdown", "movedmatch", "archivedown", "archivedowngone"} {
		proxy.HandleFunc("/github.com/o/"+repo+"/@v/v1.0.0.info", func(rw http.ResponseWriter, _ *http.Request) {
			_, _ = fmt.Fprint(rw, `{"Version":"v1.0.0","Origin":{"VCS":"git","Hash":"pinned"}}`)
		})
	}

	proxySrv := httptest.NewServer(proxy)
	t.Cleanup(proxySrv.Close)

	// Answers like GitHub: 304 when the tag still points at the If-None-Match commit.
	commit := func(sha string) http.HandlerFunc {
		return func(rw http.ResponseWriter, req *http.Request) {
			if req.Header.Get("If-None-Match") == `"`+sha+`"` {
				rw.WriteHeader(http.StatusNotModified)
				return
			}

			_, _ = fmt.Fprint(rw, sha)
		}
	}

	status := func(code int) http.HandlerFunc {
		return func(rw http.ResponseWriter, _ *http.Request) {
			rw.WriteHeader(code)
		}
	}

	// The URL of the fake GitHub, set once it starts: the zipball endpoint redirects to an absolute URL.
	var ghURL string

	gh := http.NewServeMux()
	gh.HandleFunc("/repos/o/stable/commits/v1.0.0", commit("pinned"))
	gh.HandleFunc("/repos/o/moved/commits/v1.0.0", commit("other"))
	gh.HandleFunc("/repos/o/deleted/commits/v1.0.0", status(http.StatusUnprocessableEntity))
	gh.HandleFunc("/repos/o/notfound/commits/v1.0.0", status(http.StatusNotFound))
	gh.HandleFunc("/repos/o/ghdown/commits/v1.0.0", status(http.StatusServiceUnavailable))
	gh.HandleFunc("/repos/o/movedgone/commits/v1.0.0", commit("other"))
	gh.HandleFunc("/repos/o/deletedgone/commits/v1.0.0", status(http.StatusUnprocessableEntity))
	gh.HandleFunc("/repos/o/checkdown/commits/v1.0.0", commit("other"))
	gh.HandleFunc("/repos/o/movedmatch/commits/v1.0.0", commit("other"))
	gh.HandleFunc("/repos/o/archivedown/commits/v1.0.0", commit("other"))
	gh.HandleFunc("/repos/o/archivedowngone/commits/v1.0.0", commit("other"))

	// The pinned commit, still on GitHub or gone.
	for _, repo := range []string{"moved", "deleted", "notfound", "movedmatch", "archivedown"} {
		gh.HandleFunc("/repos/o/"+repo+"/commits/pinned", commit("pinned"))
	}

	// The zipball of the moved tags.
	for _, repo := range []string{"moved", "movedgone", "checkdown", "movedmatch"} {
		gh.HandleFunc("/repos/o/"+repo+"/zipball/v1.0.0", func(rw http.ResponseWriter, _ *http.Request) {
			rw.Header().Set("Location", ghURL+"/archives/"+repo)
			rw.WriteHeader(http.StatusFound)
		})
	}

	gh.HandleFunc("/archives/", func(rw http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(rw, "tag archive")
	})
	gh.HandleFunc("/repos/o/archivedown/zipball/v1.0.0", status(http.StatusServiceUnavailable))
	gh.HandleFunc("/repos/o/archivedowngone/zipball/v1.0.0", status(http.StatusServiceUnavailable))

	gh.HandleFunc("/repos/o/movedgone/commits/pinned", status(http.StatusUnprocessableEntity))
	gh.HandleFunc("/repos/o/archivedowngone/commits/pinned", status(http.StatusUnprocessableEntity))
	gh.HandleFunc("/repos/o/deletedgone/commits/pinned", status(http.StatusUnprocessableEntity))
	gh.HandleFunc("/repos/o/checkdown/commits/pinned", status(http.StatusServiceUnavailable))

	ghSrv := httptest.NewServer(gh)
	t.Cleanup(ghSrv.Close)

	ghURL = ghSrv.URL

	ghClient := github.NewClient(nil)
	ghClient.BaseURL, _ = url.Parse(ghSrv.URL + "/")

	h := New(nil, nil, ghClient, proxySrv.URL)

	tagArchive := fmt.Sprintf("%x", sha256.Sum256([]byte("tag archive")))

	testCases := []struct {
		desc     string
		repo     string
		recorded string
		ref      string
		wantErr  bool
	}{
		{repo: "unknown", ref: "v1.0.0"},
		{repo: "noorigin", ref: "v1.0.0"},
		{repo: "stable", ref: "v1.0.0"},
		{repo: "moved", ref: "pinned"},
		{repo: "deleted", ref: "pinned"},
		{repo: "notfound", ref: "pinned"},
		{repo: "ghdown", wantErr: true},
		{repo: "proxydown", ref: "v1.0.0"},
		{repo: "proxygone", ref: "v1.0.0"},
		{repo: "Bad!Path", ref: "v1.0.0"},
		// The pinned commit, the recorded hash is from other bytes.
		{desc: "moved recorded", repo: "moved", recorded: "other", ref: "pinned"},
		// validate accepts exactly the bytes of the moved tag.
		{desc: "moved recorded from the tag", repo: "movedmatch", recorded: tagArchive, ref: "v1.0.0"},
		{desc: "moved not recorded", repo: "movedmatch", ref: "pinned"},
		// The check failed: served as if the tag does not match.
		{desc: "tag archive down", repo: "archivedown", recorded: tagArchive, ref: "pinned"},
		{desc: "tag archive down, pinned commit gone", repo: "archivedowngone", recorded: tagArchive, ref: "v1.0.0"},
		// validate checks the moved tag against the recorded hash.
		{desc: "movedgone recorded", repo: "movedgone", recorded: "other", ref: "v1.0.0"},
		// Serving the moved tag would record a hash from code the proxy did not pin.
		{desc: "movedgone not recorded", repo: "movedgone", wantErr: true},
		{desc: "deletedgone recorded", repo: "deletedgone", recorded: "other", wantErr: true},
		{repo: "checkdown", recorded: "other", wantErr: true},
	}

	for _, test := range testCases {
		desc := test.desc
		if desc == "" {
			desc = test.repo
		}

		t.Run(desc, func(t *testing.T) {
			ref, err := h.archiveRef(t.Context(), "o", test.repo, "github.com/o/"+test.repo, "v1.0.0", test.recorded)

			if test.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.ref, ref)
		})
	}
}

func TestPinnedCommit_cached(t *testing.T) {
	var calls atomic.Int32

	proxySrv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		_, _ = fmt.Fprint(rw, `{"Version":"v1.0.0","Origin":{"VCS":"git","Hash":"pinned"}}`)
	}))
	t.Cleanup(proxySrv.Close)

	h := New(nil, nil, nil, proxySrv.URL)

	for range 2 {
		pinned, err := h.pinnedCommit(t.Context(), "github.com/o/r", "v1.0.0")
		require.NoError(t, err)
		assert.Equal(t, "pinned", pinned)
	}

	assert.Equal(t, int32(1), calls.Load())
}

func TestArchiveRef_noPinProxy(t *testing.T) {
	h := New(nil, nil, github.NewClient(nil), "")

	ref, err := h.archiveRef(t.Context(), "o", "moved", "github.com/o/moved", "v1.0.0", "")
	require.NoError(t, err)

	assert.Equal(t, "v1.0.0", ref)
}

func TestTagMatchesRecorded_cached(t *testing.T) {
	var downloads atomic.Int32

	var ghURL string

	gh := http.NewServeMux()
	gh.HandleFunc("/repos/o/r/zipball/v1.0.0", func(rw http.ResponseWriter, _ *http.Request) {
		rw.Header().Set("Location", ghURL+"/archive")
		rw.WriteHeader(http.StatusFound)
	})
	gh.HandleFunc("/repos/o/r/zipball/v1.0.1", func(rw http.ResponseWriter, _ *http.Request) {
		rw.Header().Set("Location", ghURL+"/archive-v1.0.1")
		rw.WriteHeader(http.StatusFound)
	})
	gh.HandleFunc("/archive", func(rw http.ResponseWriter, _ *http.Request) {
		downloads.Add(1)
		_, _ = fmt.Fprint(rw, "tag archive")
	})
	// An annotated tag on the same commit: its zipball names another tag object.
	gh.HandleFunc("/archive-v1.0.1", func(rw http.ResponseWriter, _ *http.Request) {
		downloads.Add(1)
		_, _ = fmt.Fprint(rw, "other tag archive")
	})

	ghSrv := httptest.NewServer(gh)
	t.Cleanup(ghSrv.Close)

	ghURL = ghSrv.URL

	ghClient := github.NewClient(nil)
	ghClient.BaseURL, _ = url.Parse(ghSrv.URL + "/")

	h := New(nil, nil, ghClient, "")

	tagArchive := fmt.Sprintf("%x", sha256.Sum256([]byte("tag archive")))

	matches, err := h.tagMatchesRecorded(t.Context(), "o", "r", "v1.0.0", "tagcommit", tagArchive)
	require.NoError(t, err)
	assert.True(t, matches)

	// A realigned hash no longer matches the cached archive.
	matches, err = h.tagMatchesRecorded(t.Context(), "o", "r", "v1.0.0", "tagcommit", "realigned")
	require.NoError(t, err)
	assert.False(t, matches)

	matches, err = h.tagMatchesRecorded(t.Context(), "o", "r", "v1.0.1", "tagcommit", fmt.Sprintf("%x", sha256.Sum256([]byte("other tag archive"))))
	require.NoError(t, err)
	assert.True(t, matches)

	assert.Equal(t, int32(2), downloads.Load())
}
