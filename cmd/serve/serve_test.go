package serve

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/traefik/plugin-service/pkg/db"
	"github.com/traefik/plugin-service/pkg/handlers"
)

// routeStore records the store method a route reaches.
// The embedded Storer is nil: any other method panics, and the router answers 500.
type routeStore struct {
	handlers.Storer

	called string
}

func (s *routeStore) List(context.Context, db.Pagination) ([]db.Plugin, string, error) {
	s.called = "List"
	return nil, "", nil
}

func (s *routeStore) Create(_ context.Context, plugin db.Plugin) (db.Plugin, error) {
	s.called = "Create " + plugin.Name
	return plugin, nil
}

func (s *routeStore) Update(_ context.Context, id string, plugin db.Plugin) (db.Plugin, error) {
	s.called = "Update " + id
	return plugin, nil
}

func (s *routeStore) Get(_ context.Context, id string) (db.Plugin, error) {
	return db.Plugin{ID: id}, nil
}

func (s *routeStore) Delete(_ context.Context, id string) error {
	s.called = "Delete " + id
	return nil
}

func (s *routeStore) ListStates(context.Context) ([]db.PluginState, error) {
	s.called = "ListStates"
	return []db.PluginState{}, nil
}

func (s *routeStore) Patch(_ context.Context, id string, _ db.PluginPatch) error {
	s.called = "Patch " + id
	return nil
}

func (s *routeStore) ListBlacklist(context.Context) ([]db.BlacklistEntry, error) {
	s.called = "ListBlacklist"
	return nil, nil
}

func (s *routeStore) UpsertBlacklist(_ context.Context, entry db.BlacklistEntry) (db.BlacklistEntry, error) {
	s.called = "UpsertBlacklist " + entry.Repository
	return entry, nil
}

func (s *routeStore) DeleteBlacklist(_ context.Context, repository string) error {
	s.called = "DeleteBlacklist " + repository
	return nil
}

func Test_buildInternalRouter(t *testing.T) {
	testCases := []struct {
		method   string
		target   string
		body     string
		wantCode int
		wantCall string
	}{
		{method: http.MethodGet, target: "/internal/", wantCode: http.StatusOK, wantCall: "List"},
		{method: http.MethodPost, target: "/internal/", body: `{"name":"plugin"}`, wantCode: http.StatusCreated, wantCall: "Create plugin"},
		{method: http.MethodGet, target: "/internal/states", wantCode: http.StatusOK, wantCall: "ListStates"},
		{method: http.MethodPut, target: "/internal/123", body: `{}`, wantCode: http.StatusOK, wantCall: "Update 123"},
		{method: http.MethodPatch, target: "/internal/123", body: `{"stars":1}`, wantCode: http.StatusNoContent, wantCall: "Patch 123"},
		{method: http.MethodDelete, target: "/internal/123", wantCode: http.StatusOK, wantCall: "Delete 123"},
		{method: http.MethodGet, target: "/internal/blacklist", wantCode: http.StatusOK, wantCall: "ListBlacklist"},
		{method: http.MethodPost, target: "/internal/blacklist", body: `{"repository":"owner/repo"}`, wantCode: http.StatusOK, wantCall: "UpsertBlacklist owner/repo"},
		{method: http.MethodDelete, target: "/internal/blacklist/owner/repo", wantCode: http.StatusNoContent, wantCall: "DeleteBlacklist owner/repo"},
		// No redirect to a path without the /internal prefix.
		{method: http.MethodGet, target: "/internal/states/", wantCode: http.StatusNotFound},
		{method: http.MethodPatch, target: "/internal/123/", body: `{"stars":1}`, wantCode: http.StatusNotFound},
		{method: http.MethodGet, target: "/internal/123", wantCode: http.StatusNotFound},
		{method: http.MethodPost, target: "/internal/", body: `{"name":"plugin","stars":-1}`, wantCode: http.StatusBadRequest},
	}

	for _, test := range testCases {
		t.Run(test.method+" "+test.target, func(t *testing.T) {
			t.Parallel()

			store := &routeStore{}

			rw := httptest.NewRecorder()
			req := httptest.NewRequest(test.method, test.target, strings.NewReader(test.body))

			buildInternalRouter(handlers.New(store, nil, nil, "")).ServeHTTP(rw, req)

			assert.Equal(t, test.wantCode, rw.Code, rw.Body.String())
			assert.Equal(t, test.wantCall, store.called)

			if test.wantCode == http.StatusNotFound {
				assert.JSONEq(t, `{"error":"Not Found"}`, rw.Body.String())
			}
		})
	}
}

func Test_buildInternalRouter_panic(t *testing.T) {
	store := &routeStore{}

	rw := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/internal/?name=plugin", http.NoBody)

	// GetByName is not implemented by routeStore: it panics.
	buildInternalRouter(handlers.New(store, nil, nil, "")).ServeHTTP(rw, req)

	assert.Equal(t, http.StatusInternalServerError, rw.Code)
	assert.JSONEq(t, `{"error":"panic"}`, rw.Body.String())
}
