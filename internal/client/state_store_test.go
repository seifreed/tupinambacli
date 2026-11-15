package client

import (
	"errors"
	"os"
	"sync"
)

type memoryStateStore struct {
	mu             sync.Mutex
	session        Session
	sessionLoadErr error
	sessionSaveErr error
	catalog        CatalogCache
	hasCatalog     bool
	catalogLoadErr error
	catalogSaveErr error
}

func (s *memoryStateStore) LoadSession() (Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.session, s.sessionLoadErr
}

func (s *memoryStateStore) SaveSession(session Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sessionSaveErr != nil {
		return s.sessionSaveErr
	}
	s.session = session
	return nil
}

func (s *memoryStateStore) LoadCatalogCache() (CatalogCache, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.catalogLoadErr != nil {
		return CatalogCache{}, s.catalogLoadErr
	}
	if !s.hasCatalog {
		return CatalogCache{}, os.ErrNotExist
	}
	return s.catalog, nil
}

func (s *memoryStateStore) SaveCatalogCache(cache CatalogCache) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.catalogSaveErr != nil {
		return s.catalogSaveErr
	}
	s.catalog = cache
	s.hasCatalog = true
	return nil
}

var _ StateStore = (*memoryStateStore)(nil)

var errTestStore = errors.New("test state store error")
