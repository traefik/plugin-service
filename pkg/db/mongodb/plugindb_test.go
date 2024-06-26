package mongodb

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/traefik/plugin-service/pkg/db"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestMongoDB_Create(t *testing.T) {
	ctx := context.Background()
	store, _ := createDatabase(t, nil)

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
		Snippet: map[string]interface{}{
			"something": "there",
		},
		CreatedAt: time.Now().Add(-2 * time.Hour),
	}

	got, err := store.Create(ctx, plugin)
	require.NoError(t, err)

	plugin = toUTCPlugin(plugin)
	got = toUTCPlugin(got)

	assert.NotEqual(t, plugin.ID, got.ID)
	assert.NotEqual(t, plugin.CreatedAt, got.CreatedAt)

	plugin.ID = got.ID
	plugin.CreatedAt = got.CreatedAt

	assert.Equal(t, plugin, got)

	stored, ok := getPlugin(t, store, got.ID)
	require.True(t, ok)

	stored.Plugin = toUTCPlugin(stored.Plugin)

	assert.Equal(t, got, stored.Plugin)
	assert.Empty(t, stored.Hashes)
}

func TestMongoDB_Get(t *testing.T) {
	ctx := context.Background()
	store, fixtures := createDatabase(t, []fixture{
		{
			key: "plugin-1",
			plugin: pluginDocument{
				Plugin: db.Plugin{
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
		},
	})

	// Make sure we can get an existing plugin.
	got, err := store.Get(ctx, fixtures["plugin-1"].ID)
	require.NoError(t, err)

	assert.Equal(t, fixtures["plugin-1"].Plugin, toUTCPlugin(got))

	// Make sure we receive a NotFound error when the plugin doesn't exist.
	_, err = store.Get(ctx, "456")
	require.ErrorAs(t, err, &db.NotFoundError{})
}

func TestMongoDB_Delete(t *testing.T) {
	ctx := context.Background()
	store, fixtures := createDatabase(t, []fixture{
		{
			key: "plugin-1",
			plugin: pluginDocument{
				Plugin: db.Plugin{ID: "123"},
			},
		},
	})

	// Make sure we can delete an existing plugin.
	err := store.Delete(ctx, fixtures["plugin-1"].ID)
	require.NoError(t, err)

	// Make sure we receive a NotFound error when the plugin doesn't exist.
	err = store.Delete(ctx, "456")
	require.ErrorAs(t, err, &db.NotFoundError{})
}

func TestMongoDB_List(t *testing.T) {
	ctx := context.Background()
	store, fixtures := createDatabase(t, []fixture{
		{
			key: "9-stars",
			plugin: pluginDocument{
				Plugin: db.Plugin{ID: "234", Stars: 9},
			},
		},
		{
			key: "10-stars",
			plugin: pluginDocument{
				Plugin: db.Plugin{ID: "123", Stars: 10},
			},
		},
		{
			key: "8-stars",
			plugin: pluginDocument{
				Plugin: db.Plugin{ID: "456", Stars: 8},
			},
		},
		{
			key: "disabled",
			plugin: pluginDocument{
				Plugin: db.Plugin{ID: "789", Stars: 8, Disabled: true},
			},
		},
	})

	// Make sure plugins are listed ordered by stars and respect pagination constraints
	page := db.Pagination{Size: 2}
	plugins, next, err := store.List(ctx, page)
	require.NoError(t, err)

	assert.Equal(t, []db.Plugin{
		fixtures["10-stars"].Plugin,
		fixtures["9-stars"].Plugin,
	}, plugins)
	assert.Equal(t, fixtures["8-stars"].ID, next)

	// Make sure we can query the next page
	page.Start = next
	plugins, next, err = store.List(ctx, page)
	require.NoError(t, err)

	assert.Equal(t, []db.Plugin{
		fixtures["8-stars"].Plugin,
	}, plugins)
	assert.Zero(t, next)
}

func TestMongoDB_GetByName(t *testing.T) {
	ctx := context.Background()
	store, fixtures := createDatabase(t, []fixture{
		{
			key: "my-super-plugin",
			plugin: pluginDocument{
				Plugin: db.Plugin{
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
					Disabled:  true,
				},
			},
		},
		{
			key: "my-super--hidden-plugin",
			plugin: pluginDocument{
				Plugin: db.Plugin{
					ID:            "456",
					Name:          "my-super-hidden-plugin",
					DisplayName:   "hidden-display-name",
					Author:        "hidden-author",
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
					Disabled:  false,
					Hidden:    true,
				},
			},
		},
	})

	// Make sure we can get an existing plugin.
	got, err := store.GetByName(ctx, "my-super-plugin", false, false)
	require.NoError(t, err)

	assert.Equal(t, fixtures["my-super-plugin"].Plugin, toUTCPlugin(got))

	// Make sure we can't get an existing disabled plugin.
	_, err = store.GetByName(ctx, "my-super-plugin", true, false)
	require.Error(t, err)

	// Make sure we can get an existing hidden plugin.
	_, err = store.GetByName(ctx, "my-super-hidden-plugin", true, false)
	require.NoError(t, err)

	// Make sure we can't get an existing hidden plugin.
	_, err = store.GetByName(ctx, "my-super-hidden-plugin", true, true)
	require.Error(t, err)

	// Make sure we receive a NotFound error when the plugin doesn't exist.
	_, err = store.GetByName(ctx, "something-else", true, false)
	require.ErrorAs(t, err, &db.NotFoundError{})
}

func TestMongoDB_SearchByName(t *testing.T) {
	ctx := context.Background()
	store, fixtures := createDatabase(t, []fixture{
		{
			key: "plugin-1",
			plugin: pluginDocument{
				Plugin: db.Plugin{
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
		},
		{
			key: "plugin-2",
			plugin: pluginDocument{
				Plugin: db.Plugin{
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
		},
		{
			key: "plugin-3",
			plugin: pluginDocument{
				Plugin: db.Plugin{
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
		},
		{
			key: "plugin-4",
			plugin: pluginDocument{
				Plugin: db.Plugin{
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
		},
		{
			key: "plugin-5",
			plugin: pluginDocument{
				Plugin: db.Plugin{
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
		},
		{
			key: "plugin-6",
			plugin: pluginDocument{
				Plugin: db.Plugin{
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
		},
		{
			key: "plugin-7",
			plugin: pluginDocument{
				Plugin: db.Plugin{
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
		},
		{
			key: "plugin-8",
			plugin: pluginDocument{
				Plugin: db.Plugin{
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
		},
		{
			key: "plugin-9",
			plugin: pluginDocument{
				Plugin: db.Plugin{
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
		},
		{
			key: "plugins-10",
			plugin: pluginDocument{
				Plugin: db.Plugin{
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
		},
		{
			key: "plugins-11",
			plugin: pluginDocument{
				Plugin: db.Plugin{
					ID:            "4242",
					Name:          "plugins-11",
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
					Disabled:  true,
				},
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
				fixtures["plugins-10"].Plugin,
				fixtures["plugin-9"].Plugin,
			},
			wantNextID: buildNextID(t, fixtures["plugin-8"].Plugin),
		},
		{
			desc: "page 2/2 with 2 elements per page: no query",
			pagination: db.Pagination{
				Start: buildNextID(t, fixtures["plugin-8"].Plugin),
				Size:  2,
			},
			wantPlugins: []db.Plugin{
				fixtures["plugin-8"].Plugin,
				fixtures["plugin-7"].Plugin,
			},
			wantNextID: buildNextID(t, fixtures["plugin-1"].Plugin),
		},
		{
			desc:        "query: 'tomate' matches 'salad-tomate-onion",
			pagination:  db.Pagination{Size: 2},
			query:       "tomate",
			wantPlugins: []db.Plugin{fixtures["plugin-5"].Plugin},
		},
		{
			desc:        "query: '-tomate-' matches 'salad-tomate-onion",
			pagination:  db.Pagination{Size: 2},
			query:       "tomate",
			wantPlugins: []db.Plugin{fixtures["plugin-5"].Plugin},
		},
		{
			desc:        "query: 'tom.ate' matches 'salad-tom.te-onion",
			pagination:  db.Pagination{Size: 2},
			query:       "tom.te",
			wantPlugins: []db.Plugin{fixtures["plugin-6"].Plugin},
		},
		{
			desc:        "query: '^hello' matches 'hi^hello",
			pagination:  db.Pagination{Size: 2},
			query:       "^hello",
			wantPlugins: []db.Plugin{fixtures["plugin-7"].Plugin},
		},
		{
			desc:        "query: 'h([]){}.*p' matches 'h([]){}.*p",
			pagination:  db.Pagination{Size: 2},
			query:       "h([]){}.*p",
			wantPlugins: []db.Plugin{fixtures["plugin-9"].Plugin},
		},
		{
			desc:       "query: '*' matches 'toto*titi and sort by name",
			pagination: db.Pagination{Size: 2},
			query:      "*",
			wantPlugins: []db.Plugin{
				fixtures["plugins-10"].Plugin,
				fixtures["plugin-9"].Plugin,
			},
		},
	}

	for _, test := range tests {
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

func TestMongoDB_Update(t *testing.T) {
	ctx := context.Background()

	store, fixtures := createDatabase(t, []fixture{
		{
			key: "plugin",
			plugin: pluginDocument{
				Plugin: db.Plugin{
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
				Hashes: []db.PluginHash{
					{Name: "plugin@v1.1.1", Hash: "123"},
				},
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

	want := fixtures["plugin"].Plugin
	want.Name = "New Name"
	want.Author = "New Author"

	assert.Equal(t, want, got)

	// Check hashes are not updated
	var pluginWithHashes pluginDocument
	err = store.client.Collection(store.collName).
		FindOne(ctx, bson.D{{Key: "id", Value: "123"}}).
		Decode(&pluginWithHashes)
	require.NoError(t, err)

	assert.Equal(t, fixtures["plugin"].Hashes, pluginWithHashes.Hashes)

	// Update with same values
	got, err = store.Update(ctx, "123", got)
	require.NoError(t, err)

	assert.Equal(t, want, got)

	// Check that we get a db.NotFound when no plugin have the given id.
	_, err = store.Update(ctx, "456", got)
	require.ErrorAs(t, err, &db.NotFoundError{})
}

func TestMongoDB_CreateHash(t *testing.T) {
	ctx := context.Background()

	store, fixtures := createDatabase(t, []fixture{
		{
			key: "plugin",
			plugin: pluginDocument{
				Plugin: db.Plugin{
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
				Hashes: []db.PluginHash{
					{Name: "plugin@v1.1.1", Hash: "123"},
				},
			},
		},
	})

	_, err := store.CreateHash(ctx, "plugin", "v1.2.3", "hash")
	require.NoError(t, err)

	var pluginWithHashes pluginDocument
	err = store.client.Collection(store.collName).
		FindOne(ctx, bson.D{{Key: "id", Value: "123"}}).
		Decode(&pluginWithHashes)
	require.NoError(t, err)

	want := []db.PluginHash{
		fixtures["plugin"].Hashes[0],
		{
			Name: "plugin@v1.2.3",
			Hash: "hash",
		},
	}

	assert.Equal(t, want, pluginWithHashes.Hashes)

	// With embedded hashes, creating a new one doesn't works if the plugin doesn't exists.
	_, err = store.CreateHash(ctx, "toto", "v1.2.3", "hash")
	require.ErrorAs(t, err, &db.NotFoundError{})
}

func TestMongoDB_GetHashByName(t *testing.T) {
	ctx := context.Background()

	store, fixtures := createDatabase(t, []fixture{
		{
			key: "plugin",
			plugin: pluginDocument{
				Plugin: db.Plugin{
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
				Hashes: []db.PluginHash{
					{Name: "plugin@v1.1.2", Hash: "123"},
					{Name: "plugin@v1.1.3", Hash: "123"},
					{Name: "plugin@v1.1.1", Hash: "123"},
				},
			},
		},
	})

	// Check last version
	got, err := store.GetHashByName(ctx, "plugin", "v1.1.2")
	require.NoError(t, err)

	assert.Equal(t, fixtures["plugin"].Hashes[0], got)

	// Check older version
	got, err = store.GetHashByName(ctx, "plugin", "v1.1.1")
	require.NoError(t, err)

	assert.Equal(t, fixtures["plugin"].Hashes[2], got)

	// Check non existing version
	_, err = store.GetHashByName(ctx, "plugin", "v1.1.4")
	require.ErrorAs(t, err, &db.NotFoundError{})
}

type fixture struct {
	key    string
	plugin pluginDocument
}

func createDatabase(t *testing.T, fixtures []fixture) (*MongoDB, map[string]pluginDocument) {
	t.Helper()

	ctx := context.Background()

	n, err := rand.Int(rand.Reader, big.NewInt(time.Now().Unix()))
	require.NoError(t, err)

	dbName := "plugin-" + strconv.Itoa(n.Sign())

	clientOptions := options.Client().ApplyURI("mongodb://127.0.0.1:27017/" + dbName)
	clientOptions.Auth = &options.Credential{
		AuthSource: "admin",
		Username:   "root",
		Password:   "secret",
	}

	client, err := mongo.Connect(ctx, clientOptions)
	require.NoError(t, err)

	database := client.Database(dbName)
	mongodb := NewMongoDB(database)

	err = mongodb.Bootstrap()
	require.NoError(t, err)

	t.Cleanup(func() {
		err = database.Drop(ctx)
		require.NoError(t, err)
	})

	indexedFixtures := make(map[string]pluginDocument)

	for _, f := range fixtures {
		f.plugin.MongoID = primitive.NewObjectID()
		f.plugin.CreatedAt = f.plugin.CreatedAt.Truncate(time.Millisecond)

		_, err = mongodb.client.Collection(mongodb.collName).InsertOne(ctx, f.plugin)
		require.NoError(t, err)

		// Fixtures date needs to converted back to UTC to allow using assert.Equal
		// even if timezones differ.
		f.plugin.Plugin = toUTCPlugin(f.plugin.Plugin)

		indexedFixtures[f.key] = f.plugin
	}

	return mongodb, indexedFixtures
}

func getPlugin(t *testing.T, store *MongoDB, id string) (pluginDocument, bool) {
	t.Helper()

	ctx := context.Background()
	criteria := bson.D{{Key: "id", Value: id}}

	var plugin pluginDocument
	if err := store.client.Collection(store.collName).FindOne(ctx, criteria).Decode(&plugin); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return pluginDocument{}, false
		}

		require.NoError(t, err)
	}

	return plugin, true
}

// toUTCPlugin converts plugin dates to UTC.
func toUTCPlugin(plugin db.Plugin) db.Plugin {
	plugin.CreatedAt = plugin.CreatedAt.UTC()

	return plugin
}

func buildNextID(t *testing.T, next db.Plugin) string {
	t.Helper()

	b, err := json.Marshal(db.NextPage{
		NextID: next.ID,
		Name:   next.Name,
	})
	require.NoError(t, err)

	return base64.RawStdEncoding.EncodeToString(b)
}
