package dbinit

import (
	f "github.com/fauna/faunadb-go/v3/faunadb"
	"github.com/traefik/plugin-service/cmd/internal"
	"github.com/traefik/plugin-service/pkg/db/faunadb"
	"github.com/urfave/cli/v2"
)

// Command creates the command for initializing the production database.
func Command() *cli.Command {
	cmd := &cli.Command{
		Name:        "dbinit",
		Usage:       "Init database",
		Description: "Init production database",
		Action: func(cliCtx *cli.Context) error {
			cfg := internal.BuildFaunaConfig(cliCtx)

			return run(cfg)
		},
	}

	cmd.Flags = append(cmd.Flags, internal.FaunaFlags()...)

	return cmd
}

func run(cfg faunadb.Config) error {
	database := faunadb.NewFaunaDB(f.NewFaunaClient(cfg.Secret))

	return database.Bootstrap()
}
