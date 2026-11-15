package main

import (
	"testing"
	"time"

	"github.com/seifreed/tupinambacli/internal/client"
)

func TestFileStateStoreComposesSessionAndCatalogPersistence(t *testing.T) {
	t.Setenv("TUPINAMBA_CONFIG_DIR", t.TempDir())
	store := fileStateStore{}
	session := client.Session{Cookie: "session=value"}
	if err := store.SaveSession(session); err != nil {
		t.Fatal(err)
	}
	cache := client.CatalogCache{SavedAt: time.Now(), Products: []client.Product{{Title: "Cached"}}}
	if err := store.SaveCatalogCache(cache); err != nil {
		t.Fatal(err)
	}

	loadedSession, err := (fileStateStore{}).LoadSession()
	if err != nil || loadedSession.Cookie != session.Cookie {
		t.Fatalf("loaded session = %+v, err=%v", loadedSession, err)
	}
	loaded, err := store.LoadCatalogCache()
	if err != nil || len(loaded.Products) != 1 || loaded.Products[0].Title != "Cached" {
		t.Fatalf("loaded cache = %+v, err=%v", loaded, err)
	}
}
