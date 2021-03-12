package faunadb

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	f "github.com/fauna/faunadb-go/v3/faunadb"
	"github.com/traefik/plugin-service/pkg/db"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

const (
	collName           = "plugin"
	collNameHashSuffix = "-hash"
)

// FaunaDB provides facilities for interacting with faunadb.
type FaunaDB struct {
	client       *f.FaunaClient
	collName     string
	tracer       trace.Tracer
	pingClient   *http.Client
	pingEndpoint string
}

// NewFaunaDB creates an FaunaDB.
func NewFaunaDB(client *f.FaunaClient) *FaunaDB {
	return &FaunaDB{
		client:       client,
		collName:     collName,
		tracer:       otel.Tracer("Database"),
		pingClient:   &http.Client{Timeout: 5 * time.Second},
		pingEndpoint: "https://db.fauna.com/ping",
	}
}

// Get gets a plugin from an id.
func (d *FaunaDB) Get(ctx context.Context, id string) (db.Plugin, error) {
	client, span := d.startSpan(ctx, "db_get")
	defer span.End()

	res, err := client.Query(f.Get(f.RefCollection(f.Collection(d.collName), id)))
	if err != nil {
		span.RecordError(err)

		if errors.As(err, &f.NotFound{}) {
			return db.Plugin{}, db.ErrNotFound{Err: err}
		}

		return db.Plugin{}, fmt.Errorf("fauna error: %w", err)
	}

	return decodePlugin(res)
}

// Delete deletes a plugin.
func (d *FaunaDB) Delete(ctx context.Context, id string) error {
	client, span := d.startSpan(ctx, "db_delete")
	defer span.End()

	_, err := client.Query(
		f.Delete(
			f.RefCollection(f.Collection(d.collName), id),
		),
	)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("fauna error: %w", err)
	}

	return nil
}

// Create creates a plugin in the db.
func (d *FaunaDB) Create(ctx context.Context, plugin db.Plugin) (db.Plugin, error) {
	client, span := d.startSpan(ctx, "db_create")
	defer span.End()

	id, err := client.Query(f.NewId())
	if err != nil {
		span.RecordError(err)
		return db.Plugin{}, fmt.Errorf("fauna error: %w", err)
	}

	res, err := client.Query(f.Create(
		f.RefCollection(f.Collection(d.collName), id),
		f.Obj{
			"data": f.Merge(plugin, f.Obj{
				"createdAt": f.Now(),
				"id":        id,
			}),
		}))
	if err != nil {
		span.RecordError(err)
		return db.Plugin{}, fmt.Errorf("fauna error: %w", err)
	}

	return decodePlugin(res)
}

// List gets all the plugins.
func (d *FaunaDB) List(ctx context.Context, pagination db.Pagination) ([]db.Plugin, string, error) {
	client, span := d.startSpan(ctx, "db_list")
	defer span.End()

	paginateOptions := []f.OptionalParameter{f.Size(pagination.Size)}

	if len(pagination.Start) > 0 {
		nextPage, err := decodeNextPageList(pagination.Start)
		if err != nil {
			span.RecordError(err)
			return nil, "", fmt.Errorf("fauna error: %w", err)
		}

		paginateOptions = append(paginateOptions, f.After(f.Arr{
			nextPage.Stars,
			f.RefCollection(f.Collection(d.collName), nextPage.NextID),
			f.RefCollection(f.Collection(d.collName), nextPage.NextID),
		}))
	}

	res, err := client.Query(
		f.Map(
			f.Paginate(
				f.Match(f.Index(d.collName+"_sort_by_stars")),
				paginateOptions...,
			),
			f.Lambda(f.Arr{"stars", "ref"}, f.Select("data", f.Get(f.Var("ref")))),
		),
	)
	if err != nil {
		span.RecordError(err)
		return nil, "", fmt.Errorf("fauna error: %w", err)
	}

	var after f.ArrayV
	_ = res.At(f.ObjKey("after")).Get(&after)

	next, err := encodeNextPageList(after)
	if err != nil {
		span.RecordError(err)
		return nil, "", fmt.Errorf("fauna error: %w", err)
	}

	var plugins []db.Plugin
	err = res.At(f.ObjKey("data")).Get(&plugins)
	if err != nil {
		span.RecordError(err)
		return nil, "", fmt.Errorf("fauna error: %w", err)
	}

	return plugins, next, nil
}

// GetByName gets a plugin by name.
func (d *FaunaDB) GetByName(ctx context.Context, value string) (db.Plugin, error) {
	client, span := d.startSpan(ctx, "db_getByName")
	defer span.End()

	res, err := client.Query(
		f.Get(
			f.MatchTerm(f.Index(d.collName+"_by_value"), value),
		),
	)
	if err != nil {
		span.RecordError(err)

		if errors.As(err, &f.NotFound{}) {
			return db.Plugin{}, db.ErrNotFound{Err: err}
		}

		return db.Plugin{}, fmt.Errorf("fauna error: %w", err)
	}

	return decodePlugin(res)
}

// SearchByName returns a list of plugins matching the query.
func (d *FaunaDB) SearchByName(ctx context.Context, query string, pagination db.Pagination) ([]db.Plugin, string, error) {
	client, span := d.startSpan(ctx, "db_searchByName")
	defer span.End()

	paginateOptions := []f.OptionalParameter{f.Size(pagination.Size)}

	if len(pagination.Start) > 0 {
		nextPage, err := decodeNextPageSearch(pagination.Start)
		if err != nil {
			span.RecordError(err)
			return nil, "", fmt.Errorf("fauna error: %w", err)
		}

		paginateOptions = append(paginateOptions, f.After(f.Arr{
			nextPage.Name,
			f.RefCollection(f.Collection(d.collName), nextPage.NextID),
			f.RefCollection(f.Collection(d.collName), nextPage.NextID),
		}))
	}

	res, err := client.Query(f.Map(
		f.Filter(
			f.Paginate(
				f.Match(f.Index(d.collName+"_sort_by_display_name")),
				paginateOptions...,
			),
			f.Lambda(f.Arr{"displayName", "ref"}, f.ContainsStr(f.LowerCase(f.Var("displayName")), strings.ToLower(query))),
		),
		f.Lambda(f.Arr{"displayName", "ref"}, f.Select(f.Arr{"data"}, f.Get(f.Var("ref")))),
	))
	if err != nil {
		span.RecordError(err)
		return nil, "", fmt.Errorf("fauna error: %w", err)
	}

	var after f.ArrayV
	_ = res.At(f.ObjKey("after")).Get(&after)

	next, err := encodeNextPageSearch(after)
	if err != nil {
		span.RecordError(err)
		return nil, "", fmt.Errorf("fauna error: %w", err)
	}

	var plugins []db.Plugin
	err = res.At(f.ObjKey("data")).Get(&plugins)
	if err != nil {
		span.RecordError(err)
		return nil, "", fmt.Errorf("fauna error: %w", err)
	}

	return plugins, next, nil
}

// Update Updates a plugin in the db.
func (d *FaunaDB) Update(ctx context.Context, id string, plugin db.Plugin) (db.Plugin, error) {
	client, span := d.startSpan(ctx, "db_update")
	defer span.End()

	res, err := client.Query(
		f.Replace(
			f.Select("ref", f.Get(f.RefCollection(f.Collection(d.collName), id))),
			f.Obj{"data": plugin},
		))
	if err != nil {
		span.RecordError(err)
		return db.Plugin{}, fmt.Errorf("fauna error: %w", err)
	}

	return decodePlugin(res)
}

// -- Hash

// CreateHash stores a plugin hash.
func (d *FaunaDB) CreateHash(ctx context.Context, module, version, hash string) (db.PluginHash, error) {
	client, span := d.startSpan(ctx, "db_createHash")
	defer span.End()

	id, err := client.Query(f.NewId())
	if err != nil {
		span.RecordError(err)
		return db.PluginHash{}, fmt.Errorf("fauna error: %w", err)
	}

	res, err := client.Query(f.Create(
		f.RefCollection(f.Collection(d.collName+collNameHashSuffix), id),
		f.Obj{
			"data": f.Merge(db.PluginHash{Name: module + "@" + version, Hash: hash}, f.Obj{
				"createdAt": f.Now(),
				"id":        id,
			}),
		}))
	if err != nil {
		span.RecordError(err)
		return db.PluginHash{}, fmt.Errorf("fauna error: %w", err)
	}

	return decodePluginHash(res)
}

// GetHashByName gets a plugin hash by plugin name.
func (d *FaunaDB) GetHashByName(ctx context.Context, module, version string) (db.PluginHash, error) {
	client, span := d.startSpan(ctx, "db_getHashByName")
	defer span.End()

	res, err := client.Query(
		f.Get(
			f.MatchTerm(f.Index(d.collName+collNameHashSuffix+"_by_value"), module+"@"+version),
		),
	)
	if err != nil {
		span.RecordError(err)
		return db.PluginHash{}, fmt.Errorf("fauna error: %w", err)
	}

	return decodePluginHash(res)
}

// DeleteHash deletes a plugin hash.
func (d *FaunaDB) DeleteHash(ctx context.Context, id string) error {
	client, span := d.startSpan(ctx, "db_deleteHash")
	defer span.End()

	_, err := client.Query(
		f.Delete(
			f.RefCollection(f.Collection(d.collName+collNameHashSuffix), id),
		),
	)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("fauna error: %w", err)
	}

	return nil
}

// Ping pings FaunaDB to check its health status.
func (d *FaunaDB) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.pingEndpoint, nil)
	if err != nil {
		return err
	}

	resp, err := d.pingClient.Do(req)
	if err != nil {
		return err
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("faunaDB didn't send a valid HTTP response code: %d", resp.StatusCode)
	}

	return nil
}

func decodePlugin(obj f.Value) (db.Plugin, error) {
	var plugin *db.Plugin
	if err := obj.At(f.ObjKey("data")).Get(&plugin); err != nil {
		return db.Plugin{}, err
	}

	return *plugin, nil
}

func decodePluginHash(obj f.Value) (db.PluginHash, error) {
	var plugin *db.PluginHash
	if err := obj.At(f.ObjKey("data")).Get(&plugin); err != nil {
		return db.PluginHash{}, err
	}

	return *plugin, nil
}
