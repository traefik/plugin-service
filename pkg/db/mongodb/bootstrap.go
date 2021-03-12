package mongodb

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MongoIndex represents a MongoDB index.
type MongoIndex struct {
	Key  bson.D
	Name string
	V    int32
}

// Bootstrap indexes if not present (collection is automatically created).
func (m *MongoDB) Bootstrap() error {
	models := []mongo.IndexModel{
		{
			Options: &options.IndexOptions{
				Name:   stringPtr("_uniq_id"),
				Unique: boolPtr(true),
			},
			Keys: bson.D{{Key: "id", Value: 1}},
		},
		{
			Options: &options.IndexOptions{
				Name: stringPtr("_by_stars"),
			},
			Keys: bson.D{{Key: "stars", Value: 1}},
		},
	}

	if _, err := m.client.Collection(m.collName).Indexes().CreateMany(context.Background(), models); err != nil {
		return fmt.Errorf("unable to create indexes: %w", err)
	}

	return nil
}

func stringPtr(val string) *string {
	return &val
}

func boolPtr(val bool) *bool {
	return &val
}
