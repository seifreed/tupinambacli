package client

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestNewWithOptionsRejectsMissingStateStore(t *testing.T) {
	if _, err := NewWithOptions(ClientOptions{}).Cart(); err == nil || err.Error() != "client state store is required" {
		t.Fatalf("expected missing state store error, got %v", err)
	}
}

func TestDoJSONLogsCanceledTransport(t *testing.T) {
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	c := &Client{httpClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, context.Canceled
	})}}
	if err := c.doJSON(http.MethodGet, "https://example.com", nil, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
	if !strings.Contains(logs.String(), `"error_code":"canceled"`) {
		t.Fatalf("missing cancellation log: %s", logs.String())
	}
}

func TestAllProductsFallsBackFromStaleCacheAndIgnoresSaveFailure(t *testing.T) {
	store := &memoryStateStore{
		catalog:        CatalogCache{SavedAt: time.Now().Add(-11 * time.Minute)},
		hasCatalog:     true,
		catalogSaveErr: errTestStore,
	}
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"products":[]}`)),
			Request:    req,
		}, nil
	})
	c := &Client{httpClient: &http.Client{Transport: transport}, baseURL: BaseURL, store: store}
	products, err := c.AllProducts()
	if err != nil || len(products) != 0 {
		t.Fatalf("products=%v err=%v", products, err)
	}
}
