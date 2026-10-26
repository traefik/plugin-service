package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
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

	for _, repo := range []string{"stable", "moved", "deleted", "ghdown"} {
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

	gh := http.NewServeMux()
	gh.HandleFunc("/repos/o/stable/commits/v1.0.0", commit("pinned"))
	gh.HandleFunc("/repos/o/moved/commits/v1.0.0", commit("other"))
	gh.HandleFunc("/repos/o/deleted/commits/v1.0.0", func(rw http.ResponseWriter, _ *http.Request) {
		rw.WriteHeader(http.StatusUnprocessableEntity)
	})
	gh.HandleFunc("/repos/o/ghdown/commits/v1.0.0", func(rw http.ResponseWriter, _ *http.Request) {
		rw.WriteHeader(http.StatusServiceUnavailable)
	})

	ghSrv := httptest.NewServer(gh)
	t.Cleanup(ghSrv.Close)

	ghClient := github.NewClient(nil)
	ghClient.BaseURL, _ = url.Parse(ghSrv.URL + "/")

	h := New(nil, nil, ghClient, proxySrv.URL)

	testCases := []struct {
		repo    string
		ref     string
		wantErr bool
	}{
		{repo: "unknown", ref: "v1.0.0"},
		{repo: "noorigin", ref: "v1.0.0"},
		{repo: "stable", ref: "v1.0.0"},
		{repo: "moved", ref: "pinned"},
		{repo: "deleted", ref: "pinned"},
		{repo: "ghdown", wantErr: true},
		{repo: "proxydown", ref: "v1.0.0"},
		{repo: "Bad!Path", ref: "v1.0.0"},
	}

	for _, test := range testCases {
		t.Run(test.repo, func(t *testing.T) {
			ref, err := h.archiveRef(t.Context(), "o", test.repo, "github.com/o/"+test.repo, "v1.0.0")

			if test.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.ref, ref)
		})
	}
}

func TestArchiveRef_noPinProxy(t *testing.T) {
	h := New(nil, nil, github.NewClient(nil), "")

	ref, err := h.archiveRef(t.Context(), "o", "moved", "github.com/o/moved", "v1.0.0")
	require.NoError(t, err)

	assert.Equal(t, "v1.0.0", ref)
}
