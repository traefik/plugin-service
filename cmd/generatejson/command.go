package generatejson

import (
	"github.com/traefik/plugin-service/cmd/internal"
	"github.com/urfave/cli/v2"
)

// Command creates the command for serving the plugin service.
func Command() *cli.Command {
	cmd := &cli.Command{
		Name:        "generatejson",
		Usage:       "Generate JSON of all plugins",
		Description: "Generate JSON from MongoDB collection",
		Flags:       internal.MongoFlags(),
		Action: func(cliCtx *cli.Context) error {
			return run(cliCtx.Context, buildConfig(cliCtx))
		},
	}

	return cmd
}

func buildConfig(cliCtx *cli.Context) Config {
	return Config{
		MongoDB: internal.BuildMongoConfig(cliCtx),
	}
}
