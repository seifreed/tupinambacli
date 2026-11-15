package desktopmcp

import (
	"context"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestConcurrentApplyIsSingleUse(t *testing.T) {
	fixture := newShopFixture(t)
	session := connect(t, New(fixture.client()))
	plan := output[PlanOutput](t, callTool(t, session, "plan_cart_change", planArgs("add", 1, "10.00")))
	args := map[string]any{"plan_id": plan.PlanID, "confirmation": plan.ConfirmationPhrase}
	results := make(chan *mcp.CallToolResult, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "apply_cart_change", Arguments: args})
			if err != nil {
				results <- &mcp.CallToolResult{IsError: true}
				return
			}
			results <- result
		}()
	}
	wg.Wait()
	close(results)
	success, failed := 0, 0
	for result := range results {
		if result.IsError {
			failed++
		} else {
			success++
		}
	}
	if success != 1 || failed != 1 || fixture.quantity != 1 {
		t.Fatalf("success=%d failed=%d quantity=%d", success, failed, fixture.quantity)
	}
}
