package db

import (
	"context"
	"fmt"
	"strings"

	f "github.com/fauna/faunadb-go/v3/faunadb"
	"go.opentelemetry.io/otel/api/global"
	"go.opentelemetry.io/otel/api/trace"
)

const (
	collName           = "plugin"
	collNameHashSuffix = "-hash"
)

// PluginDB is a db interface for Plugin.
type PluginDB interface {
	Get(ctx context.Context, id string) (Plugin, error)
	Delete(ctx context.Context, id string) error
	Create(context.Context, Plugin) (Plugin, error)
	List(context.Context, Pagination) ([]Plugin, string, error)
	GetByName(context.Context, string) (Plugin, error)
	SearchByName(context.Context, string, Pagination) ([]Plugin, string, error)
	Update(context.Context, string, Plugin) (Plugin, error)

	DeleteHash(ctx context.Context, id string) error
	CreateHash(ctx context.Context, module, version, hash string) (PluginHash, error)
	GetHashByName(ctx context.Context, module, version string) (PluginHash, error)
}

// FaunaDB is a faunadb implementation.
type FaunaDB struct {
	client   *f.FaunaClient
	collName string
	tracer   trace.Tracer
}

// NewFaunaDB creates an FaunaDB.
func NewFaunaDB(client *f.FaunaClient) *FaunaDB {
	return &FaunaDB{
		client:   client,
		collName: collName,
		tracer:   global.Tracer("Database"),
	}
}

// Get gets a plugin from an id.
func (d *FaunaDB) Get(ctx context.Context, id string) (Plugin, error) {
	ctx, client, span := d.startSpan(ctx, "db_get")
	defer span.End()

	res, err := client.Query(f.Get(f.RefCollection(f.Collection(d.collName), id)))
	if err != nil {
		span.RecordError(ctx, err)
		return Plugin{}, fmt.Errorf("fauna error: %w", err)
	}

	return decodePlugin(res)
}

// Delete deletes a plugin.
func (d *FaunaDB) Delete(ctx context.Context, id string) error {
	ctx, client, span := d.startSpan(ctx, "db_delete")
	defer span.End()

	_, err := client.Query(
		f.Delete(
			f.RefCollection(f.Collection(d.collName), id),
		),
	)
	if err != nil {
		span.RecordError(ctx, err)
		return fmt.Errorf("fauna error: %w", err)
	}

	return nil
}

// Create creates a plugin in the db.
func (d *FaunaDB) Create(ctx context.Context, plugin Plugin) (Plugin, error) {
	ctx, client, span := d.startSpan(ctx, "db_create")
	defer span.End()

	id, err := client.Query(f.NewId())
	if err != nil {
		span.RecordError(ctx, err)
		return Plugin{}, fmt.Errorf("fauna error: %w", err)
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
		span.RecordError(ctx, err)
		return Plugin{}, fmt.Errorf("fauna error: %w", err)
	}

	return decodePlugin(res)
}

// List gets all the plugins.
func (d *FaunaDB) List(ctx context.Context, pagination Pagination) ([]Plugin, string, error) {
	ctx, client, span := d.startSpan(ctx, "db_list")
	defer span.End()

	paginateOptions := []f.OptionalParameter{f.Size(pagination.Size)}

	if len(pagination.Start) > 0 {
		nextPage, err := decodeNextPageList(pagination.Start)
		if err != nil {
			span.RecordError(ctx, err)
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
		span.RecordError(ctx, err)
		return nil, "", fmt.Errorf("fauna error: %w", err)
	}

	var after f.ArrayV
	_ = res.At(f.ObjKey("after")).Get(&after)

	next, err := encodeNextPageList(after)
	if err != nil {
		span.RecordError(ctx, err)
		return nil, "", fmt.Errorf("fauna error: %w", err)
	}

	var plugins []Plugin
	err = res.At(f.ObjKey("data")).Get(&plugins)
	if err != nil {
		span.RecordError(ctx, err)
		return nil, "", fmt.Errorf("fauna error: %w", err)
	}

	return plugins, next, nil
}

// GetByName gets a plugin by name.
func (d *FaunaDB) GetByName(ctx context.Context, value string) (Plugin, error) {
	ctx, client, span := d.startSpan(ctx, "db_getByName")
	defer span.End()

	res, err := client.Query(
		f.Get(
			f.MatchTerm(f.Index(d.collName+"_by_value"), value),
		),
	)
	if err != nil {
		span.RecordError(ctx, err)
		return Plugin{}, fmt.Errorf("fauna error: %w", err)
	}

	return decodePlugin(res)
}

// SearchByName returns a list of plugins matching the query.
func (d *FaunaDB) SearchByName(ctx context.Context, query string, pagination Pagination) ([]Plugin, string, error) {
	ctx, client, span := d.startSpan(ctx, "db_searchByName")
	defer span.End()

	paginateOptions := []f.OptionalParameter{f.Size(pagination.Size)}

	if len(pagination.Start) > 0 {
		nextPage, err := decodeNextPageSearch(pagination.Start)
		if err != nil {
			span.RecordError(ctx, err)
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
		span.RecordError(ctx, err)
		return nil, "", fmt.Errorf("fauna error: %w", err)
	}

	var after f.ArrayV
	_ = res.At(f.ObjKey("after")).Get(&after)

	next, err := encodeNextPageSearch(after)
	if err != nil {
		span.RecordError(ctx, err)
		return nil, "", fmt.Errorf("fauna error: %w", err)
	}

	var plugins []Plugin
	err = res.At(f.ObjKey("data")).Get(&plugins)
	if err != nil {
		span.RecordError(ctx, err)
		return nil, "", fmt.Errorf("fauna error: %w", err)
	}

	return plugins, next, nil
}

// Update Updates a plugin in the db.
func (d *FaunaDB) Update(ctx context.Context, id string, plugin Plugin) (Plugin, error) {
	ctx, client, span := d.startSpan(ctx, "db_update")
	defer span.End()

	res, err := client.Query(
		f.Replace(
			f.Select("ref", f.Get(f.RefCollection(f.Collection(d.collName), id))),
			f.Obj{"data": plugin},
		))
	if err != nil {
		span.RecordError(ctx, err)
		return Plugin{}, fmt.Errorf("fauna error: %w", err)
	}

	return decodePlugin(res)
}

// -- Hash

// CreateHash stores a plugin hash.
func (d *FaunaDB) CreateHash(ctx context.Context, module, version, hash string) (PluginHash, error) {
	ctx, client, span := d.startSpan(ctx, "db_createHash")
	defer span.End()

	id, err := client.Query(f.NewId())
	if err != nil {
		span.RecordError(ctx, err)
		return PluginHash{}, fmt.Errorf("fauna error: %w", err)
	}

	res, err := client.Query(f.Create(
		f.RefCollection(f.Collection(d.collName+collNameHashSuffix), id),
		f.Obj{
			"data": f.Merge(PluginHash{Name: module + "@" + version, Hash: hash}, f.Obj{
				"createdAt": f.Now(),
				"id":        id,
			}),
		}))
	if err != nil {
		span.RecordError(ctx, err)
		return PluginHash{}, fmt.Errorf("fauna error: %w", err)
	}

	return decodePluginHash(res)
}

// GetHashByName gets a plugin hash by plugin name.
func (d *FaunaDB) GetHashByName(ctx context.Context, module, version string) (PluginHash, error) {
	ctx, client, span := d.startSpan(ctx, "db_getHashByName")
	defer span.End()

	res, err := client.Query(
		f.Get(
			f.MatchTerm(f.Index(d.collName+collNameHashSuffix+"_by_value"), module+"@"+version),
		),
	)
	if err != nil {
		span.RecordError(ctx, err)
		return PluginHash{}, fmt.Errorf("fauna error: %w", err)
	}

	return decodePluginHash(res)
}

// DeleteHash deletes a plugin hash.
func (d *FaunaDB) DeleteHash(ctx context.Context, id string) error {
	ctx, client, span := d.startSpan(ctx, "db_deleteHash")
	defer span.End()

	_, err := client.Query(
		f.Delete(
			f.RefCollection(f.Collection(d.collName+collNameHashSuffix), id),
		),
	)
	if err != nil {
		span.RecordError(ctx, err)
		return fmt.Errorf("fauna error: %w", err)
	}

	return nil
}

func decodePlugin(obj f.Value) (Plugin, error) {
	var plugin *Plugin
	if err := obj.At(f.ObjKey("data")).Get(&plugin); err != nil {
		return Plugin{}, err
	}

	return *plugin, nil
}

func decodePluginHash(obj f.Value) (PluginHash, error) {
	var plugin *PluginHash
	if err := obj.At(f.ObjKey("data")).Get(&plugin); err != nil {
		return PluginHash{}, err
	}

	return *plugin, nil
}
