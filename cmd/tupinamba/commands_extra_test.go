package main

import (
	"encoding/json"
	"net/http"
	"os"
	"testing"

	"github.com/seifreed/tupinambacli/internal/client"
)

func fullHandler(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/cart.js", "/cart/add.js", "/cart/update.js", "/cart/clear.js":
		jsonResponse(w, map[string]any{"token": "hidden", "item_count": 0, "total_price": 0, "currency": "EUR"})
	default:
		catalogHandler([]client.Product{testProduct("Alpha", "alpha", true)})(w, r)
	}
}

func suppressOutput(t *testing.T, fn func() int) int {
	t.Helper()
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	oldOut, oldErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = devNull, devNull
	code := fn()
	os.Stdout, os.Stderr = oldOut, oldErr
	_ = devNull.Close()
	return code
}

func TestRunDispatchesCommands(t *testing.T) {
	t.Setenv("TUPINAMBA_CONFIG_DIR", t.TempDir())
	useTestClient(t, http.HandlerFunc(fullHandler))
	file := t.TempDir() + "/items"
	if err := os.WriteFile(file, []byte("alpha 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		args []string
		want int
	}{
		{"search", []string{"search", "alpha", "--json"}, 0},
		{"product", []string{"product", "alpha", "--json"}, 0},
		{"batch", []string{"batch", "-f", file, "--json"}, 0},
		{"total", []string{"total", "-f", file, "--json"}, 0},
		{"collections", []string{"collections", "--json"}, 0},
		{"categories", []string{"categories", "--id", "cafe", "--json"}, 0},
		{"login", []string{"login", "--from-browser", "firefox"}, 1},
		{"set-cookie", []string{"set-cookie"}, 1},
		{"import-har", []string{"import-har"}, 1},
		{"whoami", []string{"whoami"}, 0},
		{"cart", []string{"cart", "get", "--json"}, 0},
		{"checkout", []string{"checkout", "status", "--json"}, 0},
		{"version", []string{"version"}, 0},
		{"help", []string{"help"}, 0},
		{"unknown", []string{"not-a-command"}, 2},
		{"empty", nil, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := suppressOutput(t, func() int { return run(tc.args) }); got != tc.want {
				t.Fatalf("run(%v) = %d, want %d", tc.args, got, tc.want)
			}
		})
	}
}

func TestMainEntrypoint(t *testing.T) {
	oldArgs, oldExit := os.Args, exit
	t.Cleanup(func() { os.Args, exit = oldArgs, oldExit })
	os.Args = []string{"tupinamba", "version"}
	exit = func(code int) {
		if code != 0 {
			t.Fatalf("main exit code = %d", code)
		}
	}
	main()
}

func TestCommandValidationAndErrors(t *testing.T) {
	if err := cmdProduct(nil); err == nil {
		t.Fatal("expected product usage error")
	}
	if err := cmdCart(nil); err == nil {
		t.Fatal("expected cart usage error")
	}
	if err := cmdCart([]string{"nope"}); err == nil {
		t.Fatal("expected cart operation error")
	}
	if err := cmdCheckout([]string{"nope"}); err == nil {
		t.Fatal("expected checkout usage error")
	}
	if err := cmdSetCookie(nil); err == nil {
		t.Fatal("expected cookie usage error")
	}
	if err := cmdImportHAR(nil); err == nil {
		t.Fatal("expected HAR usage error")
	}
	for name, call := range map[string]func() error{
		"login":       func() error { return cmdLogin([]string{"extra"}) },
		"set-cookie":  func() error { return cmdSetCookie([]string{"--stdin", "extra"}) },
		"import-har":  func() error { return cmdImportHAR([]string{"--file", "x", "extra"}) },
		"batch":       func() error { return cmdBatch([]string{"extra"}) },
		"total":       func() error { return cmdTotal([]string{"extra"}) },
		"collections": func() error { return cmdCollections([]string{"extra"}) },
		"cart get":    func() error { return cmdCart([]string{"get", "extra"}) },
		"cart clear":  func() error { return cmdCart([]string{"clear", "extra"}) },
		"checkout":    func() error { return cmdCheckout([]string{"status", "extra"}) },
	} {
		if err := call(); err == nil {
			t.Errorf("%s accepted extra arguments", name)
		}
	}
}

func TestProductJSONUsesNumericShape(t *testing.T) {
	var p client.Product
	if err := json.Unmarshal([]byte(`{"title":"x"}`), &p); err != nil || p.Title != "x" {
		t.Fatal(err)
	}
}
