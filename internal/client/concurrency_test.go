package client

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestConcurrentRequestsMergeAndPersistCookiesSafely(t *testing.T) {
	store := &memoryStateStore{}
	const requests = 24
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		value := r.URL.Query().Get("id")
		http.SetCookie(w, &http.Cookie{Name: "cookie" + value, Value: value, Path: "/"})
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	c := &Client{httpClient: srv.Client(), baseURL: srv.URL, store: store}
	var wg sync.WaitGroup
	errs := make(chan error, requests)
	for i := 0; i < requests; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs <- c.doJSON(http.MethodGet, srv.URL+"/?id="+fmt.Sprint(i), nil, nil)
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if got := strings.Count(c.cookie, "cookie"); got != requests {
		t.Fatalf("in-memory cookie count = %d, want %d", got, requests)
	}
	if got := strings.Count(store.session.Cookie, "cookie"); got != requests {
		t.Fatalf("persisted cookie count = %d, want %d", got, requests)
	}
}
