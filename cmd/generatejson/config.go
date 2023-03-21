package generatejson

import (
	"github.com/traefik/plugin-service/pkg/db/mongodb"
)

// Config holds the serve configuration.
type Config struct {
	MongoDB mongodb.Config
}
