package main

import (
	"os"

	"github.com/rs/zerolog/log"
	"github.com/traefik/plugin-service/cmd/dbinit"
	"github.com/traefik/token-service/cmd/serve"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:  "Plugin CLI",
		Usage: "Run plugin service",
		Commands: []*cli.Command{
			dbInitCommand(),
			serveCommand(),
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal().Err(err).Msg("Error while executing command")
	}
}

func dbInitCommand() *cli.Command {
	cmd := &cli.Command{
		Name:        "dbinit",
		Usage:       "Init database",
		Description: "Init production database",
		Action:      dbinit.Run,
	}

	cmd.Flags = append(cmd.Flags, sharedFlags()...)

	return cmd
}

func serveCommand() *cli.Command {
	cmd := &cli.Command{
		Name:        "serve",
		Usage:       "Serve HTTP",
		Description: "Launch plugin service application",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "host",
				Usage:   "Host to listen on.",
				EnvVars: []string{"PILOT_HOST"},
			},
			&cli.StringFlag{
				Name:     "token-url",
				Usage:    "Token Service URL",
				EnvVars:  []string{"PILOT_TOKEN_URL"},
				Required: true,
			},
			&cli.StringFlag{
				Name:     "jwt-cert",
				Usage:    "Pilot JWT Cert",
				EnvVars:  []string{"PILOT_JWT_CERT"},
				Required: true,
			},
			&cli.StringFlag{
				Name:     "services-access-token",
				Usage:    "Pilot Services Access Token",
				EnvVars:  []string{"PILOT_SERVICES_ACCESS_TOKEN"},
				Required: true,
			},
			&cli.StringFlag{
				Name:     "go-proxy-url",
				Usage:    "Pilot Go Proxy URL",
				EnvVars:  []string{"PILOT_GO_PROXY_URL"},
				Required: true,
			},
			&cli.StringFlag{
				Name:     "go-proxy-username",
				Usage:    "Pilot Go Proxy Username",
				EnvVars:  []string{"PILOT_GO_PROXY_USERNAME"},
				Required: true,
			},
			&cli.StringFlag{
				Name:     "go-proxy-password",
				Usage:    "Pilot Go Proxy Password",
				EnvVars:  []string{"PILOT_GO_PROXY_PASSWORD"},
				Required: true,
			},
			&cli.StringFlag{
				Name:     "github-token",
				Usage:    "Pilot GitHub Token",
				EnvVars:  []string{"PILOT_GITHUB_TOKEN"},
				Required: true,
			},
			&cli.StringFlag{
				Name:     "tracing-endpoint",
				Usage:    "Endpoint to send traces",
				EnvVars:  []string{"TRACING_ENDPOINT"},
				Value:    "https://collector.infra.traefiklabs.tech",
				Required: false,
			},
			&cli.StringFlag{
				Name:     "tracing-username",
				Usage:    "Username to connect to Jaeger",
				EnvVars:  []string{"TRACING_USERNAME"},
				Value:    "jaeger",
				Required: false,
			},
			&cli.StringFlag{
				Name:     "tracing-password",
				Usage:    "Password to connect to Jaeger",
				EnvVars:  []string{"TRACING_PASSWORD"},
				Value:    "jaeger",
				Required: false,
			},
			&cli.Float64Flag{
				Name:     "tracing-probability",
				Usage:    "Probability to send traces.",
				EnvVars:  []string{"TRACING_PROBABILITY"},
				Value:    0,
				Required: false,
			},
		},
		Action: serve.Run,
	}

	cmd.Flags = append(cmd.Flags, sharedFlags()...)

	return cmd
}

func sharedFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:     "faunadb-secret",
			Usage:    "FaunaDB secret",
			EnvVars:  []string{"FAUNADB_SECRET"},
			Required: true,
		},
		&cli.StringFlag{
			Name:     "faunadb-endpoint",
			Usage:    "FaunaDB endpoint",
			EnvVars:  []string{"FAUNADB_ENDPOINT"},
			Required: false,
		},
	}
}
