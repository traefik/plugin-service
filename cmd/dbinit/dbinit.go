package dbinit

import (
	"github.com/fauna/faunadb-go/v3/faunadb"
	"github.com/traefik/plugin-service/pkg/db"
	"github.com/urfave/cli/v2"
)

// Run initialize the production database.
func Run(ctx *cli.Context) error {
	database := db.NewFaunaDB(faunadb.NewFaunaClient(ctx.String("faunadb-secret")))

	return database.Bootstrap()
}
