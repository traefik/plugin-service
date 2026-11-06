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
			var (
				gotID    string
				gotPatch db.PluginPatch
			)

			testDB := mockDB{
				patchFn: func(_ context.Context, id string, patch db.PluginPatch) (db.Plugin, error) {
					gotID = id
					gotPatch = patch

					return db.Plugin{ID: id, Name: "github.com/traefik/plugindemo"}, nil
				},
			}

			rw := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPatch, "/123", strings.NewReader(test.body))

			New(testDB, nil, nil, "").Patch(rw, req)

			require.Equal(t, http.StatusOK, rw.Code)
			assert.Equal(t, "123", gotID)
			assert.Equal(t, test.wantPatch, gotPatch)
		})
	}
}

func TestHandlers_Patch_badRequest(t *testing.T) {
	testDB := mockDB{
		patchFn: func(_ context.Context, _ string, _ db.PluginPatch) (db.Plugin, error) {
			t.Fatal("store should not be called on invalid input")
			return db.Plugin{}, nil
		},
	}

	testCases := []struct {
		desc string
		path string
		body string
	}{
		{desc: "empty body", path: "/123", body: ""},
		{desc: "empty object", path: "/123", body: `{}`},
		{desc: "null values", path: "/123", body: `{"stars":null}`},
		{desc: "unknown field", path: "/123", body: `{"name":"other"}`},
		{desc: "invalid JSON", path: "/123", body: `{`},
		{desc: "missing id", path: "/", body: `{"stars":1}`},
	}

	for _, test := range testCases {
		t.Run(test.desc, func(t *testing.T) {
			rw := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPatch, test.path, strings.NewReader(test.body))

			New(testDB, nil, nil, "").Patch(rw, req)

			assert.Equal(t, http.StatusBadRequest, rw.Code)
		})
	}
}

func TestHandlers_Patch_notFound(t *testing.T) {
	testDB := mockDB{
		patchFn: func(_ context.Context, _ string, _ db.PluginPatch) (db.Plugin, error) {
			return db.Plugin{}, db.NotFoundError{}
		},
	}

	rw := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/123", strings.NewReader(`{"stars":1}`))

	New(testDB, nil, nil, "").Patch(rw, req)

	assert.Equal(t, http.StatusNotFound, rw.Code)
}
