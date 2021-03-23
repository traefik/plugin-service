package internal

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/traefik/plugin-service/pkg/db/mongodb"
	"github.com/urfave/cli/v2"
	"go.mongodb.org/mongo-driver/event"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.opentelemetry.io/contrib/instrumentation/go.mongodb.org/mongo-driver/mongo/otelmongo"
)

// MongoFlags setup CLI flags for MongoDB.
func MongoFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:    "mongodb-uri",
			Usage:   "MongoDB connection string",
			EnvVars: []string{"MONGODB_URI"},
			Value:   "mongodb://mongoadmin:secret@localhost:27017",
		},
		&cli.Uint64Flag{
			Name:    "mongodb-minpool",
			Usage:   "MongoDB Min Pool Size",
			EnvVars: []string{"MONGODB_MIN_POOL"},
			Value:   10,
		},
		&cli.Uint64Flag{
			Name:    "mongodb-maxpool",
			Usage:   "MongoDB Max Pool Size",
			EnvVars: []string{"MONGODB_MAX_POOL"},
			Value:   30,
		},
	}
}

// BuildMongoConfig creates a MongoDB client.
func BuildMongoConfig(cliCtx *cli.Context) mongodb.Config {
	return mongodb.Config{
		URI:      cliCtx.String("mongodb-uri"),
		Database: "plugin",
		MinPool:  cliCtx.Uint64("mongodb-minpool"),
		MaxPool:  cliCtx.Uint64("mongodb-maxpool"),
	}
}

// CreateMongoClient creates a Mongo Client.
func CreateMongoClient(ctx context.Context, cfg mongodb.Config) (*mongodb.MongoDB, func(), error) {
	clientOptions := options.Client().ApplyURI(cfg.URI)
	clientOptions.SetSocketTimeout(2 * time.Second).
		SetConnectTimeout(10 * time.Second).
		SetServerSelectionTimeout(10 * time.Second).
		SetPoolMonitor(&event.PoolMonitor{Event: poolMonitor}).
		SetMinPoolSize(cfg.MinPool).
		SetMaxPoolSize(cfg.MaxPool).
		SetMonitor(otelmongo.NewMonitor("mongodb"))

	mongoClient, err := mongo.NewClient(clientOptions)
	if err != nil {
		return nil, nil, err
	}

	if err = mongoClient.Connect(ctx); err != nil {
		return nil, nil, err
	}

	return mongodb.NewMongoDB(mongoClient.Database(cfg.Database)), func() { _ = mongoClient.Disconnect(ctx) }, nil
}

func poolMonitor(poolEvent *event.PoolEvent) {
	log.Debug().Interface("event", poolEvent).Msg("received pool event")
}
