package db

import f "github.com/fauna/faunadb-go/faunadb"

const collName = "plugin"
const collNameHashSuffix = "-hash"

// PluginDB is a db interface for Plugin.
type PluginDB interface {
	Get(id string) (Plugin, error)
	Delete(id string) error
	Create(Plugin) (Plugin, error)
	List(Pagination) ([]Plugin, string, error)
	GetByName(string) (Plugin, error)
	Update(string, Plugin) (Plugin, error)

	DeleteHash(id string) error
	CreateHash(module string, version string, hash string) (PluginHash, error)
	GetHashByName(module string, version string) (PluginHash, error)
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
	return d.list(f.Documents(f.Collection(d.collName)), pagination)
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

// Update Updates a plugin in the db.
func (d *FaunaDB) Update(id string, plugin Plugin) (Plugin, error) {
	res, err := d.client.Query(
		f.Update(
			f.Select("ref", f.Get(f.RefCollection(f.Collection(d.collName), id))),
			f.Obj{"data": plugin},
		))

	if err != nil {
		return Plugin{}, err
	}

	return decodePlugin(res)
}

func (d *FaunaDB) list(expr f.Expr, pagination Pagination) ([]Plugin, string, error) {
	paginateOptions := []f.OptionalParameter{f.Size(pagination.Size)}
	if len(pagination.Start) > 0 {
		paginateOptions = append(paginateOptions, f.After(f.RefCollection(f.Collection(d.collName), pagination.Start)))
	}

	res, err := d.client.Query(
		f.Map(
			f.Paginate(
				expr,
				paginateOptions...,
			),
			f.Lambda("id", f.Select("data", f.Get(f.Var("id")))),
		),
	)
	if err != nil {
		return nil, "", err
	}

	var after []f.RefV
	at := res.At(f.ObjKey("after"))
	_ = at.Get(&after)
	var next string
	if len(after) == 1 {
		next = after[0].ID
	}

	var plugins []Plugin
	err = res.At(f.ObjKey("data")).Get(&plugins)
	if err != nil {
		return nil, "", err
	}

	return plugins, next, nil
}

// -- Hash

// CreateHash stores a plugin hash.
func (d *FaunaDB) CreateHash(module string, version string, hash string) (PluginHash, error) {
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
func (d *FaunaDB) GetHashByName(module string, version string) (PluginHash, error) {
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

func (d *FaunaDB) createIndex(indexName string, collRes f.RefV, terms f.Arr, values f.Arr) error {
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
