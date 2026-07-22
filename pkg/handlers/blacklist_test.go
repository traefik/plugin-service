package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/traefik/plugin-service/pkg/db"
)

func TestHandlers_ListBlacklist(t *testing.T) {
	entries := []db.BlacklistEntry{
		{Repository: "containous/plugintestxxx", Reason: "Crash piceus", Author: "alice", CreatedAt: time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC)},
		{Repository: "deas/teectl", Reason: "Not a plugin", Author: "bob", CreatedAt: time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC)},
	}

	testDB := mockDB{
		listBlacklistFn: func(_ context.Context) ([]db.BlacklistEntry, error) {
			return entries, nil
		},
	}

	rw := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/internal/blacklist", http.NoBody)

	New(testDB, nil, nil).ListBlacklist(rw, req)

	require.Equal(t, http.StatusOK, rw.Code)
	assert.JSONEq(t, `[
		{"repository":"containous/plugintestxxx","reason":"Crash piceus","author":"alice","createdAt":"2020-01-01T01:00:00Z"},
		{"repository":"deas/teectl","reason":"Not a plugin","author":"bob","createdAt":"2020-01-01T01:00:00Z"}
	]`, rw.Body.String())
}

func TestHandlers_AddToBlacklist(t *testing.T) {
	var got db.BlacklistEntry

	testDB := mockDB{
		upsertBlacklistFn: func(_ context.Context, entry db.BlacklistEntry) (db.BlacklistEntry, error) {
			got = entry
			entry.CreatedAt = time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC)
			return entry, nil
		},
	}

	rw := httptest.NewRecorder()
	body := `{"repository":"deas/teectl","reason":"Not a plugin","author":"alice"}`
	req := httptest.NewRequest(http.MethodPost, "/internal/blacklist", strings.NewReader(body))

	New(testDB, nil, nil).AddToBlacklist(rw, req)

	require.Equal(t, http.StatusOK, rw.Code)
	assert.Equal(t, "deas/teectl", got.Repository)
	assert.Equal(t, "Not a plugin", got.Reason)
	assert.Equal(t, "alice", got.Author)
}

func TestHandlers_AddToBlacklist_invalidRepository(t *testing.T) {
	testDB := mockDB{
		upsertBlacklistFn: func(_ context.Context, _ db.BlacklistEntry) (db.BlacklistEntry, error) {
			t.Fatal("store should not be called on invalid input")
			return db.BlacklistEntry{}, nil
		},
	}

	for _, repository := range []string{"", "no-slash", "too/many/slashes", "with space/repo"} {
		rw := httptest.NewRecorder()
		body := `{"repository":"` + repository + `"}`
		req := httptest.NewRequest(http.MethodPost, "/internal/blacklist", strings.NewReader(body))

		New(testDB, nil, nil).AddToBlacklist(rw, req)

		assert.Equalf(t, http.StatusBadRequest, rw.Code, "repository %q", repository)
	}
}

func TestHandlers_DeleteFromBlacklist(t *testing.T) {
	var deleted string

	testDB := mockDB{
		deleteBlacklistFn: func(_ context.Context, repository string) error {
			deleted = repository
			return nil
		},
	}

	rw := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/internal/blacklist?repository=deas/teectl", http.NoBody)

	New(testDB, nil, nil).DeleteFromBlacklist(rw, req)

	require.Equal(t, http.StatusNoContent, rw.Code)
	assert.Equal(t, "deas/teectl", deleted)
}

func TestHandlers_DeleteFromBlacklist_missingRepository(t *testing.T) {
	testDB := mockDB{
		deleteBlacklistFn: func(_ context.Context, _ string) error {
			t.Fatal("store should not be called without a repository")
			return nil
		},
	}

	rw := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/internal/blacklist", http.NoBody)

	New(testDB, nil, nil).DeleteFromBlacklist(rw, req)

	assert.Equal(t, http.StatusBadRequest, rw.Code)
}

func TestHandlers_DeleteFromBlacklist_notFound(t *testing.T) {
	testDB := mockDB{
		deleteBlacklistFn: func(_ context.Context, _ string) error {
			return db.NotFoundError{}
		},
	}

	rw := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/internal/blacklist?repository=deas/teectl", http.NoBody)

	New(testDB, nil, nil).DeleteFromBlacklist(rw, req)

	assert.Equal(t, http.StatusNotFound, rw.Code)
}
