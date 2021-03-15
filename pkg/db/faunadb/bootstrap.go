package faunadb

import (
	"fmt"

	f "github.com/fauna/faunadb-go/v3/faunadb"
)

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

	indexes := map[string]struct {
		terms, values f.Arr
	}{
		"_by_value": {
			terms: f.Arr{f.Obj{
				"field": f.Arr{"data", "name"},
			}},
		},
		"_sort_by_stars": {
			values: f.Arr{
				f.Obj{"field": f.Arr{"data", "stars"}, "reverse": true},
				f.Obj{"field": f.Arr{"ref"}},
			},
		},
		"_sort_by_display_name": {
			values: f.Arr{
				f.Obj{"field": f.Arr{"data", "displayName"}},
				f.Obj{"field": f.Arr{"ref"}},
			},
		},
	}

	for suffix, data := range indexes {
		idxName := collName + suffix
		exists, _ := contains(idxName, refsIdx)
		if !exists {
			err := d.createIndex(idxName, collRes, data.terms, data.values)
			if err != nil {
				return err
			}
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
