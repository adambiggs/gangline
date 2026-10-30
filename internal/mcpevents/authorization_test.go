package mcpevents

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestAuthorizationErrorPreservesSubscriptionAndBlocksOperations(t *testing.T) {
	m, now, _ := testManager(t)
	id := subscribe(t, m, "p", key('a'), nil)
	raw, err := os.ReadFile(m.config.StatePath)
	if err != nil {
		t.Fatal(err)
	}
	lookupErr := errors.New("registration unavailable")
	m.config.Authorize = func(context.Context, string) (bool, error) { return true, lookupErr }
	calls := 0
	m.client.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return response(202, ""), nil
	})
	ctx := context.Background()
	for _, method := range []string{"events/list", "events/subscribe", "events/unsubscribe"} {
		if _, err := m.RPC(ctx, "p", method, request(key('a'), nil)); !errors.Is(err, lookupErr) {
			t.Fatalf("%s authorization error = %v", method, err)
		}
	}
	if active, err := m.ActivePrincipals(ctx); !errors.Is(err, lookupErr) || len(active) != 0 {
		t.Fatalf("indeterminate authorization returned active principals: %v, %v", active, err)
	}
	if err := m.Deliver(ctx, "p", "mail", *now); !errors.Is(err, lookupErr) {
		t.Fatalf("delivery authorization error = %v", err)
	}
	if calls != 0 || len(m.state.Subscriptions) != 1 || len(m.state.Subscriptions[id].Receipts) != 0 {
		t.Fatalf("authorization failure changed delivery state: calls=%d subscriptions=%v", calls, m.state.Subscriptions)
	}
	if saved, err := os.ReadFile(m.config.StatePath); err != nil || !bytes.Equal(raw, saved) {
		t.Fatalf("authorization failure changed persisted subscriptions: %s, %v", saved, err)
	}
	m.config.Authorize = func(context.Context, string) (bool, error) { return true, nil }
	if active, err := m.ActivePrincipals(ctx); err != nil || len(active) != 1 || active[0] != "p" {
		t.Fatalf("restored authorization lost subscription: %v, %v", active, err)
	}
	if err := m.Deliver(ctx, "p", "mail", *now); err != nil || calls != 1 {
		t.Fatalf("restored authorization failed delivery: calls=%d, %v", calls, err)
	}
}

func TestAuthorizationErrorAfterCallbackVerificationBlocksSubscription(t *testing.T) {
	m, _, _ := testManager(t)
	lookupErr := errors.New("registration unavailable")
	var authorizationErr error
	m.config.Authorize = func(context.Context, string) (bool, error) { return true, authorizationErr }
	m.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var body struct {
			Challenge string `json:"challenge"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		authorizationErr = lookupErr
		return response(200, `{"challenge":"`+body.Challenge+`"}`), nil
	})
	if _, err := m.RPC(context.Background(), "p", "events/subscribe", request(key('a'), nil)); !errors.Is(err, lookupErr) {
		t.Fatalf("post-verification authorization error = %v", err)
	}
	if len(m.state.Subscriptions) != 0 {
		t.Fatal("indeterminate authorization created subscription")
	}
}

func TestAuthorizationErrorStopsDeliveryRetry(t *testing.T) {
	m, now, _ := testManager(t)
	id := subscribe(t, m, "p", key('a'), nil)
	lookupErr := errors.New("registration unavailable")
	var authorizationErr error
	m.config.Authorize = func(context.Context, string) (bool, error) { return true, authorizationErr }
	calls := 0
	m.client.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return response(503, ""), nil
	})
	m.wait = func(context.Context, time.Duration) error {
		authorizationErr = lookupErr
		return nil
	}
	if err := m.Deliver(context.Background(), "p", "mail", *now); !errors.Is(err, lookupErr) {
		t.Fatalf("retry authorization error = %v", err)
	}
	if calls != 1 || len(m.state.Subscriptions[id].Receipts) != 0 {
		t.Fatalf("authorization failure retried or settled delivery: calls=%d receipts=%v", calls, m.state.Subscriptions[id].Receipts)
	}
}
