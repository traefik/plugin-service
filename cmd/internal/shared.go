package internal

import (
	"github.com/fauna/faunadb-go/v3/faunadb"
)

const dbName = "plugin"

// GetDBClientParameters returns the secret and the options depending on the environment.
func GetDBClientParameters(endpoint, secret string) (string, []faunadb.ClientConfig, error) {
	var options []faunadb.ClientConfig

	// Hack to create a key if we are on a local Fauna.
	if endpoint != "" {
		options = append(options, faunadb.Endpoint(endpoint))

		adminClient := faunadb.NewFaunaClient(secret, options...)

		err := createDatabase(adminClient)
		if err != nil {
			return "", nil, err
		}

		key, err := adminClient.Query(
			faunadb.CreateKey(faunadb.Obj{
				"database": faunadb.Database(dbName),
				"role":     "server",
			}),
		)
		if err != nil {
			return "", nil, err
		}

		err = key.At(faunadb.ObjKey("secret")).Get(&secret)
		if err != nil {
			return "", nil, err
		}
	}

	return secret, options, nil
}

func createDatabase(adminClient *faunadb.FaunaClient) error {
	result, err := adminClient.Query(faunadb.Exists(faunadb.Database(dbName)))
	if err != nil {
		return err
	}

	if !getExist(result) {
		_, err = adminClient.Query(faunadb.CreateDatabase(faunadb.Obj{"name": dbName}))
		if err != nil {
			return err
		}
	}

	return nil
}

func getExist(key faunadb.Value) (exist bool) {
	_ = key.Get(&exist)
	return
}
