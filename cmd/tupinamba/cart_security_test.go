package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestCartOutputRedactsToken(t *testing.T) {
	useTestClient(t, http.HandlerFunc(fullHandler))
	for _, args := range [][]string{
		{"get"}, {"get", "--json"}, {"get", "--toon"},
		{"add", "alpha", "1"}, {"set", "alpha", "1", "--toon"}, {"clear", "--json"},
	} {
		out, err := captureStdout(t, func() error { return cmdCart(args) })
		if err != nil {
			t.Fatalf("cmdCart(%v): %v", args, err)
		}
		if strings.Contains(out, "token") || strings.Contains(out, "hidden") || !strings.Contains(out, "item_count") {
			t.Fatalf("cmdCart(%v) exposed or lost cart data: %s", args, out)
		}
	}
	if _, err := finishCart(json.RawMessage(`[]`), nil, outputFlags{}); err == nil {
		t.Fatal("accepted a non-object cart response")
	}
	if _, err := finishCart(json.RawMessage(`null`), nil, outputFlags{}); err == nil {
		t.Fatal("accepted a null cart response")
	}
}
