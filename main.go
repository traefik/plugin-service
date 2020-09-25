package main

import (
	"flag"
	"net/http"
	"os"

	"github.com/fauna/faunadb-go/v3/faunadb"
	"github.com/rs/zerolog/log"
	"github.com/traefik/plugin-service/pkg/db"
	"github.com/traefik/plugin-service/pkg/functions"
	"github.com/traefik/plugin-service/pkg/logger"
)

func main() {
	logger.Setup()

	host := flag.String("host", "0.0.0.0:8080", "listening port and hostname")
	secret := flag.String("secret", os.Getenv("FAUNADB_SECRET"), "Secret for database access.")
	endpoint := flag.String("endpoint", os.Getenv("FAUNADB_ENDPOINT"), "Endpoint for database access.")
	help := flag.Bool("h", false, "show this help")

	flag.Usage = usage
	flag.Parse()
	if *help {
		usage()
	}

	nArgs := flag.NArg()
	if nArgs > 0 {
		usage()
	}

	if secret == nil || *secret == "" {
		log.Fatal().Msg("FaunaDB secret is required.")
	}

	var options []faunadb.ClientConfig
	if endpoint != nil && *endpoint != "" {
		if err := os.Setenv("FAUNADB_ENDPOINT", *endpoint); err != nil {
			log.Fatal().Err(err).Msg("Unable to set FAUNADB_ENDPOINT")
		}
		options = append(options, faunadb.Endpoint(*endpoint))
	}

	token, err := initDB(*secret, options)
	if err != nil {
		log.Fatal().Err(err).Msg("Error while bootstraping")
	}

	if err = os.Setenv("FAUNADB_SECRET", token); err != nil {
		log.Fatal().Err(err).Msg("Unable to set FAUNADB_SECRET")
	}

	if err = bootstrap(token, options); err != nil {
		log.Fatal().Err(err).Msg("Error while bootstraping")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/public/", functions.Public)
	mux.HandleFunc("/internal/", functions.Internal)
	mux.HandleFunc("/external/", functions.External)

	err = http.ListenAndServe(*host, mux)
	if err != nil {
		log.Fatal().Err(err).Msg("Error in http server")
	}
}

func usage() {
	_, _ = os.Stderr.WriteString("\t plugin-service \n")
	flag.PrintDefaults()
	os.Exit(2)
}

func initDB(secret string, options []faunadb.ClientConfig) (string, error) {
	dbName := "plugin"

	adminClient := faunadb.NewFaunaClient(secret, options...)
	result, err := adminClient.Query(faunadb.Exists(faunadb.Database(dbName)))
	if err != nil {
		return "", err
	}

	if !getExist(result) {
		_, err = adminClient.Query(faunadb.CreateDatabase(faunadb.Obj{"name": dbName}))
		if err != nil {
			return "", err
		}
	}

	key, err := adminClient.Query(
		faunadb.CreateKey(faunadb.Obj{
			"database": faunadb.Database(dbName),
			"role":     "server",
		}),
	)

	return getSecret(key), err
}

func getSecret(key faunadb.Value) (secret string) {
	_ = key.At(faunadb.ObjKey("secret")).Get(&secret)
	return
}

func getExist(key faunadb.Value) (exist bool) {
	_ = key.Get(&exist)
	return
}

func bootstrap(token string, options []faunadb.ClientConfig) error {
	database := db.NewFaunaDB(faunadb.NewFaunaClient(token, options...))
	return database.Bootstrap()
}
