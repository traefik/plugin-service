package handlers

import (
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

	for _, repo := range []string{"stable", "moved", "deleted", "notfound", "ghdown", "movedgone", "deletedgone", "checkdown"} {
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

	gh := http.NewServeMux()
	gh.HandleFunc("/repos/o/stable/commits/v1.0.0", commit("pinned"))
	gh.HandleFunc("/repos/o/moved/commits/v1.0.0", commit("other"))
	gh.HandleFunc("/repos/o/deleted/commits/v1.0.0", status(http.StatusUnprocessableEntity))
	gh.HandleFunc("/repos/o/notfound/commits/v1.0.0", status(http.StatusNotFound))
	gh.HandleFunc("/repos/o/ghdown/commits/v1.0.0", status(http.StatusServiceUnavailable))
	gh.HandleFunc("/repos/o/movedgone/commits/v1.0.0", commit("other"))
	gh.HandleFunc("/repos/o/deletedgone/commits/v1.0.0", status(http.StatusUnprocessableEntity))
	gh.HandleFunc("/repos/o/checkdown/commits/v1.0.0", commit("other"))

	// The pinned commit, still on GitHub or gone.
	for _, repo := range []string{"moved", "deleted", "notfound"} {
		gh.HandleFunc("/repos/o/"+repo+"/commits/pinned", commit("pinned"))
	}

	gh.HandleFunc("/repos/o/movedgone/commits/pinned", status(http.StatusUnprocessableEntity))
	gh.HandleFunc("/repos/o/deletedgone/commits/pinned", status(http.StatusUnprocessableEntity))
	gh.HandleFunc("/repos/o/checkdown/commits/pinned", status(http.StatusServiceUnavailable))

	ghSrv := httptest.NewServer(gh)
	t.Cleanup(ghSrv.Close)

	ghClient := github.NewClient(nil)
	ghClient.BaseURL, _ = url.Parse(ghSrv.URL + "/")

	h := New(nil, nil, ghClient, proxySrv.URL)

	testCases := []struct {
		desc     string
		repo     string
		recorded bool
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
		// validate checks the moved tag against the recorded hash.
		{desc: "movedgone recorded", repo: "movedgone", recorded: true, ref: "v1.0.0"},
		// Serving the moved tag would record a hash from code the proxy did not pin.
		{desc: "movedgone not recorded", repo: "movedgone", wantErr: true},
		{desc: "deletedgone recorded", repo: "deletedgone", recorded: true, wantErr: true},
		{repo: "checkdown", recorded: true, wantErr: true},
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

	ref, err := h.archiveRef(t.Context(), "o", "moved", "github.com/o/moved", "v1.0.0", false)
	require.NoError(t, err)

	assert.Equal(t, "v1.0.0", ref)
}
