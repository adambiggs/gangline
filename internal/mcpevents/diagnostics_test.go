package mcpevents

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestCallbackTransportCauseIsRetainedAndURLSecretsAreRedacted(t *testing.T) {
	for _, operation := range []string{"verification", "delivery"} {
		t.Run(operation, func(t *testing.T) {
			m, now, _ := testManager(t)
			ctx := context.Background()
			raw := json.RawMessage(strings.Replace(string(request(key('a'), nil)), "https://callback.example/events", "https://callback.example/events?token=callback-query-secret", 1))
			if operation == "delivery" {
				if _, err := m.RPC(ctx, "p", "events/subscribe", raw); err != nil {
					t.Fatal(err)
				}
			}
			cause := errors.New("synthetic TLS certificate rejected: unknown authority")
			transportErr := &url.Error{Op: "dial", URL: "https://callback-user:callback-password@callback.example/redirect?token=nested-query-secret", Err: cause}
			m.client.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, transportErr })
			m.wait = func(_ context.Context, d time.Duration) error { *now = now.Add(d); return nil }
			var err error
			if operation == "delivery" {
				err = m.Deliver(ctx, "p", "mail", *now)
			} else {
				_, err = m.RPC(ctx, "p", "events/subscribe", raw)
			}
			if !errors.Is(err, cause) || !errors.Is(err, transportErr) {
				t.Fatalf("callback lost transport cause: %v", err)
			}
			if !strings.Contains(err.Error(), cause.Error()) || !strings.Contains(err.Error(), "https://callback.example/events") {
				t.Fatalf("missing local callback diagnostic: %v", err)
			}
			for _, secret := range []string{"callback-query-secret", "nested-query-secret", "callback-user", "callback-password", "?token="} {
				if strings.Contains(err.Error(), secret) {
					t.Fatalf("callback diagnostic exposed URL secret: %v", err)
				}
			}
			if operation == "verification" {
				var protocolErr *Error
				if !errors.As(err, &protocolErr) {
					t.Fatalf("verification lost protocol error: %v", err)
				}
				code, message, data := protocolErr.RPCError()
				public, marshalErr := json.Marshal(map[string]any{"code": code, "message": message, "data": data})
				if marshalErr != nil || code != -32015 || protocolErr.Data["reason"] != "unreachable" || strings.Contains(string(public), cause.Error()) || strings.Contains(string(public), "callback.example") {
					t.Fatalf("unsafe public callback error: %s, %v", public, marshalErr)
				}
			}
		})
	}
}
