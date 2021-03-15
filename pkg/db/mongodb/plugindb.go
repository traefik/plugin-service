package mongodb

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/traefik/plugin-service/pkg/db"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
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
	Hashes  []db.PluginHash    `bson:"hashes"`
}

// Get returns the plugin corresponding to the given ID.
func (m *MongoDB) Get(ctx context.Context, id string) (db.Plugin, error) {
	ctx, span := m.tracer.Start(ctx, "db_get")
	defer span.End()

	criteria := bson.D{
		{Key: "id", Value: id},
	}

	opts := &options.FindOneOptions{}
	opts.SetProjection(bson.D{{Key: "hashes", Value: 0}})

	var plugin db.Plugin

	if err := m.client.Collection(m.collName).FindOne(ctx, criteria, opts).Decode(&plugin); err != nil {
		span.RecordError(err)

		if errors.Is(err, mongo.ErrNoDocuments) {
			return db.Plugin{}, db.ErrNotFound{Err: err}
		}

		return db.Plugin{}, err
	}

	return plugin, nil
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
func (m *MongoDB) List(ctx context.Context, page db.Pagination) ([]db.Plugin, string, error) {
	ctx, span := m.tracer.Start(ctx, "db_create")
	defer span.End()

	criteria := bson.D{}

	if len(page.Start) > 0 {
		// page.Start represents a FaunaDB ID and we can't use the $gt operator on a string, it must be done
		// on an ObjectID. So, we first need to retrieve the corresponding MongoID.
		var firstPlugin pluginDocument

		pageCriteria := bson.D{{Key: "id", Value: page.Start}}

		if err := m.client.Collection(m.collName).FindOne(ctx, pageCriteria).Decode(&firstPlugin); err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				return nil, "", nil
			}

			return nil, "", fmt.Errorf("unable to retrieve first plugin: %w", err)
		}

		criteria = append(criteria, bson.E{
			Key: "_id",
			Value: bson.D{
				{Key: "$gte", Value: firstPlugin.MongoID},
			},
		})
	}

	opts := &options.FindOptions{}
	opts.SetLimit(int64(page.Size + 1))
	opts.SetProjection(bson.D{{Key: "hashes", Value: 0}})
	opts.SetSort(bson.D{{Key: "stars", Value: -1}})

	cursor, err := m.client.Collection(m.collName).Find(ctx, criteria, opts)
	if err != nil {
		return nil, "", fmt.Errorf("unable to find plugins: %w", err)
	}

	var plugins []db.Plugin
	if err = cursor.All(ctx, &plugins); err != nil {
		return nil, "", fmt.Errorf("unable to unmarshal plugins: %w", err)
	}

	var nextPage string

	if len(plugins) > page.Size {
		nextPage = plugins[page.Size].ID
		plugins = plugins[:page.Size]
	}

	return plugins, nextPage, nil
}

// GetByName gets the plugin with the given name.
func (m *MongoDB) GetByName(ctx context.Context, name string) (db.Plugin, error) {
	ctx, span := m.tracer.Start(ctx, "db_get_by_name")
	defer span.End()

	criteria := bson.D{
		{Key: "name", Value: name},
	}

	opts := &options.FindOneOptions{}
	opts.SetProjection(bson.D{{Key: "hashes", Value: 0}})

	var plugin db.Plugin

	if err := m.client.Collection(m.collName).FindOne(ctx, criteria, opts).Decode(&plugin); err != nil {
		span.RecordError(err)

		if errors.Is(err, mongo.ErrNoDocuments) {
			return db.Plugin{}, db.ErrNotFound{Err: err}
		}

		return db.Plugin{}, err
	}

	return plugin, nil
}

// SearchByName searches for plugins matching with the given name.
func (m *MongoDB) SearchByName(ctx context.Context, name string, page db.Pagination) ([]db.Plugin, string, error) {
	ctx, span := m.tracer.Start(ctx, "db_search_by_name")
	defer span.End()

	criteria := bson.D{
		{Key: "displayName", Value: primitive.Regex{Pattern: regexp.QuoteMeta(name), Options: "i"}},
	}

	if len(page.Start) > 0 {
		nextPage, err := decodeNextPage(page.Start)
		if err != nil {
			span.RecordError(err)

			return nil, "", fmt.Errorf("unable to decode next page cursor: %w", err)
		}

		// nextID represents a FaunaDB ID and we can't use the $gt operator on a string, it must be done
		// on an ObjectID. So, we first need to retrieve the corresponding MongoID.
		var firstPlugin pluginDocument

		err = m.client.Collection(m.collName).
			FindOne(ctx, bson.D{{Key: "id", Value: nextPage.NextID}}).
			Decode(&firstPlugin)

		if err != nil {
			span.RecordError(err)

			if errors.Is(err, mongo.ErrNoDocuments) {
				return nil, "", nil
			}

			return nil, "", fmt.Errorf("unable to retrieve first plugin: %w", err)
		}

		criteria = append(criteria, bson.E{Key: "_id", Value: bson.D{
			{Key: "$lte", Value: firstPlugin.MongoID},
		}})
	}

	opts := &options.FindOptions{}
	opts.SetLimit(int64(page.Size + 1))
	opts.SetSort(bson.D{{Key: "displayName", Value: 1}})

	cursor, err := m.client.Collection(collName).Find(ctx, criteria, opts)
	if err != nil {
		span.RecordError(err)

		return nil, "", fmt.Errorf("unable to find plugins: %w", err)
	}

	var plugins []db.Plugin
	if err = cursor.All(ctx, &plugins); err != nil {
		span.RecordError(err)

		return nil, "", fmt.Errorf("unable to unmarshal plugins: %w", err)
	}

	var nextPage string

	if len(plugins) > page.Size {
		nextPlugin := plugins[page.Size]
		plugins = plugins[:page.Size]

		nextPage, err = encodeNextPage(db.NextPage{Name: nextPlugin.Name, NextID: nextPlugin.ID})
		if err != nil {
			span.RecordError(err)

			return nil, "", fmt.Errorf("unable to build next page cursor: %w", err)
		}
	}

	return plugins, nextPage, nil
}

// Update updates the given plugin.
func (m *MongoDB) Update(ctx context.Context, id string, plugin db.Plugin) (db.Plugin, error) {
	ctx, span := m.tracer.Start(ctx, "db_update")
	defer span.End()

	var updated db.Plugin

	filter := bson.D{
		{Key: "id", Value: id},
	}

	update := bson.D{
		{Key: "$set", Value: plugin},
	}

	opts := &options.FindOneAndUpdateOptions{}
	opts.SetReturnDocument(options.After)

	if err := m.client.Collection(collName).FindOneAndUpdate(ctx, filter, update, opts).Decode(&updated); err != nil {
		span.RecordError(err)

		if errors.Is(err, mongo.ErrNoDocuments) {
			return db.Plugin{}, db.ErrNotFound{Err: err}
		}

		return db.Plugin{}, fmt.Errorf("unable to update plugin: %w", err)
	}

	return updated, nil
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

func encodeNextPage(page db.NextPage) (string, error) {
	b, err := json.Marshal(page)
	if err != nil {
		return "", err
	}

	return base64.RawStdEncoding.EncodeToString(b), nil
}

func decodeNextPage(cursor string) (db.NextPage, error) {
	decodeString, err := base64.RawStdEncoding.DecodeString(cursor)
	if err != nil {
		return db.NextPage{}, err
	}

	var nextPage db.NextPage

	if err = json.Unmarshal(decodeString, &nextPage); err != nil {
		return db.NextPage{}, err
	}

	return nextPage, nil
}
