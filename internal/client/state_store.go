package client

type StateStore interface {
	LoadSession() (Session, error)
	SaveSession(Session) error
	LoadCatalogCache() (CatalogCache, error)
	SaveCatalogCache(CatalogCache) error
}
