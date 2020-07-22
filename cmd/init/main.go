package main

import (
	"flag"
	"os"

	"github.com/containous/plugin-service/pkg/db"
	"github.com/containous/plugin-service/pkg/logger"
	"github.com/fauna/faunadb-go/faunadb"
	"github.com/rs/zerolog/log"
)

func main() {
	logger.Setup()

	secret := flag.String("secret", os.Getenv("FAUNADB_SECRET"), "secret for database access")

	flag.Parse()

	if secret == nil || len(*secret) == 0 {
		log.Fatal().Msg("You need to specify secret")
	}

	database := db.NewFaunaDB(faunadb.NewFaunaClient(*secret))
	err := database.Bootstrap()
	if err != nil {
		log.Fatal().Err(err).Msg("Error while bootstraping")
	}
}
