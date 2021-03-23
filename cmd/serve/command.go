package serve

import (
	"encoding/base64"
	"fmt"

	"github.com/traefik/plugin-service/cmd/internal"
	"github.com/urfave/cli/v2"
)

// Command creates the command for serving the plugin service.
func Command() *cli.Command {
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
				Name:     "github-token",
				Usage:    "Pilot GitHub Token",
				EnvVars:  []string{"PILOT_GITHUB_TOKEN"},
				Required: true,
			},
			&cli.StringFlag{
				Name:    "database-driver",
				Usage:   "DB to use (fauna/mongo).",
				EnvVars: []string{"PILOT_DATABASE_DRIVER"},
				Value:   "fauna",
			},
		},
		Action: func(cliCtx *cli.Context) error {
			cfg, err := buildConfig(cliCtx)
			if err != nil {
				return err
			}

			return run(cliCtx.Context, cfg)
		},
	}

	cmd.Flags = append(cmd.Flags, goProxyFlags()...)
	cmd.Flags = append(cmd.Flags, tracingFlags()...)
	cmd.Flags = append(cmd.Flags, internal.FaunaFlags()...)
	cmd.Flags = append(cmd.Flags, internal.MongoFlags()...)

	return cmd
}

func buildConfig(cliCtx *cli.Context) (Config, error) {
	servicesAccessToken, err := base64.StdEncoding.DecodeString(cliCtx.String("services-access-token"))
	if err != nil {
		return Config{}, fmt.Errorf("unable to decode services access token: %w", err)
	}

	return Config{
		FaunaDB: internal.BuildFaunaConfig(cliCtx),
		MongoDB: internal.BuildMongoConfig(cliCtx),
		Tracing: Tracing{
			Endpoint:    cliCtx.String("tracing-endpoint"),
			Username:    cliCtx.String("tracing-username"),
			Password:    cliCtx.String("tracing-password"),
			Probability: cliCtx.Float64("tracing-probability"),
		},
		Pilot: Pilot{
			Host:                cliCtx.String("host"),
			JWTCert:             cliCtx.String("jwt-cert"),
			TokenURL:            cliCtx.String("token-url"),
			GitHubToken:         cliCtx.String("github-token"),
			ServicesAccessToken: string(servicesAccessToken),
			DatabaseDriver:      cliCtx.String("database-driver"),
		},
		GoProxy: GoProxy{
			URL:      cliCtx.String("go-proxy-url"),
			Username: cliCtx.String("go-proxy-username"),
			Password: cliCtx.String("go-proxy-password"),
		},
	}, nil
}

func goProxyFlags() []cli.Flag {
	return []cli.Flag{
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
	}
}

func tracingFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:    "tracing-endpoint",
			Usage:   "Endpoint to send traces",
			EnvVars: []string{"TRACING_ENDPOINT"},
			Value:   "https://collector.infra.traefiklabs.tech",
		},
		&cli.StringFlag{
			Name:    "tracing-username",
			Usage:   "Username to connect to Jaeger",
			EnvVars: []string{"TRACING_USERNAME"},
			Value:   "jaeger",
		},
		&cli.StringFlag{
			Name:    "tracing-password",
			Usage:   "Password to connect to Jaeger",
			EnvVars: []string{"TRACING_PASSWORD"},
			Value:   "jaeger",
		},
		&cli.Float64Flag{
			Name:    "tracing-probability",
			Usage:   "Probability to send traces.",
			EnvVars: []string{"TRACING_PROBABILITY"},
			Value:   0,
		},
	}
}
