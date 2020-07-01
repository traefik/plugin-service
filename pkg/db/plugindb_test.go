package db

import (
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	f "github.com/fauna/faunadb-go/faunadb"
	"github.com/stretchr/testify/require"
)

const prefix = "plugin-test-"

func TestNameDB(t *testing.T) {
	db := createTempDB(t, nil)

	data, err := db.Create(Plugin{
		Name:          "github.com/containous/plugintest",
		DisplayName:   "Add Header",
		Author:        "ldez",
		Type:          "middleware",
		Import:        "jdklsq",
		Compatibility: "dkjslkq",
		Summary:       "dsqsd",
		Readme:        "gfdsdfghjhg",
		LatestVersion: "v1.1.0",
		Versions:      []string{"aaa", "bbb"},
		Stars:         42,
		Snippet: map[string]interface{}{
			"Header": map[string]interface{}{
				"foo": "bar",
			},
		},
	})
	require.NoError(t, err)

	fmt.Println(data)

	_, err = db.Update("test", Plugin{
		Name:          "test",
		DisplayName:   "",
		Author:        "ldez",
		Type:          "middleware",
		Import:        "abc",
		Compatibility: "def",
		Summary:       "",
		Readme:        "",
		LatestVersion: "",
		Versions:      nil,
		Stars:         4,
		Snippet:       nil,
	})
	require.NoError(t, err)

	list, next, err := db.List(Pagination{Size: 100})
	require.NoError(t, err)

	fmt.Println(next)
	fmt.Println(list)

	value, err := db.GetByName("test")
	require.NoError(t, err)

	fmt.Println(value)
}

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

	dbName := prefix + strconv.Itoa(rand.New(rand.NewSource(time.Now().Unix())).Int())

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

func populate(t *testing.T, db *FaunaDB, plugins []Plugin) {
	for _, plugin := range plugins {
		_, err := db.Create(plugin)
		require.NoError(t, err)
	}
}

func getSecret(key f.Value) (secret string) {
	_ = key.At(f.ObjKey("secret")).Get(&secret)
	return
}
