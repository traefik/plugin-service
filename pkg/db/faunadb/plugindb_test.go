package faunadb

import (
	"context"
	"testing"

	f "github.com/fauna/faunadb-go/v3/faunadb"
	"github.com/stretchr/testify/require"
	"github.com/traefik/plugin-service/pkg/db"
)

const prefix = "plugin-test-"

func TestNameDB(t *testing.T) {
	store := createTempDB(t, nil)

	data, err := store.Create(context.Background(), db.Plugin{
		ID:            "123",
		Name:          "github.com/traefik/plugintest",
		DisplayName:   "Add Header",
		Author:        "ldez",
		Type:          "middleware",
		Import:        "jdklsq",
		Compatibility: "dkjslkq",
		Summary:       "dsqsd",
		IconURL:       "icon.png",
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

	_, err = store.Update(context.Background(), data.ID, db.Plugin{
		Name:          "github.com/traefik/plugintest",
		DisplayName:   "Foo",
		Author:        "ldez",
		Type:          "middleware",
		Import:        "abc",
		Compatibility: "def",
		Summary:       "",
		BannerURL:     "banner.png",
		Readme:        "",
		LatestVersion: "",
		Versions:      nil,
		Stars:         4,
		Snippet:       nil,
	})
	require.NoError(t, err)

	_, _, err = store.List(context.Background(), db.Pagination{Size: 100})
	require.NoError(t, err)

	_, err = store.GetByName(context.Background(), "github.com/traefik/plugintest")
	require.NoError(t, err)
}

func populate(t *testing.T, db *FaunaDB, plugins []db.Plugin) {
	t.Helper()

	for _, plugin := range plugins {
		_, err := db.Create(context.Background(), plugin)
		require.NoError(t, err)
	}
}

func getSecret(key f.Value) (secret string) {
	_ = key.At(f.ObjKey("secret")).Get(&secret)
	return
}
