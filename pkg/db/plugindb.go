package db

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	f "github.com/fauna/faunadb-go/v3/faunadb"
	"go.opentelemetry.io/otel/api/global"
	"go.opentelemetry.io/otel/api/trace"
	"go.opentelemetry.io/otel/label"
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

// NextPageList represents a pagination header value.
type NextPageList struct {
	Stars  int    `json:"stars"`
	NextID string `json:"nextId"`
}

// NextPageSearch represents a pagination header value for the search request.
type NextPageSearch struct {
	Name   string `json:"name"`
	NextID string `json:"nextId"`
}

// FaunaTracing is the fauna tracing configuration.
type FaunaTracing struct {
	Endpoint string
	Username string
	Password string
}

// FaunaDB is a faunadb implementation.
type FaunaDB struct {
	client   *f.FaunaClient
	collName string
	tracer   trace.Tracer
	tracing  FaunaTracing
}

// Pagination is a configuration struct for pagination.
type Pagination struct {
	Start string
	Size  int
}

// NewFaunaDB creates an FaunaDB.
func NewFaunaDB(client *f.FaunaClient, tracing FaunaTracing) *FaunaDB {
	return &FaunaDB{
		client:   client,
		collName: collName,
		tracer:   global.Tracer("Database"),
		tracing:  tracing,
	}
}

// Get gets a plugin from an id.
func (d *FaunaDB) Get(ctx context.Context, id string) (Plugin, error) {
	client, span := d.startSpan(ctx, "db_get")
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
	client, span := d.startSpan(ctx, "db_delete")
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
	client, span := d.startSpan(ctx, "db_create")
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
	client, span := d.startSpan(ctx, "db_list")
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
	client, span := d.startSpan(ctx, "db_getByName")
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
	client, span := d.startSpan(ctx, "db_searchByName")
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
	client, span := d.startSpan(ctx, "db_update")
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
	client, span := d.startSpan(ctx, "db_createHash")
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
	client, span := d.startSpan(ctx, "db_getHashByName")
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
	client, span := d.startSpan(ctx, "db_deleteHash")
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

// Observe sends trace for Fauna requests.
func Observe(ctx context.Context, tracer trace.Tracer) f.ObserverCallback {
	return func(result *f.QueryResult) {
		_, span := tracer.Start(ctx, "fauna_"+strings.SplitN(result.Query.String(), "(", 2)[0], trace.WithTimestamp(result.StartTime))
		defer span.End(trace.WithTimestamp(result.EndTime))

		attributes := []label.KeyValue{
			{Key: label.Key("fauna.request"), Value: label.StringValue(result.Query.String())},
		}

		for key, value := range result.Headers {
			attributeName := strings.ReplaceAll(key, "X-", "fauna.")
			attributeName = strings.ReplaceAll(attributeName, "-", ".")

			if len(value) > 0 {
				attributes = append(attributes, label.KeyValue{
					Key:   label.Key(attributeName),
					Value: label.StringValue(value[0]),
				})
			}
		}
		span.SetAttributes(attributes...)
	}
}

// startSpan starts a new span for tracing and start a Fauna client with a new Observer function.
func (d *FaunaDB) startSpan(ctx context.Context, name string) (f.FaunaClient, trace.Span) {
	ctx, span := d.tracer.Start(ctx, name)

	client := d.client.NewWithObserver(Observe(ctx, d.tracer))

	return *client, span
}

// Bootstrap create collection and indexes if not present.
func (d *FaunaDB) Bootstrap() error {
	ref, err := d.createCollection(d.collName + collNameHashSuffix)
	if err != nil {
		return err
	}

	err = d.createIndexes(d.collName+collNameHashSuffix, ref)
	if err != nil {
		return err
	}

	ref, err = d.createCollection(d.collName)
	if err != nil {
		return err
	}
	return d.createIndexes(d.collName, ref)
}

func (d *FaunaDB) createCollection(collName string) (f.RefV, error) {
	result, err := d.client.Query(f.Paginate(f.Collections()))
	if err != nil {
		return f.RefV{}, err
	}

	var refs []f.RefV
	err = result.At(f.ObjKey("data")).Get(&refs)
	if err != nil {
		return f.RefV{}, err
	}

	exists, collRes := contains(collName, refs)

	if !exists {
		var err error
		collRes, err = d.queryForRef(f.CreateCollection(f.Obj{"name": collName}))
		if err != nil {
			return f.RefV{}, err
		}
	}

	return collRes, nil
}

func (d *FaunaDB) queryForRef(expr f.Expr) (f.RefV, error) {
	var ref f.RefV
	value, err := d.client.Query(expr)
	if err != nil {
		return ref, err
	}

	err = value.At(f.ObjKey("ref")).Get(&ref)
	return ref, err
}

func (d *FaunaDB) createIndex(indexName string, collRes f.RefV, terms, values f.Arr) error {
	_, err := d.client.Query(
		f.CreateIndex(f.Obj{
			"name":   indexName,
			"active": true,
			"source": collRes,
			"terms":  terms,
			"values": values,
		}),
	)
	if err != nil {
		return fmt.Errorf("fauna error: %w", err)
	}

	return nil
}

func (d *FaunaDB) createIndexes(collName string, collRes f.RefV) error {
	resultIdx, _ := d.client.Query(f.Paginate(f.Indexes()))

	var refsIdx []f.RefV
	errIdx := resultIdx.At(f.ObjKey("data")).Get(&refsIdx)
	if errIdx != nil {
		return errIdx
	}

	idxName := collName + "_by_value"
	exists, _ := contains(idxName, refsIdx)
	if !exists {
		err := d.createIndex(idxName, collRes, f.Arr{f.Obj{
			"field": f.Arr{"data", "name"},
		}}, nil)
		if err != nil {
			return err
		}
	}

	idxName = collName + "_sort_by_stars"
	exists, _ = contains(idxName, refsIdx)
	if !exists {
		err := d.createIndex(idxName, collRes, nil, f.Arr{
			f.Obj{"field": f.Arr{"data", "stars"}, "reverse": true},
			f.Obj{"field": f.Arr{"ref"}},
		})
		if err != nil {
			return err
		}
	}

	idxName = collName + "_sort_by_display_name"
	exists, _ = contains(idxName, refsIdx)
	if !exists {
		err := d.createIndex(idxName, collRes, nil, f.Arr{
			f.Obj{"field": f.Arr{"data", "displayName"}},
			f.Obj{"field": f.Arr{"ref"}},
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func contains(s string, idx []f.RefV) (bool, f.RefV) {
	for _, ref := range idx {
		if ref.ID == s {
			return true, ref
		}
	}

	return false, f.RefV{}
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

func encodeNextPageList(after f.ArrayV) (string, error) {
	if len(after) <= 2 {
		return "", nil
	}

	var nextN int
	err := after[0].Get(&nextN)
	if err != nil {
		return "", err
	}

	var next f.RefV
	err = after[1].Get(&next)
	if err != nil {
		return "", err
	}

	nextPage := NextPageList{NextID: next.ID, Stars: nextN}

	b, err := json.Marshal(nextPage)
	if err != nil {
		return "", err
	}

	return base64.RawStdEncoding.EncodeToString(b), nil
}

func decodeNextPageList(data string) (NextPageList, error) {
	decodeString, err := base64.RawStdEncoding.DecodeString(data)
	if err != nil {
		return NextPageList{}, err
	}

	var nextPage NextPageList
	err = json.Unmarshal(decodeString, &nextPage)
	if err != nil {
		return NextPageList{}, err
	}
	return nextPage, nil
}

func encodeNextPageSearch(after f.ArrayV) (string, error) {
	if len(after) <= 2 {
		return "", nil
	}

	var nextN string
	err := after[0].Get(&nextN)
	if err != nil {
		return "", err
	}

	var next f.RefV
	err = after[1].Get(&next)
	if err != nil {
		return "", err
	}

	nextPage := NextPageSearch{NextID: next.ID, Name: nextN}

	b, err := json.Marshal(nextPage)
	if err != nil {
		return "", err
	}

	return base64.RawStdEncoding.EncodeToString(b), nil
}

func decodeNextPageSearch(data string) (NextPageSearch, error) {
	decodeString, err := base64.RawStdEncoding.DecodeString(data)
	if err != nil {
		return NextPageSearch{}, err
	}

	var nextPage NextPageSearch
	err = json.Unmarshal(decodeString, &nextPage)
	if err != nil {
		return NextPageSearch{}, err
	}
	return nextPage, nil
}
