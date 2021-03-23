package faunadb

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	f "github.com/fauna/faunadb-go/v3/faunadb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/traefik/plugin-service/pkg/db"
)

const prefix = "plugin-test-"

func TestFaunaDB_Create(t *testing.T) {
	ctx := context.Background()
	store, _, _ := createTempDB(t, nil)

	plugin := db.Plugin{
		ID:            "123",
		Name:          "name",
		DisplayName:   "display-name",
		Author:        "author",
		Type:          "type",
		Import:        "import",
		Compatibility: "compatibility",
		Summary:       "summary",
		IconURL:       "iconURL",
		BannerURL:     "bannerURL",
		Readme:        "readme",
		LatestVersion: "latestVersion",
		Versions:      []string{"v1.0.0"},
		Stars:         10,
		Snippet:       map[string]interface{}{},
		CreatedAt:     time.Now().Add(-2 * time.Hour),
	}

	got, err := store.Create(ctx, plugin)
	require.NoError(t, err)

	assert.NotEqual(t, plugin.ID, got.ID)
	assert.NotEqual(t, plugin.CreatedAt, got.CreatedAt)

	plugin.ID = got.ID
	plugin.CreatedAt = got.CreatedAt

	assert.Equal(t, plugin, got)

	stored := getPlugin(t, store, got.ID)

	assert.Equal(t, got, stored)
}

func TestFaunaDB_Get(t *testing.T) {
	ctx := context.Background()
	store, fixtures, _ := createTempDB(t, []fixture{
		{
			key: "plugin-1",
			plugin: db.Plugin{
				ID:            "123",
				Name:          "name",
				DisplayName:   "display-name",
				Author:        "author",
				Type:          "type",
				Import:        "import",
				Compatibility: "compatibility",
				Summary:       "summary",
				IconURL:       "iconURL",
				BannerURL:     "bannerURL",
				Readme:        "readme",
				LatestVersion: "latestVersion",
				Versions:      []string{"v1.0.0"},
				Stars:         10,
				Snippet: map[string]interface{}{
					"something": "there",
				},
				CreatedAt: time.Now().Add(-2 * time.Hour),
			},
		},
	})

	// Make sure we can get an existing plugin.
	got, err := store.Get(ctx, fixtures["plugin-1"].ID)
	require.NoError(t, err)

	assert.Equal(t, fixtures["plugin-1"], got)

	// Make sure we receive a NotFound error when the plugin doesn't exist.
	_, err = store.Get(ctx, "456")
	require.ErrorAs(t, err, &db.ErrNotFound{})
}

func TestFaunaDB_Delete(t *testing.T) {
	ctx := context.Background()
	store, fixtures, _ := createTempDB(t, []fixture{
		{
			key:    "plugin-1",
			plugin: db.Plugin{ID: "123"},
		},
	})

	// Make sure we can delete an existing plugin.
	err := store.Delete(ctx, fixtures["plugin-1"].ID)
	require.NoError(t, err)

	// Make sure we receive a NotFound error when the plugin doesn't exist.
	err = store.Delete(ctx, "456")
	require.ErrorAs(t, err, &db.ErrNotFound{})
}

func TestFaunaDB_List(t *testing.T) {
	ctx := context.Background()
	store, fixtures, _ := createTempDB(t, []fixture{
		{
			key:    "9-stars",
			plugin: db.Plugin{ID: "234", Stars: 9},
		},
		{
			key:    "10-stars",
			plugin: db.Plugin{ID: "123", Stars: 10},
		},
		{
			key:    "8-stars",
			plugin: db.Plugin{ID: "456", Stars: 8},
		},
	})

	// Make sure plugins are listed ordered by stars and respect pagination constraints
	page := db.Pagination{Size: 2}
	plugins, next, err := store.List(ctx, page)
	require.NoError(t, err)

	assert.Equal(t, []db.Plugin{
		fixtures["10-stars"],
		fixtures["9-stars"],
	}, plugins)
	assert.Equal(t, buildListNextID(t, fixtures["8-stars"]), next)

	// Make sure we can query the next page
	page.Start = next
	plugins, next, err = store.List(ctx, page)
	require.NoError(t, err)

	assert.Equal(t, []db.Plugin{
		fixtures["8-stars"],
	}, plugins)
	assert.Zero(t, next)
}

func TestFaunaDB_GetByName(t *testing.T) {
	ctx := context.Background()
	store, fixtures, _ := createTempDB(t, []fixture{
		{
			key: "my-super-plugin",
			plugin: db.Plugin{
				ID:            "123",
				Name:          "my-super-plugin",
				DisplayName:   "display-name",
				Author:        "author",
				Type:          "type",
				Import:        "import",
				Compatibility: "compatibility",
				Summary:       "summary",
				IconURL:       "iconURL",
				BannerURL:     "bannerURL",
				Readme:        "readme",
				LatestVersion: "latestVersion",
				Versions:      []string{"v1.0.0"},
				Stars:         10,
				Snippet: map[string]interface{}{
					"something": "there",
				},
				CreatedAt: time.Now().Add(-2 * time.Hour),
			},
		},
	})

	// Make sure we can get an existing plugin.
	got, err := store.GetByName(ctx, "my-super-plugin")
	require.NoError(t, err)

	assert.Equal(t, fixtures["my-super-plugin"], got)

	// Make sure we receive a NotFound error when the plugin doesn't exist.
	_, err = store.GetByName(ctx, "something-else")
	require.ErrorAs(t, err, &db.ErrNotFound{})
}

func TestFaunaDB_SearchByName(t *testing.T) {
	ctx := context.Background()
	store, fixtures, _ := createTempDB(t, []fixture{
		{
			key: "plugin-1",
			plugin: db.Plugin{
				ID:            "123",
				Name:          "plugin-1",
				DisplayName:   "plugin-1",
				Author:        "author",
				Type:          "type",
				Import:        "import",
				Compatibility: "compatibility",
				Summary:       "summary",
				IconURL:       "iconURL",
				BannerURL:     "bannerURL",
				Readme:        "readme",
				LatestVersion: "latestVersion",
				Versions:      []string{"v1.0.0"},
				Stars:         10,
				Snippet: map[string]interface{}{
					"something": "there",
				},
				CreatedAt: time.Now().Add(-2 * time.Hour),
			},
		},
		{
			key: "plugin-2",
			plugin: db.Plugin{
				ID:            "234",
				Name:          "plugin-2",
				DisplayName:   "plugin-2",
				Author:        "author",
				Type:          "type",
				Import:        "import",
				Compatibility: "compatibility",
				Summary:       "summary",
				IconURL:       "iconURL",
				BannerURL:     "bannerURL",
				Readme:        "readme",
				LatestVersion: "latestVersion",
				Versions:      []string{"v1.0.0"},
				Stars:         10,
				Snippet: map[string]interface{}{
					"something": "there",
				},
				CreatedAt: time.Now().Add(-2 * time.Hour),
			},
		},
		{
			key: "plugin-3",
			plugin: db.Plugin{
				ID:            "345",
				Name:          "plugin-3",
				DisplayName:   "plugin-3",
				Author:        "author",
				Type:          "type",
				Import:        "import",
				Compatibility: "compatibility",
				Summary:       "summary",
				IconURL:       "iconURL",
				BannerURL:     "bannerURL",
				Readme:        "readme",
				LatestVersion: "latestVersion",
				Versions:      []string{"v1.0.0"},
				Stars:         10,
				Snippet: map[string]interface{}{
					"something": "there",
				},
				CreatedAt: time.Now().Add(-2 * time.Hour),
			},
		},
		{
			key: "plugin-4",
			plugin: db.Plugin{
				ID:            "456",
				Name:          "plugin-4",
				DisplayName:   "plugin-4",
				Author:        "author",
				Type:          "type",
				Import:        "import",
				Compatibility: "compatibility",
				Summary:       "summary",
				IconURL:       "iconURL",
				BannerURL:     "bannerURL",
				Readme:        "readme",
				LatestVersion: "latestVersion",
				Versions:      []string{"v1.0.0"},
				Stars:         10,
				Snippet: map[string]interface{}{
					"something": "there",
				},
				CreatedAt: time.Now().Add(-2 * time.Hour),
			},
		},
		{
			key: "plugin-5",
			plugin: db.Plugin{
				ID:            "147",
				Name:          "plugin-5",
				DisplayName:   "salad-tomate-onion",
				Author:        "author",
				Type:          "type",
				Import:        "import",
				Compatibility: "compatibility",
				Summary:       "summary",
				IconURL:       "iconURL",
				BannerURL:     "bannerURL",
				Readme:        "readme",
				LatestVersion: "latestVersion",
				Versions:      []string{"v1.0.0"},
				Stars:         10,
				Snippet: map[string]interface{}{
					"something": "there",
				},
				CreatedAt: time.Now().Add(-2 * time.Hour),
			},
		},
		{
			key: "plugin-6",
			plugin: db.Plugin{
				ID:            "741",
				Name:          "plugin-6",
				DisplayName:   "salad-tom.te-onion",
				Author:        "author",
				Type:          "type",
				Import:        "import",
				Compatibility: "compatibility",
				Summary:       "summary",
				IconURL:       "iconURL",
				BannerURL:     "bannerURL",
				Readme:        "readme",
				LatestVersion: "latestVersion",
				Versions:      []string{"v1.0.0"},
				Stars:         10,
				Snippet: map[string]interface{}{
					"something": "there",
				},
				CreatedAt: time.Now().Add(-2 * time.Hour),
			},
		},
		{
			key: "plugin-7",
			plugin: db.Plugin{
				ID:            "258",
				Name:          "plugin-7",
				DisplayName:   "hi^hello",
				Author:        "author",
				Type:          "type",
				Import:        "import",
				Compatibility: "compatibility",
				Summary:       "summary",
				IconURL:       "iconURL",
				BannerURL:     "bannerURL",
				Readme:        "readme",
				LatestVersion: "latestVersion",
				Versions:      []string{"v1.0.0"},
				Stars:         10,
				Snippet: map[string]interface{}{
					"something": "there",
				},
				CreatedAt: time.Now().Add(-2 * time.Hour),
			},
		},
		{
			key: "plugin-8",
			plugin: db.Plugin{
				ID:            "852",
				Name:          "plugin-8",
				DisplayName:   "hello",
				Author:        "author",
				Type:          "type",
				Import:        "import",
				Compatibility: "compatibility",
				Summary:       "summary",
				IconURL:       "iconURL",
				BannerURL:     "bannerURL",
				Readme:        "readme",
				LatestVersion: "latestVersion",
				Versions:      []string{"v1.0.0"},
				Stars:         10,
				Snippet: map[string]interface{}{
					"something": "there",
				},
				CreatedAt: time.Now().Add(-2 * time.Hour),
			},
		},
		{
			key: "plugin-9",
			plugin: db.Plugin{
				ID:            "369",
				Name:          "plugin-9",
				DisplayName:   "h([]){}.*p",
				Author:        "author",
				Type:          "type",
				Import:        "import",
				Compatibility: "compatibility",
				Summary:       "summary",
				IconURL:       "iconURL",
				BannerURL:     "bannerURL",
				Readme:        "readme",
				LatestVersion: "latestVersion",
				Versions:      []string{"v1.0.0"},
				Stars:         10,
				Snippet: map[string]interface{}{
					"something": "there",
				},
				CreatedAt: time.Now().Add(-2 * time.Hour),
			},
		},
		{
			key: "plugins-10",
			plugin: db.Plugin{
				ID:            "963",
				Name:          "plugins-10",
				DisplayName:   "*",
				Author:        "author",
				Type:          "type",
				Import:        "import",
				Compatibility: "compatibility",
				Summary:       "summary",
				IconURL:       "iconURL",
				BannerURL:     "bannerURL",
				Readme:        "readme",
				LatestVersion: "latestVersion",
				Versions:      []string{"v1.0.0"},
				Stars:         10,
				Snippet: map[string]interface{}{
					"something": "there",
				},
				CreatedAt: time.Now().Add(-2 * time.Hour),
			},
		},
	})

	tests := []struct {
		desc        string
		pagination  db.Pagination
		query       string
		wantPlugins []db.Plugin
		wantNextID  string
		wantErr     bool
	}{
		{
			desc:       "page 1/2 with 2 elements per page: no query",
			pagination: db.Pagination{Size: 2},
			wantPlugins: []db.Plugin{
				fixtures["plugins-10"],
				fixtures["plugin-9"],
			},
			wantNextID: buildSearchNextID(t, fixtures["plugin-8"]),
		},
		{
			desc: "page 2/2 with 2 elements per page: no query",
			pagination: db.Pagination{
				Start: buildSearchNextID(t, fixtures["plugin-8"]),
				Size:  2,
			},
			wantPlugins: []db.Plugin{
				fixtures["plugin-8"],
				fixtures["plugin-7"],
			},
			wantNextID: buildSearchNextID(t, fixtures["plugin-1"]),
		},
		{
			desc:        "query: 'tomate' matches 'salad-tomate-onion",
			pagination:  db.Pagination{Size: 10},
			query:       "tomate",
			wantPlugins: []db.Plugin{fixtures["plugin-5"]},
		},
		{
			desc:        "query: '-tomate-' matches 'salad-tomate-onion",
			pagination:  db.Pagination{Size: 10},
			query:       "tomate",
			wantPlugins: []db.Plugin{fixtures["plugin-5"]},
		},
		{
			desc:        "query: 'tom.ate' matches 'salad-tom.te-onion",
			pagination:  db.Pagination{Size: 10},
			query:       "tom.te",
			wantPlugins: []db.Plugin{fixtures["plugin-6"]},
		},
		{
			desc:        "query: '^hello' matches 'hi^hello",
			pagination:  db.Pagination{Size: 10},
			query:       "^hello",
			wantPlugins: []db.Plugin{fixtures["plugin-7"]},
		},
		{
			desc:        "query: 'h([]){}.*p' matches 'h([]){}.*p",
			pagination:  db.Pagination{Size: 10},
			query:       "h([]){}.*p",
			wantPlugins: []db.Plugin{fixtures["plugin-9"]},
		},
		{
			desc:       "query: '*' matches 'toto*titi and sort by name",
			pagination: db.Pagination{Size: 10},
			query:      "*",
			wantPlugins: []db.Plugin{
				fixtures["plugins-10"],
				fixtures["plugin-9"],
			},
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.desc, func(t *testing.T) {
			plugins, nextID, err := store.SearchByName(ctx, test.query, test.pagination)
			if test.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.wantPlugins, plugins)
			assert.Equal(t, test.wantNextID, nextID)
		})
	}
}

func TestFaunaDB_Update(t *testing.T) {
	ctx := context.Background()

	store, fixtures, _ := createTempDB(t, []fixture{
		{
			key: "plugin",
			plugin: db.Plugin{
				ID:            "123",
				Name:          "plugin",
				DisplayName:   "plugin",
				Author:        "author",
				Type:          "type",
				Import:        "import",
				Compatibility: "compatibility",
				Summary:       "summary",
				IconURL:       "icon",
				BannerURL:     "banner",
				Readme:        "readme",
				LatestVersion: "v1.1.1",
				Versions: []string{
					"v1.1.1",
				},
				Stars:   10,
				Snippet: nil,
			},
		},
	})

	got, err := store.Update(ctx, "123", db.Plugin{
		ID:            "123",
		Name:          "New Name",
		DisplayName:   "plugin",
		Author:        "New Author",
		Type:          "type",
		Import:        "import",
		Compatibility: "compatibility",
		Summary:       "summary",
		IconURL:       "icon",
		BannerURL:     "banner",
		Readme:        "readme",
		LatestVersion: "v1.1.1",
		Versions: []string{
			"v1.1.1",
		},
		Stars:   10,
		Snippet: nil,
	})
	require.NoError(t, err)

	want := fixtures["plugin"]
	want.Name = "New Name"
	want.Author = "New Author"

	assert.Equal(t, want, got)

	// Update with same values
	got, err = store.Update(ctx, "123", got)
	require.NoError(t, err)

	assert.Equal(t, want, got)

	// Check that we get a db.NotFound when no plugin have the given id.
	_, err = store.Update(ctx, "456", got)
	require.ErrorAs(t, err, &db.ErrNotFound{})
}

func TestMongoDB_CreateHash(t *testing.T) {
	ctx := context.Background()

	store, _, _ := createTempDB(t, []fixture{
		{
			key: "plugin",
			plugin: db.Plugin{
				ID:            "123",
				Name:          "plugin",
				DisplayName:   "plugin",
				Author:        "author",
				Type:          "type",
				Import:        "import",
				Compatibility: "compatibility",
				Summary:       "summary",
				IconURL:       "icon",
				BannerURL:     "banner",
				Readme:        "readme",
				LatestVersion: "v1.1.1",
				Versions: []string{
					"v1.1.1",
				},
				Stars:   10,
				Snippet: nil,
			},
			hashes: []db.PluginHash{
				{Name: "plugin@v1.1.1", Hash: "123"},
			},
		},
	})

	got, err := store.CreateHash(ctx, "plugin", "v1.2.3", "hash")
	require.NoError(t, err)

	assert.Equal(t, db.PluginHash{Name: "plugin@v1.2.3", Hash: "hash"}, got)

	// Create the same hash works
	got, err = store.CreateHash(ctx, "plugin", "v1.2.3", "hash")
	require.NoError(t, err)

	assert.Equal(t, db.PluginHash{Name: "plugin@v1.2.3", Hash: "hash"}, got)
}

func TestMongoDB_GetHashByName(t *testing.T) {
	ctx := context.Background()

	store, _, fixturesHashes := createTempDB(t, []fixture{
		{
			key: "plugin",
			plugin: db.Plugin{
				ID:            "123",
				Name:          "plugin",
				DisplayName:   "plugin",
				Author:        "author",
				Type:          "type",
				Import:        "import",
				Compatibility: "compatibility",
				Summary:       "summary",
				IconURL:       "icon",
				BannerURL:     "banner",
				Readme:        "readme",
				LatestVersion: "v1.1.1",
				Versions: []string{
					"v1.1.1",
				},
				Stars:   10,
				Snippet: nil,
			},
			hashes: []db.PluginHash{
				{Name: "plugin@v1.1.2", Hash: "123"},
				{Name: "plugin@v1.1.3", Hash: "123"},
				{Name: "plugin@v1.1.1", Hash: "123"},
			},
		},
	})

	// Check last version
	got, err := store.GetHashByName(ctx, "plugin", "v1.1.2")
	require.NoError(t, err)

	assert.Equal(t, fixturesHashes["plugin"][0], got)

	// Check older version
	got, err = store.GetHashByName(ctx, "plugin", "v1.1.1")
	require.NoError(t, err)

	assert.Equal(t, fixturesHashes["plugin"][2], got)

	// Check non existing version
	_, err = store.GetHashByName(ctx, "plugin", "v1.1.4")
	require.ErrorAs(t, err, &db.ErrNotFound{})
}

func getSecret(key f.Value) (secret string) {
	_ = key.At(f.ObjKey("secret")).Get(&secret)
	return
}

func getPlugin(t *testing.T, client *FaunaDB, id string) db.Plugin {
	t.Helper()

	res, err := client.client.Query(
		f.Get(f.RefCollection(f.Collection(client.collName), id)),
	)
	require.NoError(t, err)

	plugin, err := decodePlugin(res)
	require.NoError(t, err)

	return plugin
}

func buildListNextID(t *testing.T, next db.Plugin) string {
	t.Helper()

	b, err := json.Marshal(NextPageList{
		NextID: next.ID,
		Stars:  next.Stars,
	})
	require.NoError(t, err)

	return base64.RawStdEncoding.EncodeToString(b)
}

func buildSearchNextID(t *testing.T, next db.Plugin) string {
	t.Helper()

	b, err := json.Marshal(NextPageSearch{
		NextID: next.ID,
		Name:   next.DisplayName,
	})
	require.NoError(t, err)

	return base64.RawStdEncoding.EncodeToString(b)
}
