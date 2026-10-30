//go:build acceptance

package outsidemcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

type fakeEvents struct{}

func (fakeEvents) RPC(context.Context, string, string, json.RawMessage) (any, error) {
	return map[string]any{"resultType": "complete", "events": []any{}}, nil
}

func fixture(t *testing.T) (*Handler, *int) {
	t.Helper()
	sends := 0
	return New(Config{Origin: "https://mail.example", Challenge: `Bearer resource_metadata="https://mail.example/.well-known/oauth-protected-resource"`,
		Authenticate: func(token string) (string, error) {
			if token != "access" {
				return "", errors.New("invalid")
			}
			return "principal", nil
		},
		Authorize: func(string) error { return nil }, Pending: func(string) ([]Mail, error) { return []Mail{{ID: "mail-1", From: "lead", Body: "read report"}}, nil },
		Ack: func(p string, ids []string) error {
			if p != "principal" || len(ids) != 1 || ids[0] != "mail-1" {
				t.Fatal("wrong acknowledgment")
			}
			return nil
		},
		Send: func(p, to, body, key string) (Receipt, error) {
			sends++
			if p != "principal" || to != "lead" || body != "verdict" || key != "request-1" {
				t.Fatal("wrong send")
			}
			return Receipt{"reply-1", "queued"}, nil
		}, Events: fakeEvents{}}), &sends
}
func request(method string, params any) *http.Request {
	p := map[string]any{"_meta": map[string]any{"io.modelcontextprotocol/protocolVersion": ProtocolVersion, "io.modelcontextprotocol/clientCapabilities": map[string]any{}}}
	for key, value := range params.(map[string]any) {
		p[key] = value
	}
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": p})
	r := httptest.NewRequest("POST", "https://mail.example/mcp", strings.NewReader(string(body)))
	r.Header.Set("Authorization", "Bearer access")
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("MCP-Protocol-Version", ProtocolVersion)
	r.Header.Set("Mcp-Method", method)
	if name, ok := p["name"].(string); ok {
		r.Header.Set("Mcp-Name", name)
	}
	return r
}
func TestMailboxToolsAndDiscovery(t *testing.T) {
	h, sends := fixture(t)
	for _, tt := range []struct {
		method string
		params map[string]any
		want   string
	}{
		{"server/discover", map[string]any{}, `"events":{}`},
		{"tools/list", map[string]any{}, `read_report`},
		{"tools/call", map[string]any{"name": "fetch_mail", "arguments": map[string]any{}}, `"from":"lead"`},
		{"tools/call", map[string]any{"name": "ack_mail", "arguments": map[string]any{"ids": []string{"mail-1"}}}, `"acknowledged"`},
		{"tools/call", map[string]any{"name": "send_message", "arguments": map[string]any{"to": "lead", "body": "verdict", "idempotency_key": "request-1"}}, `"outcome":"queued"`},
		{"events/list", map[string]any{}, `"events":[]`},
	} {
		t.Run(tt.method+fmt.Sprint(tt.params), func(t *testing.T) {
			w := httptest.NewRecorder()
			h.ServeHTTP(w, request(tt.method, tt.params))
			if w.Code != 200 || !strings.Contains(w.Body.String(), tt.want) || strings.Contains(w.Body.String(), `"error"`) {
				t.Fatalf("%d %s", w.Code, w.Body)
			}
		})
	}
	if *sends != 1 {
		t.Fatal(*sends)
	}
}
func TestRejectsUnauthenticatedRevokedAndMalformedTransport(t *testing.T) {
	for _, tt := range []struct {
		name   string
		change func(*Handler, *http.Request)
		status int
	}{
		{"no bearer", func(_ *Handler, r *http.Request) { r.Header.Del("Authorization") }, 401},
		{"bad bearer", func(_ *Handler, r *http.Request) { r.Header.Set("Authorization", "Bearer wrong") }, 401},
		{"revoked", func(h *Handler, _ *http.Request) {
			h.config.Authorize = func(string) error { return errors.New("revoked") }
		}, 401},
		{"origin", func(_ *Handler, r *http.Request) { r.Header.Set("Origin", "https://attacker.example") }, 403},
		{"version header", func(_ *Handler, r *http.Request) { r.Header.Set("MCP-Protocol-Version", "2025-11-25") }, 400},
		{"method header", func(_ *Handler, r *http.Request) { r.Header.Set("Mcp-Method", "tools/call") }, 400},
		{"wrong type", func(_ *Handler, r *http.Request) { r.Header.Set("Content-Type", "text/plain") }, 415},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h, sends := fixture(t)
			r := request("server/discover", map[string]any{})
			tt.change(h, r)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tt.status || *sends != 0 {
				t.Fatalf("%d %s", w.Code, w.Body)
			}
		})
	}
}
func TestToolArgumentsCannotChooseSenderOrDirectoryRoots(t *testing.T) {
	h, sends := fixture(t)
	for _, args := range []map[string]any{
		{"to": "lead", "body": "verdict", "idempotency_key": "request-1", "from": "lead"},
		{"to": "lead", "body": "verdict", "idempotency_key": "request-1", "principal": "other"},
	} {
		r := request("tools/call", map[string]any{"name": "send_message", "arguments": args})
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if !strings.Contains(w.Body.String(), `"isError":true`) {
			t.Fatal(w.Body)
		}
	}
	if *sends != 0 {
		t.Fatal("bad arguments sent a message")
	}
}

func TestUnknownMethodsAndInvalidParamsHaveProtocolHTTPStatus(t *testing.T) {
	h, _ := fixture(t)
	for _, tt := range []struct {
		method string
		params map[string]any
		status int
		code   string
	}{
		{"unknown/method", map[string]any{}, 404, `"code":-32601`},
		{"server/discover", map[string]any{"unexpected": true}, 400, `"code":-32602`},
	} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, request(tt.method, tt.params))
		if w.Code != tt.status || !strings.Contains(w.Body.String(), tt.code) {
			t.Fatalf("%d %s", w.Code, w.Body)
		}
	}
}
func TestToolErrorsKeepPrivateStatePathsInServerDiagnostics(t *testing.T) {
	h, _ := fixture(t)
	err := errors.New("read /private/operator/state/teams/team/outside/secret-id.json: permission denied")
	h.config.Pending = func(string) ([]Mail, error) { return nil, err }
	var diagnostic error
	h.config.ReportError = func(err error) { diagnostic = err }
	w := httptest.NewRecorder()
	h.ServeHTTP(w, request("tools/call", map[string]any{"name": "fetch_mail", "arguments": map[string]any{}}))
	if strings.Contains(w.Body.String(), "/private/operator") || !strings.Contains(w.Body.String(), "operation failed") || diagnostic != err {
		t.Fatalf("private error not confined: %s", w.Body)
	}
}
func TestReportContainmentPaginationAndSpecialFiles(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "reports")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	write := func(path, body string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(dir, "secret"), "secret")
	body := strings.Repeat("a", reportPageBytes-1) + "é" + "ending"
	path := filepath.Join(root, "report.md")
	write(path, body)
	if err := os.Symlink(filepath.Join(dir, "secret"), filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(root, "pipe"), 0600); err != nil {
		t.Fatal(err)
	}
	write(filepath.Join(root, "binary"), string([]byte{0xff, 0x00}))
	reports, err := OpenReports([]string{root})
	if err != nil {
		t.Fatal(err)
	}
	defer reports.Close()
	first, err := reports.Read(path, 0)
	if err != nil || first.NextOffset == nil {
		t.Fatalf("%+v %v", first, err)
	}
	second, err := reports.Read(path, *first.NextOffset)
	if err != nil || second.NextOffset != nil || first.Content+second.Content != body {
		t.Fatalf("bad pagination %v", err)
	}
	for _, p := range []string{filepath.Join(dir, "secret"), filepath.Join(root, "escape"), filepath.Join(root, "pipe"), filepath.Join(root, "binary"), root, root + "/../secret", root + "-sibling/report.md", "relative.md"} {
		if _, err := reports.Read(p, 0); err == nil {
			t.Errorf("allowed %s", p)
		}
	}
	if _, err := reports.Read(path, int64(len(body)+1)); err == nil {
		t.Fatal("allowed offset after EOF")
	}
	if _, err := reports.Read(path, int64(reportPageBytes)); err == nil {
		t.Fatal("allowed offset within a rune")
	}
	// Replacing the path of the configured directory cannot redirect the open root.
	if err := os.Rename(root, root+"-moved"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(dir, root); err != nil {
		t.Fatal(err)
	}
	if _, err := reports.Read(filepath.Join(root, "secret"), 0); err == nil {
		t.Fatal("root replacement escaped")
	}
}
func TestReportRechecksRevocationAfterRead(t *testing.T) {
	h, _ := fixture(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "report")
	if err := os.WriteFile(path, []byte("sensitive"), 0600); err != nil {
		t.Fatal(err)
	}
	r, err := OpenReports([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	h.config.Reports = r
	h.config.Authorize = func(string) error { return errors.New("revoked") }
	if _, err := h.tool("principal", "read_report", json.RawMessage(fmt.Sprintf(`{"path":%q}`, path))); err == nil {
		t.Fatal("exposed report after revocation")
	}
}
