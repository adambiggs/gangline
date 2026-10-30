//go:build acceptance

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/internal/outsidemcp"
	"github.com/adambiggs/gangline/store"
)

type failedOutsideEvents struct {
	principal string
	calls     int
}

func (e *failedOutsideEvents) ActivePrincipals(context.Context) ([]string, error) {
	return []string{e.principal}, nil
}
func (e *failedOutsideEvents) Deliver(context.Context, string, string, time.Time) error {
	e.calls++
	return errors.New("callback rejected delivery")
}
func (*failedOutsideEvents) Prune(string, []string) error { return nil }
func TestOutsideCallbackFailureKeepsMailboxService(t *testing.T) {
	f := newStateFixture(t)
	_, principal := registerOutsideFixture(t, f)
	if err := f.cmd.send([]string{"toby", "--from", "operator", "review"}); err != nil {
		t.Fatal(err)
	}
	events := &failedOutsideEvents{principal: principal.String()}
	s := &outsideService{run: f.run, events: events}
	if err := s.flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	if events.calls != 1 || !strings.Contains(f.errOut.String(), "pending mail remains available") {
		t.Fatal("missing visible callback failure")
	}
	mail, err := f.run.outsidePending(principal.String())
	if err != nil || len(mail) != 1 {
		t.Fatal("callback failure consumed mail")
	}
}

func TestOutsideMCPRoundTripAndCredentialRevocation(t *testing.T) {
	f := newStateFixture(t)
	credentialFile := filepath.Join(t.TempDir(), "credential")
	if err := f.cmd.execute([]string{"outside", "register", "toby", "--credential-file", credentialFile}); err != nil {
		t.Fatal(err)
	}
	credential, err := os.ReadFile(credentialFile)
	if err != nil {
		t.Fatal(err)
	}
	paths := store.Paths{Root: f.env["GANG_STATE_ROOT"]}
	principal, err := paths.AuthenticateOutside("unit", strings.TrimSpace(string(credential)))
	if err != nil {
		t.Fatal(err)
	}
	f.add(t, "worker-id", "worker", "codex")
	f.run.cmd.detach = func(string, hookNotice) error { return nil }
	dir := t.TempDir()
	reportPath := filepath.Join(dir, "review.md")
	if err := os.WriteFile(reportPath, []byte("review evidence"), 0600); err != nil {
		t.Fatal(err)
	}
	service, err := f.run.newOutsideService("https://mail.example", "chatgpt", "https://chatgpt.com/connector_platform_oauth_redirect", []string{dir})
	if err != nil {
		t.Fatal(err)
	}
	defer service.reports.Close()
	// Use the actual HTTP listener, OAuth consent and token exchange before
	// exercising mailbox tools. No registration bearer is accepted at /mcp.
	server := httptest.NewServer(service.http)
	defer server.Close()
	client := server.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	formRequest := func(method, path string, values url.Values, origin string) (int, http.Header, []byte) {
		t.Helper()
		var body io.Reader
		if values != nil {
			body = strings.NewReader(values.Encode())
		}
		r, err := http.NewRequest(method, server.URL+path, body)
		if err != nil {
			t.Fatal(err)
		}
		if values != nil {
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		response, err := client.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		data, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		return response.StatusCode, response.Header, data
	}
	verifier := strings.Repeat("v", 43)
	hash := sha256.Sum256([]byte(verifier))
	q := url.Values{"response_type": {"code"}, "client_id": {"chatgpt"}, "redirect_uri": {"https://chatgpt.com/connector_platform_oauth_redirect"}, "resource": {"https://mail.example/mcp"}, "scope": {"gangline"}, "state": {"opaque-state"}, "code_challenge_method": {"S256"}, "code_challenge": {base64.RawURLEncoding.EncodeToString(hash[:])}}
	status, _, body := formRequest("GET", "/authorize?"+q.Encode(), nil, "")
	if status != 200 {
		t.Fatalf("consent status %d", status)
	}
	nonce := regexp.MustCompile(`name="request" value="([^"]+)"`).FindSubmatch(body)
	if len(nonce) != 2 {
		t.Fatal("missing nonce")
	}
	status, headers, _ := formRequest("POST", "/authorize", url.Values{"request": {string(nonce[1])}, "credential": {strings.TrimSpace(string(credential))}}, "https://mail.example")
	if status != 303 {
		t.Fatalf("authorization status %d", status)
	}
	callback, err := url.Parse(headers.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	if callback.Query().Get("iss") != "https://mail.example" || callback.Query().Get("state") != "opaque-state" {
		t.Fatal("callback binding")
	}
	status, _, body = formRequest("POST", "/token", url.Values{"grant_type": {"authorization_code"}, "code": {callback.Query().Get("code")}, "client_id": {"chatgpt"}, "redirect_uri": {"https://chatgpt.com/connector_platform_oauth_redirect"}, "resource": {"https://mail.example/mcp"}, "code_verifier": {verifier}}, "")
	if status != 200 {
		t.Fatalf("token status %d", status)
	}
	var tokens struct {
		Access string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &tokens); err != nil || tokens.Access == "" {
		t.Fatal("missing access token")
	}
	rpc := func(token, method string, params map[string]any) (int, map[string]any) {
		t.Helper()
		params["_meta"] = map[string]any{"io.modelcontextprotocol/protocolVersion": outsidemcp.ProtocolVersion, "io.modelcontextprotocol/clientCapabilities": map[string]any{}}
		data, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
		if err != nil {
			t.Fatal(err)
		}
		r, err := http.NewRequest("POST", server.URL+"/mcp", bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Authorization", "Bearer "+token)
		r.Header.Set("MCP-Protocol-Version", outsidemcp.ProtocolVersion)
		r.Header.Set("Mcp-Method", method)
		if name, ok := params["name"].(string); ok {
			r.Header.Set("Mcp-Name", name)
		}
		response, err := client.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		var result map[string]any
		if response.StatusCode == 200 {
			if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
				t.Fatal(err)
			}
		}
		return response.StatusCode, result
	}
	if status, _ := rpc(strings.TrimSpace(string(credential)), "server/discover", map[string]any{}); status != 401 {
		t.Fatal("registration credential admitted as access token")
	}
	if err := f.cmd.send([]string{"toby", "--from", "operator", "review " + reportPath}); err != nil {
		t.Fatal(err)
	}
	mail, err := paths.ReadOutsidePending("unit", principal)
	if err != nil || len(mail) != 1 {
		t.Fatal("outbound mailbox missing", err)
	}
	call := func(name string, args map[string]any) map[string]any {
		t.Helper()
		status, result := rpc(tokens.Access, "tools/call", map[string]any{"name": name, "arguments": args})
		if status != 200 || result["error"] != nil {
			t.Fatalf("tool %s status %d", name, status)
		}
		tool := result["result"].(map[string]any)
		if tool["isError"] != false {
			t.Fatalf("tool %s failed: %v", name, tool["content"])
		}
		return tool["structuredContent"].(map[string]any)
	}
	if got := call("fetch_mail", map[string]any{}); len(got["messages"].([]any)) != 1 {
		t.Fatal("fetch lost message")
	}
	if got := call("read_report", map[string]any{"path": reportPath}); got["content"] != "review evidence" {
		t.Fatal("report read mismatch")
	}
	first := call("send_message", map[string]any{"to": "worker", "body": "approved", "idempotency_key": "review-one"})
	second := call("send_message", map[string]any{"to": "worker", "body": "approved", "idempotency_key": "review-one"})
	if first["id"] != second["id"] || first["outcome"] != "queued" {
		t.Fatal("lost idempotent receipt")
	}
	p, _ := f.run.team.Agent("worker-id")
	pending, err := p.ListNew()
	if err != nil || len(pending) != 1 || pending[0].From.Kind != core.SenderOutside || pending[0].From.Name != "toby" {
		t.Fatalf("verified inbound failed: %v", err)
	}
	call("ack_mail", map[string]any{"ids": []string{string(mail[0].ID)}})
	if got := call("fetch_mail", map[string]any{}); len(got["messages"].([]any)) != 0 {
		t.Fatal("ack did not remove mail")
	}
	// Restart preserves the access grant and mailbox; rotation then revokes it.
	restarted, err := f.run.newOutsideService("https://mail.example", "chatgpt", "https://chatgpt.com/connector_platform_oauth_redirect", []string{dir})
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.reports.Close()
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "https://mail.example/mcp", strings.NewReader(`{}`))
	r.Header.Set("Authorization", "Bearer "+tokens.Access)
	restarted.http.ServeHTTP(w, r)
	if w.Code == 401 {
		t.Fatal("restart lost grant")
	}
	if err := paths.RotateOutside("unit", "toby", filepath.Join(t.TempDir(), "replacement"), credentialTestOrigin()); err != nil {
		t.Fatal(err)
	}
	if status, _ := rpc(tokens.Access, "server/discover", map[string]any{}); status != 401 {
		t.Fatal("rotated grant stayed valid")
	}
}
