package db

import (
	"encoding/base64"
	"encoding/json"
	"strings"

	f "github.com/fauna/faunadb-go/v3/faunadb"
)

const (
	collName           = "plugin"
	collNameHashSuffix = "-hash"
)

// PluginDB is a db interface for Plugin.
type PluginDB interface {
	Get(id string) (Plugin, error)
	Delete(id string) error
	Create(Plugin) (Plugin, error)
	List(Pagination) ([]Plugin, string, error)
	GetByName(string) (Plugin, error)
	SearchByName(string, Pagination) ([]Plugin, string, error)
	Update(string, Plugin) (Plugin, error)

	DeleteHash(id string) error
	CreateHash(module, version, hash string) (PluginHash, error)
	GetHashByName(module, version string) (PluginHash, error)
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

// FaunaDB is a faunadb implementation.
type FaunaDB struct {
	client   *f.FaunaClient
	collName string
}

// Pagination is a configuration struct for pagination.
type Pagination struct {
	Start string
	Size  int
}

// NewFaunaDB creates an FaunaDB.
func NewFaunaDB(client *f.FaunaClient) *FaunaDB {
	return &FaunaDB{
		client:   client,
		collName: collName,
	}
}

// Get gets a plugin from an id.
func (d *FaunaDB) Get(id string) (Plugin, error) {
	res, err := d.client.Query(f.Get(f.RefCollection(f.Collection(d.collName), id)))
	if err != nil {
		return Plugin{}, err
	}

	return decodePlugin(res)
}

// Delete deletes a plugin.
func (d *FaunaDB) Delete(id string) error {
	_, err := d.client.Query(
		f.Delete(
			f.RefCollection(f.Collection(d.collName), id),
		),
	)
	if err != nil {
		return err
	}

	return nil
}

// Create creates a plugin in the db.
func (d *FaunaDB) Create(plugin Plugin) (Plugin, error) {
	id, err := d.client.Query(f.NewId())
	if err != nil {
		return Plugin{}, err
	}

	res, err := d.client.Query(f.Create(
		f.RefCollection(f.Collection(d.collName), id),
		f.Obj{
			"data": f.Merge(plugin, f.Obj{
				"createdAt": f.Now(),
				"id":        id,
			}),
		}))
	if err != nil {
		return Plugin{}, err
	}

	return decodePlugin(res)
}

// List gets all the plugins.
func (d *FaunaDB) List(pagination Pagination) ([]Plugin, string, error) {
	paginateOptions := []f.OptionalParameter{f.Size(pagination.Size)}

	if len(pagination.Start) > 0 {
		nextPage, err := decodeNextPageList(pagination.Start)
		if err != nil {
			return nil, "", err
		}

		paginateOptions = append(paginateOptions, f.After(f.Arr{
			nextPage.Stars,
			f.RefCollection(f.Collection(d.collName), nextPage.NextID),
			f.RefCollection(f.Collection(d.collName), nextPage.NextID),
		}))
	}

	res, err := d.client.Query(
		f.Map(
			f.Paginate(
				f.Match(f.Index(d.collName+"_sort_by_stars")),
				paginateOptions...,
			),
			f.Lambda(f.Arr{"stars", "ref"}, f.Select("data", f.Get(f.Var("ref")))),
		),
	)
	if err != nil {
		return nil, "", err
	}

	var after f.ArrayV
	_ = res.At(f.ObjKey("after")).Get(&after)

	next, err := encodeNextPageList(after)
	if err != nil {
		return nil, "", err
	}

	var plugins []Plugin
	err = res.At(f.ObjKey("data")).Get(&plugins)
	if err != nil {
		return nil, "", err
	}

	return plugins, next, nil
}

// GetByName gets a plugin by name.
func (d *FaunaDB) GetByName(value string) (Plugin, error) {
	res, err := d.client.Query(
		f.Get(
			f.MatchTerm(f.Index(d.collName+"_by_value"), value),
		),
	)
	if err != nil {
		return Plugin{}, err
	}

	return decodePlugin(res)
}

// SearchByName returns a list of plugins matching the query.
func (d *FaunaDB) SearchByName(query string, pagination Pagination) ([]Plugin, string, error) {
	paginateOptions := []f.OptionalParameter{f.Size(pagination.Size)}

	if len(pagination.Start) > 0 {
		nextPage, err := decodeNextPageSearch(pagination.Start)
		if err != nil {
			return nil, "", err
		}

		paginateOptions = append(paginateOptions, f.After(f.Arr{
			nextPage.Name,
			f.RefCollection(f.Collection(d.collName), nextPage.NextID),
			f.RefCollection(f.Collection(d.collName), nextPage.NextID),
		}))
	}

	res, err := d.client.Query(f.Map(
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
		return nil, "", err
	}

	var after f.ArrayV
	_ = res.At(f.ObjKey("after")).Get(&after)

	next, err := encodeNextPageSearch(after)
	if err != nil {
		return nil, "", err
	}

	var plugins []Plugin
	err = res.At(f.ObjKey("data")).Get(&plugins)
	if err != nil {
		return nil, "", err
	}

	return plugins, next, nil
}

// Update Updates a plugin in the db.
func (d *FaunaDB) Update(id string, plugin Plugin) (Plugin, error) {
	res, err := d.client.Query(
		f.Replace(
			f.Select("ref", f.Get(f.RefCollection(f.Collection(d.collName), id))),
			f.Obj{"data": plugin},
		))
	if err != nil {
		return Plugin{}, err
	}

	return decodePlugin(res)
}

// -- Hash

// CreateHash stores a plugin hash.
func (d *FaunaDB) CreateHash(module, version, hash string) (PluginHash, error) {
	id, err := d.client.Query(f.NewId())
	if err != nil {
		return PluginHash{}, err
	}

	res, err := d.client.Query(f.Create(
		f.RefCollection(f.Collection(d.collName+collNameHashSuffix), id),
		f.Obj{
			"data": f.Merge(PluginHash{Name: module + "@" + version, Hash: hash}, f.Obj{
				"createdAt": f.Now(),
				"id":        id,
			}),
		}))
	if err != nil {
		return PluginHash{}, err
	}

	return decodePluginHash(res)
}

// GetHashByName gets a plugin hash by plugin name.
func (d *FaunaDB) GetHashByName(module, version string) (PluginHash, error) {
	res, err := d.client.Query(
		f.Get(
			f.MatchTerm(f.Index(d.collName+collNameHashSuffix+"_by_value"), module+"@"+version),
		),
	)
	if err != nil {
		return PluginHash{}, err
	}

	return decodePluginHash(res)
}

// DeleteHash deletes a plugin hash.
func (d *FaunaDB) DeleteHash(id string) error {
	_, err := d.client.Query(
		f.Delete(
			f.RefCollection(f.Collection(d.collName+collNameHashSuffix), id),
		),
	)
	if err != nil {
		return err
	}

	return nil
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
	return err
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
