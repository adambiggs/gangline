package mcpevents

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func response(code int, body string) *http.Response {
	return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
}
func key(b byte) string {
	return "whsec_" + base64.StdEncoding.EncodeToString([]byte(strings.Repeat(string([]byte{b}), 32)))
}
func request(secret string, ttl any) json.RawMessage {
	p := map[string]any{"name": EventName, "arguments": map[string]any{}, "delivery": map[string]any{"mode": "webhook", "url": "https://callback.example/events", "secret": secret}, "cursor": nil, "_meta": map[string]any{"test": "metadata"}}
	if ttl != "omit" {
		p["ttlMs"] = ttl
	}
	raw, _ := json.Marshal(p)
	return raw
}
func testManager(t *testing.T) (*Manager, *time.Time, *bool) {
	t.Helper()
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	authorized := true
	m, err := New(Config{StatePath: filepath.Join(t.TempDir(), "events.json"), Now: func() time.Time { return now }, Authorize: func(context.Context, string) bool { return authorized }})
	if err != nil {
		t.Fatal(err)
	}
	m.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var body struct {
			Challenge string `json:"challenge"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		b, _ := json.Marshal(map[string]string{"challenge": body.Challenge})
		return response(200, string(b)), nil
	})
	m.wait = func(context.Context, time.Duration) error { t.Fatal("unexpected retry"); return nil }
	return m, &now, &authorized
}
func subscribe(t *testing.T, m *Manager, principal, secret string, ttl any) string {
	t.Helper()
	result, err := m.RPC(context.Background(), principal, "events/subscribe", request(secret, ttl))
	if err != nil {
		t.Fatal(err)
	}
	fields := result.(map[string]any)
	if fields["cursor"] != nil || fields["truncated"] != false || fields["resultType"] != "complete" {
		t.Fatalf("bad response: %#v", fields)
	}
	return fields["id"].(string)
}
func assertSignature(t *testing.T, r *http.Request, secret string, body []byte, index int) {
	t.Helper()
	keyBytes, _ := base64.StdEncoding.DecodeString(strings.TrimPrefix(secret, "whsec_"))
	mac := hmac.New(sha256.New, keyBytes)
	mac.Write([]byte(r.Header.Get("webhook-id") + "." + r.Header.Get("webhook-timestamp") + "."))
	mac.Write(body)
	parts := strings.Fields(r.Header.Get("webhook-signature"))
	if len(parts) <= index {
		t.Fatal("missing signature")
	}
	if parts[index] != "v1,"+base64.StdEncoding.EncodeToString(mac.Sum(nil)) {
		t.Fatal("bad webhook HMAC")
	}
}
func TestLifecycleAndPrivatePersistence(t *testing.T) {
	m, now, _ := testManager(t)
	ctx := context.Background()
	checks := 0
	m.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(r.Body)
		assertSignature(t, r, key('a'), body, 0)
		if r.Header.Get("X-MCP-Subscription-Id") == "" {
			t.Fatal("missing subscription ID")
		}
		var v map[string]any
		if err := json.Unmarshal(body, &v); err != nil {
			t.Fatal(err)
		}
		checks++
		return response(200, `{"challenge":"`+v["challenge"].(string)+`"}`), nil
	})
	id := subscribe(t, m, "registration-a", key('a'), "omit")
	if got := subscribe(t, m, "registration-a", key('a'), "omit"); got != id {
		t.Fatal("refresh changed identity")
	}
	other := subscribe(t, m, "registration-b", key('a'), "omit")
	if other == id {
		t.Fatal("principal identities collided")
	}
	if checks != 2 {
		t.Fatalf("verification cache failed: %d", checks)
	}
	st, err := os.Stat(m.config.StatePath)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0600 {
		t.Fatal("subscription secrets not private")
	}
	if expires := m.state.Subscriptions[id].ExpiresAt; expires == nil || !expires.Equal(now.Add(defaultTTL)) {
		t.Fatal("default expiration")
	}
	reload, err := New(m.config)
	if err != nil {
		t.Fatal(err)
	}
	if reload.identity("registration-a", "https://callback.example/events") != id {
		t.Fatal("identity changed after restart")
	}
	raw := request("", nil)
	if _, err := reload.RPC(ctx, "registration-a", "events/unsubscribe", raw); err != nil {
		t.Fatal(err)
	}
	if _, exists := reload.state.Subscriptions[id]; exists {
		t.Fatal("unsubscribe retained subscription")
	}
	if _, err := reload.RPC(ctx, "registration-a", "events/unsubscribe", raw); err != nil {
		t.Fatal(err)
	}
	if _, exists := reload.state.Subscriptions[other]; !exists {
		t.Fatal("unsubscribe crossed principal")
	}
}
func TestDeliverReceiptRotationAndNoReplay(t *testing.T) {
	m, now, _ := testManager(t)
	ctx := context.Background()
	id := subscribe(t, m, "p", key('a'), nil)
	if m.state.Subscriptions[id].ExpiresAt != nil {
		t.Fatal("null TTL must be infinite")
	}
	*now = now.Add(time.Second)
	subscribe(t, m, "p", key('b'), nil)
	count := 0
	m.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(r.Body)
		assertSignature(t, r, key('b'), body, 0)
		assertSignature(t, r, key('a'), body, 1)
		var event struct {
			ID        string         `json:"eventId"`
			Name      string         `json:"name"`
			Timestamp string         `json:"timestamp"`
			Data      map[string]any `json:"data"`
			Cursor    any            `json:"cursor"`
		}
		if err := json.Unmarshal(body, &event); err != nil {
			t.Fatal(err)
		}
		if event.ID != "mail-1" || event.Name != EventName || event.Data["id"] != "mail-1" || len(event.Data) != 1 || event.Cursor != nil || event.Timestamp != now.Format(time.RFC3339Nano) {
			t.Fatalf("unexpected payload: %s", body)
		}
		if r.Header.Get("webhook-id") != event.ID {
			t.Fatal("header identity differs")
		}
		count++
		return response(204, ""), nil
	})
	if err := m.Deliver(ctx, "p", "old-mail", now.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := m.Deliver(ctx, "p", "mail-1", *now); err != nil {
		t.Fatal(err)
	}
	reload, err := New(m.config)
	if err != nil {
		t.Fatal(err)
	}
	reload.client = m.client
	if err := reload.Deliver(ctx, "p", "mail-1", *now); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("redelivered acknowledged webhook %d", count)
	}
	if err := reload.Prune("p", nil); err != nil {
		t.Fatal(err)
	}
	if len(reload.state.Subscriptions[id].Receipts) != 0 {
		t.Fatal("receipt not pruned")
	}
	*now = now.Add(rotationWindow + time.Second)
	reload.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(r.Body)
		assertSignature(t, r, key('b'), body, 0)
		if len(strings.Fields(r.Header.Get("webhook-signature"))) != 1 {
			t.Fatal("old key retained after rotation")
		}
		return response(200, ""), nil
	})
	if err := reload.Deliver(ctx, "p", "mail-2", *now); err != nil {
		t.Fatal(err)
	}
}
func TestRetryBoundAndTerminalResponses(t *testing.T) {
	for _, status := range []int{500, 429, 410, 413, 400} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			m, now, _ := testManager(t)
			id := subscribe(t, m, "p", key('a'), nil)
			calls := 0
			var waits []time.Duration
			m.wait = func(_ context.Context, d time.Duration) error {
				waits = append(waits, d)
				*now = now.Add(d)
				return nil
			}
			var firstBody string
			m.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				b, _ := io.ReadAll(r.Body)
				if firstBody == "" {
					firstBody = string(b)
				} else if firstBody != string(b) {
					t.Fatal("retry changed event bytes")
				}
				assertSignature(t, r, key('a'), b, 0)
				if r.Header.Get("webhook-timestamp") != strconv.FormatInt(now.Unix(), 10) {
					t.Fatal("stale signing timestamp")
				}
				calls++
				return response(status, ""), nil
			})
			if err := m.Deliver(context.Background(), "p", "mail-1", *now); err == nil {
				t.Fatal("failure reported as success")
			}
			want := 1
			if status == 500 || status == 429 {
				want = 3
			}
			if calls != want {
				t.Fatalf("calls=%d want=%d", calls, want)
			}
			if want == 3 && (len(waits) != 2 || waits[0] != time.Second || waits[1] != 2*time.Second) {
				t.Fatal("retry backoff differs")
			}
			if err := m.Deliver(context.Background(), "p", "mail-1", *now); err != nil {
				t.Fatal(err)
			}
			if calls != want {
				t.Fatal("terminal failure retried by pump")
			}
			if status == 410 {
				if _, ok := m.state.Subscriptions[id]; ok {
					t.Fatal("gone callback retained")
				}
			}
		})
	}
}
func TestExpirationAndRevocation(t *testing.T) {
	m, now, authorized := testManager(t)
	ctx := context.Background()
	subscribe(t, m, "p", key('a'), 1000)
	*now = now.Add(time.Second)
	m.client.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("expired subscription delivered")
		return nil, nil
	})
	if err := m.Deliver(ctx, "p", "mail", *now); err != nil {
		t.Fatal(err)
	}
	names, err := m.ActivePrincipals(ctx)
	if err != nil || len(names) != 0 {
		t.Fatal("expired subscription active")
	}
	// The successful callback verification remains cached for this principal and URL.
	subscribe(t, m, "p", key('a'), nil)
	*authorized = false
	if _, err := m.RPC(ctx, "p", "events/list", nil); err == nil {
		t.Fatal("revoked principal discovered events")
	}
	if err := m.Deliver(ctx, "p", "mail", *now); err != nil {
		t.Fatal(err)
	}
	names, err = m.ActivePrincipals(ctx)
	if err != nil || len(names) != 0 || len(m.state.Subscriptions) != 0 {
		t.Fatal("revoked subscription retained")
	}
}
func TestVerificationFailuresAndValidation(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		err    error
		reason string
	}{
		{"challenge", 200, `{"challenge":"wrong"}`, nil, "challenge_failed"},
		{"status", 403, "", nil, "http_status"},
		{"timeout", 0, "", context.DeadlineExceeded, "timeout"},
		{"blocked", 0, "", &blockedAddressError{}, "blocked_address"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, _, _ := testManager(t)
			m.client.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
				if tt.err != nil {
					return nil, tt.err
				}
				return response(tt.status, tt.body), nil
			})
			_, err := m.RPC(context.Background(), "p", "events/subscribe", request(key('a'), nil))
			var rpcErr *Error
			if !errors.As(err, &rpcErr) || rpcErr.Code != -32015 || rpcErr.Data["reason"] != tt.reason {
				t.Fatalf("unexpected failure: %v", err)
			}
			if len(m.state.Subscriptions) != 0 {
				t.Fatal("failed verification activated subscription")
			}
		})
	}
	for _, secret := range []string{"abc", key('a') + "x", "whsec_" + base64.StdEncoding.EncodeToString(make([]byte, 23)), "whsec_" + base64.StdEncoding.EncodeToString(make([]byte, 65))} {
		m, _, _ := testManager(t)
		if _, err := m.RPC(context.Background(), "p", "events/subscribe", request(secret, nil)); err == nil {
			t.Fatal("invalid secret accepted")
		}
	}
	for _, ttl := range []any{0, -1, 1.5, "infinite", int64(1<<63 - 1)} {
		m, _, _ := testManager(t)
		if _, err := m.RPC(context.Background(), "p", "events/subscribe", request(key('a'), ttl)); err == nil {
			t.Fatalf("invalid TTL accepted: %v", ttl)
		}
	}
	m, _, _ := testManager(t)
	raw := strings.Replace(string(request(key('a'), nil)), `"arguments":{}`, `"arguments":{"other":"value"}`, 1)
	if _, err := m.RPC(context.Background(), "p", "events/subscribe", json.RawMessage(raw)); err == nil {
		t.Fatal("undeclared arguments accepted")
	}
	for _, u := range []string{"http://example.com", "https://user:pass@example.com", "https://example.com/#fragment", "https://example.com:99999"} {
		if validateURL(u) == nil {
			t.Fatalf("invalid URL accepted: %s", u)
		}
	}
}
func TestRedirectBlocked(t *testing.T) {
	m, _, _ := testManager(t)
	calls := 0
	m.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		resp := response(302, "")
		resp.Header.Set("Location", "https://127.0.0.1/private")
		return resp, nil
	})
	_, err := m.RPC(context.Background(), "p", "events/subscribe", request(key('a'), nil))
	if err == nil || calls != 1 {
		t.Fatal("redirect followed or accepted")
	}
}

func TestTransientFailureThenSuccess(t *testing.T) {
	m, now, _ := testManager(t)
	ctx := context.Background()
	subscribe(t, m, "p", key('a'), nil)
	calls := 0
	m.wait = func(_ context.Context, d time.Duration) error { *now = now.Add(d); return nil }
	m.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return response(503, ""), nil
		}
		return response(202, ""), nil
	})
	if err := m.Deliver(ctx, "p", "mail", *now); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("calls=%d", calls)
	}
	if err := m.Deliver(ctx, "p", "mail", *now); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatal("accepted delivery retried")
	}
}

func TestRotationKeyCleanupAndAuthorizationDuringRetry(t *testing.T) {
	m, now, authorized := testManager(t)
	ctx := context.Background()
	id := subscribe(t, m, "p", key('a'), nil)
	subscribe(t, m, "p", key('b'), nil)
	*now = now.Add(rotationWindow)
	if _, err := m.ActivePrincipals(ctx); err != nil {
		t.Fatal(err)
	}
	if m.state.Subscriptions[id].OldSecret != "" {
		t.Fatal("expired rotation secret retained")
	}
	calls := 0
	m.client.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) { calls++; return response(503, ""), nil })
	m.wait = func(context.Context, time.Duration) error { *authorized = false; return nil }
	if err := m.Deliver(ctx, "p", "mail", *now); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatal("revoked credential sent a retry")
	}
}
