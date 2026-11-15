package main

import (
	"github.com/seifreed/tupinambacli/internal/client"
	"github.com/seifreed/tupinambacli/internal/config"
)

const sessionFileName = "session.json"
const catalogCacheFileName = "catalog.json"

type fileStateStore struct{}

func newConfiguredClient() *client.Client { return client.New(fileStateStore{}) }

func (fileStateStore) LoadSession() (client.Session, error) {
	var session client.Session
	err := config.Load(sessionFileName, &session)
	return session, err
}

func (fileStateStore) SaveSession(session client.Session) error {
	return config.Save(sessionFileName, session)
}

func (fileStateStore) LoadCatalogCache() (client.CatalogCache, error) {
	var cache client.CatalogCache
	err := config.Load(catalogCacheFileName, &cache)
	return cache, err
}

func (fileStateStore) SaveCatalogCache(cache client.CatalogCache) error {
	return config.Save(catalogCacheFileName, cache)
}
