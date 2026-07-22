package mongodb

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/traefik/plugin-service/pkg/db"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const blacklistCollName = "blacklist"

// ListBlacklist returns all the blacklisted repositories, sorted by repository.
func (m *MongoDB) ListBlacklist(ctx context.Context) ([]db.BlacklistEntry, error) {
	ctx, span := m.tracer.Start(ctx, "db_list_blacklist")
	defer span.End()

	opts := &options.FindOptions{}
	opts.SetSort(bson.D{{Key: "repository", Value: 1}})

	cursor, err := m.client.Collection(blacklistCollName).Find(ctx, bson.D{}, opts)
	if err != nil {
		span.RecordError(err)

		return nil, fmt.Errorf("unable to find blacklist entries: %w", err)
	}

	entries := []db.BlacklistEntry{}

	if err = cursor.All(ctx, &entries); err != nil {
		span.RecordError(err)

		return nil, fmt.Errorf("unable to unmarshal blacklist entries: %w", err)
	}

	return entries, nil
}

// UpsertBlacklist creates or updates a blacklist entry, keyed by repository.
// CreatedAt is only set on insert.
func (m *MongoDB) UpsertBlacklist(ctx context.Context, entry db.BlacklistEntry) (db.BlacklistEntry, error) {
	ctx, span := m.tracer.Start(ctx, "db_upsert_blacklist")
	defer span.End()

	filter := bson.D{{Key: "repository", Value: entry.Repository}}

	update := bson.D{
		{Key: "$set", Value: bson.D{
			{Key: "reason", Value: entry.Reason},
			{Key: "author", Value: entry.Author},
		}},
		{Key: "$setOnInsert", Value: bson.D{
			{Key: "repository", Value: entry.Repository},
			{Key: "createdAt", Value: time.Now().Truncate(time.Millisecond)},
		}},
	}

	opts := &options.FindOneAndUpdateOptions{}
	opts.SetUpsert(true)
	opts.SetReturnDocument(options.After)

	var updated db.BlacklistEntry
	if err := m.client.Collection(blacklistCollName).FindOneAndUpdate(ctx, filter, update, opts).Decode(&updated); err != nil {
		span.RecordError(err)

		return db.BlacklistEntry{}, fmt.Errorf("unable to upsert blacklist entry: %w", err)
	}

	return updated, nil
}

// DeleteBlacklist removes the blacklist entry for the given repository.
func (m *MongoDB) DeleteBlacklist(ctx context.Context, repository string) error {
	ctx, span := m.tracer.Start(ctx, "db_delete_blacklist")
	defer span.End()

	filter := bson.D{{Key: "repository", Value: repository}}

	res, err := m.client.Collection(blacklistCollName).DeleteOne(ctx, filter)
	if err != nil {
		span.RecordError(err)

		return err
	}

	if res.DeletedCount == 0 {
		return db.NotFoundError{Err: errors.New(repository)}
	}

	return nil
}
