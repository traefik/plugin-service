package handlers

import (
	"context"

	"github.com/traefik/plugin-service/pkg/db"
)

type mockDB struct {
	getFn          func(ctx context.Context, id string) (db.Plugin, error)
	deleteFn       func(ctx context.Context, id string) error
	createFn       func(context.Context, db.Plugin) (db.Plugin, error)
	listFn         func(context.Context, db.Pagination) ([]db.Plugin, string, error)
	getByNameFn    func(context.Context, string, bool) (db.Plugin, error)
	searchByNameFn func(context.Context, string, db.Pagination) ([]db.Plugin, string, error)
	updateFn       func(context.Context, string, db.Plugin) (db.Plugin, error)

	deleteHashFn    func(ctx context.Context, id string) error
	createHashFn    func(ctx context.Context, module, version, hash string) (db.PluginHash, error)
	getHashByNameFn func(ctx context.Context, module, version string) (db.PluginHash, error)
}

func (m mockDB) Get(ctx context.Context, id string) (db.Plugin, error) {
	return m.getFn(ctx, id)
}

func (m mockDB) Delete(ctx context.Context, id string) error {
	return m.deleteFn(ctx, id)
}

func (m mockDB) Create(ctx context.Context, plugin db.Plugin) (db.Plugin, error) {
	return m.createFn(ctx, plugin)
}

func (m mockDB) List(ctx context.Context, pagination db.Pagination) ([]db.Plugin, string, error) {
	return m.listFn(ctx, pagination)
}

func (m mockDB) GetByName(ctx context.Context, name string, _, filterHidden bool) (db.Plugin, error) {
	return m.getByNameFn(ctx, name, filterHidden)
}

func (m mockDB) SearchByName(ctx context.Context, query string, pagination db.Pagination) ([]db.Plugin, string, error) {
	return m.searchByNameFn(ctx, query, pagination)
}

func (m mockDB) Update(ctx context.Context, id string, plugin db.Plugin) (db.Plugin, error) {
	return m.updateFn(ctx, id, plugin)
}

func (m mockDB) DeleteHash(ctx context.Context, id string) error {
	return m.deleteHashFn(ctx, id)
}

func (m mockDB) CreateHash(ctx context.Context, module, version, hash string) (db.PluginHash, error) {
	return m.createHashFn(ctx, module, version, hash)
}

func (m mockDB) GetHashByName(ctx context.Context, module, version string) (db.PluginHash, error) {
	return m.getHashByNameFn(ctx, module, version)
}
