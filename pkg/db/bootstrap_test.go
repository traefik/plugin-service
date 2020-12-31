package db

import (
	"crypto/rand"
	"math/big"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	f "github.com/fauna/faunadb-go/v3/faunadb"
	"github.com/stretchr/testify/require"
)

func createTempDB(t *testing.T, plugins []Plugin) *FaunaDB {
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

	db := NewFaunaDB(client)
	db.collName = dbName

	err := db.Bootstrap()
	require.NoError(t, err)

	if len(plugins) > 0 {
		populate(t, db, plugins)
	}

	return db
}
