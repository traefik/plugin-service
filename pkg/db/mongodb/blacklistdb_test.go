package mongodb

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/traefik/plugin-service/pkg/db"
)

func TestMongoDB_Blacklist(t *testing.T) {
	ctx := context.Background()
	store, _ := createDatabase(t, nil)

	// Empty at first.
	entries, err := store.ListBlacklist(ctx)
	require.NoError(t, err)
	assert.Empty(t, entries)

	// Insert.
	created, err := store.UpsertBlacklist(ctx, db.BlacklistEntry{
		Repository: "deas/teectl",
		Reason:     "Not a plugin",
		Author:     "alice",
	})
	require.NoError(t, err)
	assert.Equal(t, "deas/teectl", created.Repository)
	assert.False(t, created.CreatedAt.IsZero())

	// Upsert keeps CreatedAt and updates reason/author.
	updated, err := store.UpsertBlacklist(ctx, db.BlacklistEntry{
		Repository: "deas/teectl",
		Reason:     "Still not a plugin",
		Author:     "bob",
	})
	require.NoError(t, err)
	assert.Equal(t, "Still not a plugin", updated.Reason)
	assert.Equal(t, "bob", updated.Author)
	assert.Equal(t, created.CreatedAt, updated.CreatedAt)

	// List returns the single entry.
	entries, err = store.ListBlacklist(ctx)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "deas/teectl", entries[0].Repository)

	// Delete.
	require.NoError(t, store.DeleteBlacklist(ctx, "deas/teectl"))

	// Delete missing returns NotFoundError.
	err = store.DeleteBlacklist(ctx, "deas/teectl")
	require.ErrorAs(t, err, &db.NotFoundError{})

	entries, err = store.ListBlacklist(ctx)
	require.NoError(t, err)
	assert.Empty(t, entries)
}
