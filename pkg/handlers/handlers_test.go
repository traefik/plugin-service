package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/traefik/plugin-service/pkg/db"
)

func TestHandlers_List(t *testing.T) {
	data := []db.Plugin{
		{
			Author:        "traefik",
			Compatibility: "v2",
			CreatedAt:     time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC),
			DisplayName:   "Demo Plugin",
			ID:            "276809780784267776",
			Import:        "github.com/traefik/plugindemo",
			LatestVersion: "v0.2.1",
			Name:          "github.com/traefik/plugindemo",
			Readme:        "README",
			Snippet:       map[string]interface{}{"toml": "toml", "yaml": "yaml"},
			Stars:         22,
			Summary:       "[Demo] Add Request Header",
			Type:          "middleware",
			Versions:      []string{"v0.2.1", "v0.2.0", "v0.1.0"},
		},
		{
			Author:        "traefik",
			Compatibility: "v2",
			CreatedAt:     time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC),
			DisplayName:   "Block Path",
			ID:            "2768097807845374",
			Import:        "github.com/traefik/plugin-blockpath",
			LatestVersion: "v0.3.1",
			Name:          "github.com/traefik/plugin-blockpath",
			Readme:        "README",
			Snippet:       map[string]interface{}{"toml": "toml", "yaml": "yaml"},
			Stars:         3,
			Summary:       "Block Path plugin",
			Type:          "middleware",
			Versions:      []string{"v0.3.1", "v0.2.0", "v0.1.0"},
		},
	}

	testDB := mockDB{
		listFn: func(_ context.Context, _ db.Pagination) ([]db.Plugin, string, error) {
			return data, "next", nil
		},
	}

	rw := httptest.NewRecorder()

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)

	New(testDB, nil, nil).List(rw, req)

	assert.Equal(t, http.StatusOK, rw.Code)
	assert.Equal(t, "next", rw.Header().Get(nextPageHeader))

	file, err := os.ReadFile("./fixtures/get_plugins.json")
	require.NoError(t, err)

	assert.JSONEq(t, string(file), rw.Body.String())
}

func TestHandlers_List_GetByName(t *testing.T) {
	data := db.Plugin{
		Author:        "traefik",
		Compatibility: "v2",
		CreatedAt:     time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC),
		DisplayName:   "Demo Plugin",
		ID:            "276809780784267776",
		Import:        "github.com/traefik/plugindemo",
		LatestVersion: "v0.2.1",
		Name:          "github.com/traefik/plugindemo",
		Readme:        "README",
		Snippet:       map[string]interface{}{"toml": "toml", "yaml": "yaml"},
		Stars:         22,
		Summary:       "[Demo] Add Request Header",
		Type:          "middleware",
		Versions:      []string{"v0.2.1", "v0.2.0", "v0.1.0"},
	}

	testDB := mockDB{
		getByNameFn: func(_ context.Context, _ string, _ bool) (db.Plugin, error) {
			return data, nil
		},
	}

	rw := httptest.NewRecorder()

	req := httptest.NewRequest(http.MethodGet, "/?name=Demo%20Plugin", http.NoBody)

	New(testDB, nil, nil).getByName(rw, req)

	assert.Equal(t, http.StatusOK, rw.Code)

	file, err := os.ReadFile("./fixtures/get_plugin_by_name.json")
	require.NoError(t, err)

	assert.JSONEq(t, string(file), rw.Body.String())
}

func TestHandlers_List_GetByName_hidden(t *testing.T) {
	data := db.Plugin{
		Author:        "traefik",
		Compatibility: "v2",
		CreatedAt:     time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC),
		DisplayName:   "Demo Plugin",
		ID:            "276809780784267776",
		Import:        "github.com/traefik/plugindemo",
		LatestVersion: "v0.2.1",
		Name:          "github.com/traefik/plugindemo",
		Readme:        "README",
		Snippet:       map[string]interface{}{"toml": "toml", "yaml": "yaml"},
		Stars:         22,
		Summary:       "[Demo] Add Request Header",
		Type:          "middleware",
		Versions:      []string{"v0.2.1", "v0.2.0", "v0.1.0"},
		Hidden:        true,
	}

	testDB := mockDB{
		getByNameFn: func(_ context.Context, _ string, filterHidden bool) (db.Plugin, error) {
			if filterHidden {
				return data, nil
			}
			return db.Plugin{}, db.NotFoundError{}
		},
	}

	rw := httptest.NewRecorder()

	req := httptest.NewRequest(http.MethodGet, "/?name=Demo%20Plugin", http.NoBody)

	New(testDB, nil, nil).getByName(rw, req)

	assert.Equal(t, http.StatusNotFound, rw.Code)

	rw = httptest.NewRecorder()

	req = httptest.NewRequest(http.MethodGet, "/?name=Demo%20Plugin&filterHidden=true", http.NoBody)

	New(testDB, nil, nil).getByName(rw, req)

	assert.Equal(t, http.StatusOK, rw.Code)

	file, err := os.ReadFile("./fixtures/get_plugin_by_name_hidden.json")
	require.NoError(t, err)

	assert.JSONEq(t, string(file), rw.Body.String())
}

func TestHandlers_List_SearchByName(t *testing.T) {
	data := db.Plugin{
		Author:        "traefik",
		Compatibility: "v2",
		CreatedAt:     time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC),
		DisplayName:   "Demo Plugin",
		ID:            "276809780784267776",
		Import:        "github.com/traefik/plugindemo",
		LatestVersion: "v0.2.1",
		Name:          "github.com/traefik/plugindemo",
		Readme:        "README",
		Snippet:       map[string]interface{}{"toml": "toml", "yaml": "yaml"},
		Stars:         22,
		Summary:       "[Demo] Add Request Header",
		Type:          "middleware",
		Versions:      []string{"v0.2.1", "v0.2.0", "v0.1.0"},
	}

	testDB := mockDB{
		getByNameFn: func(_ context.Context, _ string, _ bool) (db.Plugin, error) {
			return data, nil
		},
	}

	rw := httptest.NewRecorder()

	req := httptest.NewRequest(http.MethodGet, "/?query=demo", http.NoBody)

	New(testDB, nil, nil).getByName(rw, req)

	assert.Equal(t, http.StatusOK, rw.Code)

	file, err := os.ReadFile("./fixtures/get_plugin_by_name.json")
	require.NoError(t, err)

	assert.JSONEq(t, string(file), rw.Body.String())
}
