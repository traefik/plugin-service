package main

import (
	"os"

	"github.com/rs/zerolog/log"
	"github.com/traefik/plugin-service/cmd/serve"
	"github.com/traefik/plugin-service/pkg/logger"
	"github.com/urfave/cli/v2"
)

func main() {
	logger.Setup()

	app := &cli.App{
		Name:  "Plugin CLI",
		Usage: "Run plugin service",
		Commands: []*cli.Command{
			serve.Command(),
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal().Err(err).Msg("Error while executing command")
	}
}
