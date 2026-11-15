package shopmcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/seifreed/tupinambacli/internal/client"
)

type memoryStore struct{}

func (memoryStore) LoadSession() (client.Session, error)           { return client.Session{}, nil }
func (memoryStore) SaveSession(client.Session) error               { return nil }
func (memoryStore) LoadCatalogCache() (client.CatalogCache, error) { return client.CatalogCache{}, nil }
func (memoryStore) SaveCatalogCache(client.CatalogCache) error     { return nil }

type shopFixture struct {
	mu                   sync.Mutex
	server               *httptest.Server
	quantity             int
	totalCents           int
	price                string
	fail                 map[string]int
	badCart              bool
	failAfterWrite       bool
	overBudgetAfterWrite bool
}

const productJSON = `{"id":10,"title":"Cafe Arabica","handle":"arabica","body_html":"<p>Single origin coffee</p>","product_type":"coffee","variants":[{"id":111,"title":"250g","sku":"ARA","available":true,"price":"%s"},{"id":112,"title":"Unavailable","sku":"OUT","available":false,"price":"8.00"}]}`

func newShopFixture(t *testing.T) *shopFixture {
	t.Helper()
	f := &shopFixture{price: "5.00", fail: map[string]int{}}
	f.server = httptest.NewServer(http.HandlerFunc(f.serveHTTP))
	t.Cleanup(f.server.Close)
	return f
}

func (f *shopFixture) client() *client.Client {
	return client.NewWithOptions(client.ClientOptions{StateStore: memoryStore{}, BaseURL: f.server.URL})
}

func (f *shopFixture) serveHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail[r.URL.Path] > 0 {
		f.fail[r.URL.Path]--
		http.Error(w, "remote failure", http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	switch r.URL.Path {
	case "/products/arabica.js":
		_, _ = fmt.Fprintf(w, productJSON, f.price)
	case "/products.json":
		_, _ = fmt.Fprintf(w, `{"products":[%s]}`, fmt.Sprintf(productJSON, f.price))
	case "/cart.js":
		if f.badCart {
			_, _ = w.Write([]byte(`{"item_count":"bad"}`))
			return
		}
		lines := []any{}
		count := 0
		if f.quantity > 0 {
			lines = append(lines, map[string]any{"variant_id": 111, "quantity": f.quantity})
			count = f.quantity
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"item_count": count, "total_price": f.totalCents, "currency": "EUR", "items": lines, "token": "must-not-leak"})
	case "/cart/add.js":
		var body struct {
			ID       string `json:"id"`
			Quantity int    `json:"quantity"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ID != "111" {
			http.Error(w, "invalid", http.StatusBadRequest)
			return
		}
		f.quantity += body.Quantity
		f.totalCents += body.Quantity * 500
		if f.failAfterWrite {
			f.fail["/cart.js"] = 1
			f.failAfterWrite = false
		}
		if f.overBudgetAfterWrite {
			f.totalCents = 1100
			f.overBudgetAfterWrite = false
		}
		_, _ = fmt.Fprintf(w, `{"ok":true,"quantity":%d}`, f.quantity)
	case "/cart/update.js":
		var body struct {
			Updates map[string]int `json:"updates"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid", http.StatusBadRequest)
			return
		}
		f.quantity = body.Updates["111"]
		f.totalCents = f.quantity * 500
		_, _ = fmt.Fprintf(w, `{"ok":true,"quantity":%d}`, f.quantity)
	default:
		http.NotFound(w, r)
	}
}

func (f *shopFixture) setFail(path string, count int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fail[path] = count
}

func (f *shopFixture) setCart(quantity, total int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.quantity, f.totalCents = quantity, total
}

func (f *shopFixture) setPrice(price string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.price = price
}

func connect(t *testing.T, service *Server) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	serverSession, err := service.MCP("test").Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	clientSession, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil).Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = clientSession.Close()
		_ = serverSession.Close()
	})
	return clientSession
}

func callTool(t *testing.T, session *mcp.ClientSession, name string, args any) *mcp.CallToolResult {
	t.Helper()
	b, err := json.Marshal(args)
	if err != nil {
		t.Fatal(err)
	}
	var arguments map[string]any
	if err := json.Unmarshal(b, &arguments); err != nil {
		t.Fatal(err)
	}
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: arguments})
	if err != nil {
		t.Fatalf("tool %s call: %v", name, err)
	}
	return result
}

func output[T any](t *testing.T, result *mcp.CallToolResult) T {
	t.Helper()
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	b, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var got T
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	return got
}
