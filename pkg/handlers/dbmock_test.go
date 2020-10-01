package handlers

import (
	f "github.com/fauna/faunadb-go/v3/faunadb"
	"github.com/traefik/plugin-service/pkg/db"
)

type mockDB struct {
	getFn          func(id string) (db.Plugin, error)
	deleteFn       func(id string) error
	createFn       func(db.Plugin) (db.Plugin, error)
	listFn         func(db.Pagination) ([]db.Plugin, string, error)
	getByNameFn    func(string) (db.Plugin, error)
	searchByNameFn func(string, db.Pagination) ([]db.Plugin, string, error)
	updateFn       func(string, db.Plugin) (db.Plugin, error)

	deleteHashFn    func(id string) error
	createHashFn    func(module, version, hash string) (db.PluginHash, error)
	getHashByNameFn func(module, version string) (db.PluginHash, error)
}

func (m mockDB) Get(id string) (db.Plugin, error) {
	return m.getFn(id)
}

func (m mockDB) Delete(id string) error {
	return m.deleteFn(id)
}

func (m mockDB) Create(plugin db.Plugin) (db.Plugin, error) {
	return m.createFn(plugin)
}

func (m mockDB) List(pagination db.Pagination) ([]db.Plugin, string, error) {
	return m.listFn(pagination)
}

func (m mockDB) GetByName(name string) (db.Plugin, error) {
	return m.getByNameFn(name)
}

func (m mockDB) SearchByName(query string, pagination db.Pagination) ([]db.Plugin, string, error) {
	return m.searchByNameFn(query, pagination)
}

func (m mockDB) Update(id string, plugin db.Plugin) (db.Plugin, error) {
	return m.updateFn(id, plugin)
}

func (m mockDB) DeleteHash(id string) error {
	return m.deleteHashFn(id)
}

func (m mockDB) CreateHash(module, version, hash string) (db.PluginHash, error) {
	return m.createHashFn(module, version, hash)
}

func (m mockDB) GetHashByName(module, version string) (db.PluginHash, error) {
	return m.getHashByNameFn(module, version)
}

type faunaNotFoundError struct{}

func (f faunaNotFoundError) Error() string {
	return "not found error"
}

func (f faunaNotFoundError) Status() int {
	return 404
}

func (f faunaNotFoundError) Errors() []f.QueryError {
	return nil
}
