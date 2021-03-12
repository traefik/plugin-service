package mongodb

import (
	"context"
	"errors"
	"time"

	"github.com/traefik/plugin-service/pkg/db"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

const collName = "plugin"

// MongoDB is a mongoDB client.
type MongoDB struct {
	client   *mongo.Database
	collName string
	tracer   trace.Tracer
}

// NewMongoDB creates a MongoDB.
func NewMongoDB(client *mongo.Database) *MongoDB {
	return &MongoDB{
		client:   client,
		collName: collName,
		tracer:   otel.Tracer("Database"),
	}
}

type pluginDocument struct {
	db.Plugin `bson:",inline"`

	MongoID primitive.ObjectID `bson:"_id,omitempty"`
	Hashes  []db.PluginHash
}

// Get returns the plugin corresponding to the given ID.
func (m *MongoDB) Get(ctx context.Context, id string) (db.Plugin, error) {
	ctx, span := m.tracer.Start(ctx, "db_get")
	defer span.End()

	criteria := bson.D{
		{Key: "id", Value: id},
	}

	var doc pluginDocument

	if err := m.client.Collection(m.collName).FindOne(ctx, criteria).Decode(&doc); err != nil {
		span.RecordError(err)

		if errors.Is(err, mongo.ErrNoDocuments) {
			return db.Plugin{}, db.ErrNotFound{Err: err}
		}

		return db.Plugin{}, err
	}

	return doc.Plugin, nil
}

// Delete deletes the plugin corresponding to the given ID.
func (m *MongoDB) Delete(ctx context.Context, id string) error {
	ctx, span := m.tracer.Start(ctx, "db_delete")
	defer span.End()

	criteria := bson.D{
		{Key: "id", Value: id},
	}

	res, err := m.client.Collection(m.collName).DeleteOne(ctx, criteria)
	if err != nil {
		span.RecordError(err)

		return err
	}

	if res.DeletedCount == 0 {
		return db.ErrNotFound{Err: err}
	}

	return nil
}

// Create creates a new plugin.
func (m *MongoDB) Create(ctx context.Context, plugin db.Plugin) (db.Plugin, error) {
	ctx, span := m.tracer.Start(ctx, "db_create")
	defer span.End()

	id := primitive.NewObjectID()
	plugin.ID = id.Hex()
	plugin.CreatedAt = time.Now().Truncate(time.Millisecond)

	doc := pluginDocument{
		Plugin:  plugin,
		MongoID: id,
	}

	_, err := m.client.Collection(m.collName).InsertOne(ctx, doc)
	if err != nil {
		span.RecordError(err)

		return db.Plugin{}, err
	}

	return plugin, nil
}

// List lists plugins.
func (m *MongoDB) List(ctx context.Context, pagination db.Pagination) ([]db.Plugin, string, error) {
	panic("implement me")
}

// GetByName gets the plugin with the given name.
func (m *MongoDB) GetByName(ctx context.Context, s string) (db.Plugin, error) {
	panic("implement me")
}

// SearchByName searches for plugins matching with the given name.
func (m *MongoDB) SearchByName(ctx context.Context, s string, pagination db.Pagination) ([]db.Plugin, string, error) {
	panic("implement me")
}

// Update updates the given plugin.
func (m *MongoDB) Update(ctx context.Context, id string, plugin db.Plugin) (db.Plugin, error) {
	panic("implement me")
}

// DeleteHash deletes a plugin hash.
func (m *MongoDB) DeleteHash(ctx context.Context, id string) error {
	panic("implement me")
}

// CreateHash creates a new plugin hash.
func (m *MongoDB) CreateHash(ctx context.Context, module, version, hash string) (db.PluginHash, error) {
	panic("implement me")
}

// GetHashByName returns the hash corresponding the given name.
func (m *MongoDB) GetHashByName(ctx context.Context, module, version string) (db.PluginHash, error) {
	panic("implement me")
}

// Ping pings MongoDB to check it health status.
func (m *MongoDB) Ping(ctx context.Context) error {
	return m.client.Client().Ping(ctx, nil)
}
