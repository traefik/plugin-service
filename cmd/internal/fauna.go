package internal

import (
	"encoding/json"
	"time"

	f "github.com/fauna/faunadb-go/v3/faunadb"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/rs/zerolog/log"
	"github.com/traefik/plugin-service/pkg/db/faunadb"
	"github.com/urfave/cli/v2"
)

// All constants for retry client.
const (
	faunaRetryMax     = 3
	faunaRetryWaitMin = 1 * time.Second
	faunaRetryWaitMax = 3 * time.Second
)

// FaunaFlags setup CLI flags for FaunaDB.
func FaunaFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:     "faunadb-secret",
			Usage:    "FaunaDB secret",
			EnvVars:  []string{"FAUNADB_SECRET"},
			Required: true,
		},
		&cli.StringFlag{
			Name:    "faunadb-endpoint",
			Usage:   "FaunaDB endpoint",
			EnvVars: []string{"FAUNADB_ENDPOINT"},
		},
	}
}

// BuildFaunaConfig created faunadb.Config from CLI flags for FaunaDB.
func BuildFaunaConfig(cliCtx *cli.Context) faunadb.Config {
	return faunadb.Config{
		Database: "plugin",
		Endpoint: cliCtx.String("faunadb-endpoint"),
		Secret:   cliCtx.String("faunadb-secret"),
	}
}

// CreateFaunaClient creates a FaunaDB client.
func CreateFaunaClient(cfg faunadb.Config) (*faunadb.FaunaDB, error) {
	token, opts, err := getDBClientParameters(cfg.Endpoint, cfg.Secret, cfg.Database)
	if err != nil {
		return nil, err
	}

	retryClient := retryablehttp.NewClient()
	retryClient.Logger = &log.Logger
	retryClient.RetryMax = faunaRetryMax
	retryClient.RetryWaitMin = faunaRetryWaitMin
	retryClient.RetryWaitMax = faunaRetryWaitMax

	opts = append(opts, f.HTTP(retryClient.StandardClient()), f.Observer(observer))

	return faunadb.NewFaunaDB(f.NewFaunaClient(token, opts...)), nil
}

// getDBClientParameters returns the secret and the options depending on the environment.
func getDBClientParameters(endpoint, secret, dbName string) (string, []f.ClientConfig, error) {
	var opts []f.ClientConfig

	// Hack to create a key if we are on a local Fauna.
	if endpoint != "" {
		opts = append(opts, f.Endpoint(endpoint))

		adminClient := f.NewFaunaClient(secret, opts...)

		err := createDatabase(adminClient, dbName)
		if err != nil {
			return "", nil, err
		}

		key, err := adminClient.Query(
			f.CreateKey(f.Obj{
				"database": f.Database(dbName),
				"role":     "server",
			}),
		)
		if err != nil {
			return "", nil, err
		}

		err = key.At(f.ObjKey("secret")).Get(&secret)
		if err != nil {
			return "", nil, err
		}
	}

	return secret, opts, nil
}

func createDatabase(adminClient *f.FaunaClient, name string) error {
	result, err := adminClient.Query(f.Exists(f.Database(name)))
	if err != nil {
		return err
	}

	if !getExist(result) {
		_, err = adminClient.Query(f.CreateDatabase(f.Obj{"name": name}))
		if err != nil {
			return err
		}
	}

	return nil
}

func getExist(key f.Value) (exist bool) {
	_ = key.Get(&exist)
	return
}

func observer(result *f.QueryResult) {
	if result.StatusCode/100 != 2 {
		query, _ := json.Marshal(result.Query)

		ctx := log.With().
			Str("query", string(query)).
			Int("query", result.StatusCode).
			Time("StartTime", result.StartTime).
			Time("EndTime", result.EndTime)

		for name, values := range result.Headers {
			ctx.Strs("RESPONSE_HEADER_"+name, values)
		}

		logger := ctx.Logger()
		logger.Error().Msg("faunaDB call")
	}
}
