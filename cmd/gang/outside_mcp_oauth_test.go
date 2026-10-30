//go:build acceptance

package main

import (
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOutsideOAuthWriteFailureReportsLocalCause(t *testing.T) {
	f := newStateFixture(t)
	service, err := f.run.newOutsideService("https://mail.example", "client", "https://client.example/callback", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer service.reports.Close()
	path := filepath.Join(f.run.team.Directory, "outside-mcp", "oauth.json")
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	values := url.Values{
		"response_type": {"code"}, "client_id": {"client"}, "redirect_uri": {"https://client.example/callback"},
		"resource": {"https://mail.example/mcp"}, "state": {"synthetic-state"},
		"code_challenge_method": {"S256"}, "code_challenge": {strings.Repeat("a", 43)},
	}
	w := httptest.NewRecorder()
	service.http.ServeHTTP(w, httptest.NewRequest("GET", "https://mail.example/authorize?"+values.Encode(), nil))
	if w.Code != 500 || strings.TrimSpace(w.Body.String()) != `{"error":"server_error"}` {
		t.Fatalf("OAuth failure response = %d %q", w.Code, w.Body.String())
	}
	diagnostic := f.errOut.String()
	if !strings.Contains(diagnostic, "persist OAuth state") || !strings.Contains(diagnostic, "rename") || !strings.Contains(diagnostic, path+": ") {
		t.Fatalf("missing local storage operation and cause: %q", diagnostic)
	}
	for _, value := range []string{values.Get("state"), values.Get("code_challenge"), values.Get("redirect_uri")} {
		if strings.Contains(diagnostic, value) {
			t.Fatalf("OAuth diagnostic exposed request value: %q", diagnostic)
		}
	}
}
