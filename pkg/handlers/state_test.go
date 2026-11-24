package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/traefik/plugin-service/pkg/db"
)

func TestHandlers_ListStates(t *testing.T) {
	testDB := mockDB{
		listStatesFn: func(_ context.Context) ([]db.PluginState, error) {
			return []db.PluginState{
				{ID: "123", Name: "github.com/traefik/plugindemo", LatestVersion: "v0.2.1", Stars: 22, TagsETag: `W/"abc"`},
				{ID: "456", Name: "github.com/traefik/hidden", LatestVersion: "v1.0.0", Hidden: true, Disabled: true},
			}, nil
		},
	}

	rw := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/internal/states", http.NoBody)

	New(testDB, nil, nil, "").ListStates(rw, req)

	require.Equal(t, http.StatusOK, rw.Code)
	assert.JSONEq(t, `[
		{"id":"123","name":"github.com/traefik/plugindemo","latestVersion":"v0.2.1","stars":22,"hidden":false,"disabled":false,"tagsETag":"W/\"abc\""},
		{"id":"456","name":"github.com/traefik/hidden","latestVersion":"v1.0.0","stars":0,"hidden":true,"disabled":true,"tagsETag":""}
	]`, rw.Body.String())
}

func TestHandlers_ListStates_empty(t *testing.T) {
	testDB := mockDB{
		listStatesFn: func(_ context.Context) ([]db.PluginState, error) {
			return []db.PluginState{}, nil
		},
	}

	rw := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/internal/states", http.NoBody)

	New(testDB, nil, nil, "").ListStates(rw, req)

	require.Equal(t, http.StatusOK, rw.Code)
	assert.JSONEq(t, `[]`, rw.Body.String())
}

func TestHandlers_Patch(t *testing.T) {
	testCases := []struct {
		desc      string
		body      string
		wantPatch db.PluginPatch
	}{
		{
			desc:      "stars only",
			body:      `{"stars":12}`,
			wantPatch: db.PluginPatch{Stars: new(12)},
		},
		{
			desc:      "hidden only",
			body:      `{"hidden":false}`,
			wantPatch: db.PluginPatch{Hidden: new(false)},
		},
		{
			desc:      "empty ETag",
			body:      `{"tagsETag":""}`,
			wantPatch: db.PluginPatch{TagsETag: new("")},
		},
		{
			desc:      "all fields",
			body:      `{"stars":0,"hidden":true,"tagsETag":"W/\"abc\""}`,
			wantPatch: db.PluginPatch{Stars: new(0), Hidden: new(true), TagsETag: new(`W/"abc"`)},
		},
	}

	for _, test := range testCases {
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()

			var (
				gotID    string
				gotPatch db.PluginPatch
			)

			testDB := mockDB{
				patchFn: func(_ context.Context, id string, patch db.PluginPatch) error {
					gotID = id
					gotPatch = patch

					return nil
				},
			}

			rw := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPatch, "/123", strings.NewReader(test.body))

			New(testDB, nil, nil, "").Patch(rw, req)

			require.Equal(t, http.StatusNoContent, rw.Code)
			assert.Empty(t, rw.Body.String())
			assert.Equal(t, "123", gotID)
			assert.Equal(t, test.wantPatch, gotPatch)
		})
	}
}

func TestHandlers_Patch_invalid(t *testing.T) {
	testCases := []struct {
		desc     string
		path     string
		body     string
		wantCode int
	}{
		{desc: "empty body", path: "/123", body: "", wantCode: http.StatusBadRequest},
		{desc: "empty object", path: "/123", body: `{}`, wantCode: http.StatusBadRequest},
		{desc: "null values", path: "/123", body: `{"stars":null}`, wantCode: http.StatusBadRequest},
		{desc: "unknown field", path: "/123", body: `{"name":"other"}`, wantCode: http.StatusBadRequest},
		{desc: "invalid JSON", path: "/123", body: `{`, wantCode: http.StatusBadRequest},
		{desc: "missing id", path: "/", body: `{"stars":1}`, wantCode: http.StatusBadRequest},
		{desc: "two JSON objects", path: "/123", body: `{"stars":1}{"hidden":true}`, wantCode: http.StatusBadRequest},
		{desc: "trailing data", path: "/123", body: `{"stars":1} garbage`, wantCode: http.StatusBadRequest},
		{desc: "trailing brace", path: "/123", body: `{"stars":1}}`, wantCode: http.StatusBadRequest},
		{desc: "trailing bracket", path: "/123", body: `{"stars":1}]`, wantCode: http.StatusBadRequest},
		{desc: "negative stars", path: "/123", body: `{"stars":-5}`, wantCode: http.StatusBadRequest},
		{desc: "too large", path: "/123", body: `{"tagsETag":"` + strings.Repeat("a", maxPatchSize) + `"}`, wantCode: http.StatusRequestEntityTooLarge},
	}

	for _, test := range testCases {
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()

			testDB := mockDB{
				patchFn: func(_ context.Context, _ string, patch db.PluginPatch) error {
					// The store is the one that rejects an empty patch.
					if patch == (db.PluginPatch{}) {
						return db.ErrEmptyPatch
					}

					t.Error("store should not be called on invalid input")

					return nil
				},
			}

			rw := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPatch, test.path, strings.NewReader(test.body))

			New(testDB, nil, nil, "").Patch(rw, req)

			assert.Equal(t, test.wantCode, rw.Code)
		})
	}
}

func TestHandlers_Patch_notFound(t *testing.T) {
	testDB := mockDB{
		patchFn: func(_ context.Context, _ string, _ db.PluginPatch) error {
			return db.NotFoundError{}
		},
	}

	rw := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/123", strings.NewReader(`{"stars":1}`))

	New(testDB, nil, nil, "").Patch(rw, req)

	assert.Equal(t, http.StatusNotFound, rw.Code)
}
