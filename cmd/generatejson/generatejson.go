package generatejson

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/traefik/plugin-service/cmd/internal"
)

func run(ctx context.Context, cfg Config) error {
	store, tearDown, err := internal.CreateMongoClient(ctx, cfg.MongoDB)
	if err != nil {
		return fmt.Errorf("unable to create MongoDB client: %w", err)
	}
	defer tearDown()

	plugins, err := store.ListAll(ctx)
	if err != nil {
		return fmt.Errorf("unable to list plugins: %w", err)
	}
	if err := json.NewEncoder(os.Stdout).Encode(plugins); err != nil {
		return fmt.Errorf("unable to encode plugins to json: %w", err)
	}

	return nil
}
