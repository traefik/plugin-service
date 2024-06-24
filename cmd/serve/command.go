package serve

import (
	"github.com/ettle/strcase"
	"github.com/traefik/plugin-service/cmd/internal"
	"github.com/traefik/plugin-service/pkg/tracer"
	"github.com/urfave/cli/v2"
)

const (
	flagAddr    = "addr"
	flagGHToken = "github-token"

	flagTraceServiceURL = "trace-service-url"

	flagGoProxyURL      = "go-proxy-url"
	flagGoProxyUsername = "go-proxy-username"
	flagGoProxyPassword = "go-proxy-password"

	flagTracingAddress     = "tracing-address"
	flagTracingInsecure    = "tracing-insecure"
	flagTracingUsername    = "tracing-username"
	flagTracingPassword    = "tracing-password"
	flagTracingProbability = "tracing-probability"
)

// Command creates the command for serving the plugin service.
func Command() *cli.Command {
	cmd := &cli.Command{
		Name:        "serve",
		Usage:       "Serve HTTP",
		Description: "Launch plugin service application",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    flagAddr,
				Usage:   "Addr to listen on.",
				EnvVars: []string{strcase.ToSNAKE(flagAddr)},
			},
			&cli.StringFlag{
				Name:     flagGHToken,
				Usage:    "GitHub Token",
				EnvVars:  []string{strcase.ToSNAKE(flagGHToken)},
				Required: true,
			},
			&cli.StringFlag{
				Name:    flagTraceServiceURL,
				Usage:   "URL of the trace service",
				EnvVars: []string{strcase.ToSNAKE(flagTraceServiceURL)},
			},
		},
		Action: func(cliCtx *cli.Context) error {
			return run(cliCtx.Context, buildConfig(cliCtx))
		},
	}

	cmd.Flags = append(cmd.Flags, goProxyFlags()...)
	cmd.Flags = append(cmd.Flags, tracingFlags()...)
	cmd.Flags = append(cmd.Flags, internal.MongoFlags()...)

	return cmd
}

func buildConfig(cliCtx *cli.Context) Config {
	return Config{
		MongoDB: internal.BuildMongoConfig(cliCtx),
		Tracing: tracer.Config{
			Address:     cliCtx.String(flagTracingAddress),
			Insecure:    cliCtx.Bool(flagTracingInsecure),
			Username:    cliCtx.String(flagTracingUsername),
			Password:    cliCtx.String(flagTracingPassword),
			Probability: cliCtx.Float64(flagTracingProbability),
			ServiceName: "plugin-service",
		},
		TraceURL:    cliCtx.String(flagTraceServiceURL),
		Addr:        cliCtx.String(flagAddr),
		GitHubToken: cliCtx.String(flagGHToken),
		GoProxy: GoProxy{
			URL:      cliCtx.String(flagGoProxyURL),
			Username: cliCtx.String(flagGoProxyUsername),
			Password: cliCtx.String(flagGoProxyPassword),
		},
	}
}

func goProxyFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:     flagGoProxyURL,
			Usage:    "Go Proxy URL",
			EnvVars:  []string{strcase.ToSNAKE(flagGoProxyURL)},
			Required: true,
		},
		&cli.StringFlag{
			Name:     flagGoProxyUsername,
			Usage:    "Go Proxy Username",
			EnvVars:  []string{strcase.ToSNAKE(flagGoProxyUsername)},
			Required: true,
		},
		&cli.StringFlag{
			Name:     flagGoProxyPassword,
			Usage:    "Go Proxy Password",
			EnvVars:  []string{strcase.ToSNAKE(flagGoProxyPassword)},
			Required: true,
		},
	}
}

func tracingFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:    flagTracingAddress,
			Usage:   "Address to send traces",
			EnvVars: []string{strcase.ToSNAKE(flagTracingAddress)},
			Value:   "jaeger.jaeger.svc.cluster.local:4318",
		},
		&cli.BoolFlag{
			Name:    flagTracingInsecure,
			Usage:   "use HTTP instead of HTTPS",
			EnvVars: []string{strcase.ToSNAKE(flagTracingInsecure)},
			Value:   true,
		},
		&cli.StringFlag{
			Name:    flagTracingUsername,
			Usage:   "Username to connect to Jaeger",
			EnvVars: []string{strcase.ToSNAKE(flagTracingUsername)},
			Value:   "jaeger",
		},
		&cli.StringFlag{
			Name:    flagTracingPassword,
			Usage:   "Password to connect to Jaeger",
			EnvVars: []string{strcase.ToSNAKE(flagTracingPassword)},
			Value:   "jaeger",
		},
		&cli.Float64Flag{
			Name:    flagTracingProbability,
			Usage:   "Probability to send traces",
			EnvVars: []string{strcase.ToSNAKE(flagTracingProbability)},
			Value:   0,
		},
	}
}
