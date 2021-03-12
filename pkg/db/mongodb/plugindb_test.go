package mongodb

import (
	"context"
	"crypto/rand"
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
	require.Error(t, err)
	assert.ErrorAs(t, err, &db.ErrNotFound{})
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
		Username:   "mongoadmin",
		Password:   "secret",
	}

	client, err := mongo.NewClient(clientOptions)
	require.NoError(t, err)

	err = client.Connect(ctx)
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

		_, err := mongodb.client.Collection(mongodb.collName).InsertOne(ctx, f.plugin)
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
