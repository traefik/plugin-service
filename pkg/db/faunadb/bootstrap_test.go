package faunadb

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	f "github.com/fauna/faunadb-go/v3/faunadb"
	"github.com/stretchr/testify/require"
	"github.com/traefik/plugin-service/pkg/db"
)

type fixture struct {
	key    string
	plugin db.Plugin
	hashes []db.PluginHash
}

func createTempDB(t *testing.T, fixtures []fixture) (*FaunaDB, map[string]db.Plugin, map[string][]db.PluginHash) {
	t.Helper()

	var count int
	for {
		resp, err := http.Get("http://127.0.0.1:8443")
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		if err == nil && resp.StatusCode == http.StatusUnauthorized {
			break
		}

		time.Sleep(time.Second)

		require.Less(t, count, 10, "Timeout when contacting local database")

		count++
	}

	n, errRand := rand.Int(rand.Reader, big.NewInt(time.Now().Unix()))
	require.NoError(t, errRand)
	dbName := prefix + strconv.Itoa(n.Sign())

	var client *f.FaunaClient
	if os.Getenv("FAUNADB_SECRET") == "" {
		// only with faunadb docker image
		secret := "secret"
		adminClient := f.NewFaunaClient(secret, f.Endpoint("http://127.0.0.1:8443"))

		_, err := adminClient.Query(f.CreateDatabase(f.Obj{"name": dbName}))
		require.NoError(t, err)

		key, err := adminClient.Query(
			f.CreateKey(f.Obj{
				"database": f.Database(dbName),
				"role":     "server",
			}),
		)

		client = adminClient.NewSessionClient(getSecret(key))
		t.Cleanup(func() {
			_, err = adminClient.Query(f.Delete(f.Database(dbName)))
			require.NoError(t, err)
		})
	} else {
		client = f.NewFaunaClient(os.Getenv("FAUNADB_SECRET"))
		t.Cleanup(func() {
			_, err := client.Query(f.Delete(f.Collection(dbName)))
			require.NoError(t, err)
		})
	}

	store := NewFaunaDB(client)
	store.collName = dbName

	err := store.Bootstrap()
	require.NoError(t, err)

	indexedPluginFixtures := make(map[string]db.Plugin)
	indexedHashFixtures := make(map[string][]db.PluginHash)

	for _, fixt := range fixtures {
		indexedPluginFixtures[fixt.key], err = createPlugin(store, fixt.plugin)
		require.NoError(t, err)

		for _, hash := range fixt.hashes {
			storedHash, err := createHash(store, hash)

			indexedHashFixtures[fixt.key] = append(indexedHashFixtures[fixt.key], storedHash)

			require.NoError(t, err)
		}
	}

	return store, indexedPluginFixtures, indexedHashFixtures
}

func createPlugin(fauna *FaunaDB, plugin db.Plugin) (db.Plugin, error) {
	// FaunaDB saves dates as timestamp without changing the location to UTC.
	plugin.CreatedAt = plugin.CreatedAt.UTC()

	res, err := fauna.client.Query(f.Create(
		f.RefCollection(f.Collection(fauna.collName), plugin.ID),
		f.Obj{"data": plugin}))
	if err != nil {
		return db.Plugin{}, err
	}

	return decodePlugin(res)
}

func createHash(fauna *FaunaDB, hash db.PluginHash) (db.PluginHash, error) {
	id, err := fauna.client.Query(f.NewId())
	if err != nil {
		return db.PluginHash{}, fmt.Errorf("fauna error: %w", err)
	}

	res, err := fauna.client.Query(f.Create(
		f.RefCollection(f.Collection(fauna.collName+collNameHashSuffix), id),
		f.Obj{"data": hash}))
	if err != nil {
		return db.PluginHash{}, err
	}

	return decodePluginHash(res)
}
