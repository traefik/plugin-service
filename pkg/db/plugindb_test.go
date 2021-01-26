package db

import (
	"context"
	"testing"

	f "github.com/fauna/faunadb-go/v3/faunadb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const prefix = "plugin-test-"

func TestNameDB(t *testing.T) {
	db := createTempDB(t, nil)

	data, err := db.Create(context.Background(), Plugin{
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

	_, err = db.Update(context.Background(), data.ID, Plugin{
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

	_, count, _, err := db.List(context.Background(), Pagination{Size: 100})
	require.NoError(t, err)
	assert.Equal(t, count, "1")

	_, err = db.GetByName(context.Background(), "github.com/traefik/plugintest")
	require.NoError(t, err)
}

func populate(t *testing.T, db *FaunaDB, plugins []Plugin) {
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
