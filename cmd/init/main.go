package main

import (
	"flag"
	"os"

	"github.com/rs/zerolog/log"

	"github.com/containous/plugin-service/pkg/db"
	"github.com/fauna/faunadb-go/faunadb"
)

func main() {
	secret := flag.String("secret", os.Getenv("FAUNADB_SECRET"), "secret for database access")

	flag.Parse()

	if secret == nil || len(*secret) == 0 {
		log.Fatal().Msg("You need to specify secret")
	}

	database := db.NewFaunaDB(faunadb.NewFaunaClient(*secret))
	err := database.Bootstrap()
	if err != nil {
		log.Fatal().Msgf("Error while bootstraping: %v", err)
	}
}
