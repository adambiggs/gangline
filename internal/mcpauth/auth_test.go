package mcpauth

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

type fixture struct {
	s       *Server
	cfg     Config
	now     time.Time
	revoked bool
}

func setup(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{now: time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)}
	f.cfg = Config{Issuer: "https://gang.example", ClientID: "chatgpt", RedirectURI: "https://chatgpt.com/connector/oauth/callback", StateDir: filepath.Join(t.TempDir(), "oauth"), Clock: func() time.Time { return f.now }, AuthenticateCredential: func(s string) (string, error) {
		if s == "registration-secret" {
			return "outside.generation", nil
		}
		return "", errors.New("bad credential")
	}, Authorize: func(p string) error {
		if f.revoked || p != "outside.generation" {
			return errors.New("revoked")
		}
		return nil
	}}
	var err error
	f.s, err = New(f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	return f
}
func (f *fixture) request(method, path string, v url.Values, origin string) *httptest.ResponseRecorder {
	body := ""
	if v != nil {
		body = v.Encode()
	}
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if v != nil {
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if origin != "" {
		r.Header.Set("Origin", origin)
	}
	w := httptest.NewRecorder()
	f.s.ServeHTTP(w, r)
	return w
}

const verifier = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func authValues() url.Values {
	h := sha256.Sum256([]byte(verifier))
	return url.Values{"client_id": {"chatgpt"}, "redirect_uri": {"https://chatgpt.com/connector/oauth/callback"}, "response_type": {"code"}, "resource": {"https://gang.example/mcp"}, "scope": {"gangline"}, "state": {"client-state"}, "code_challenge_method": {"S256"}, "code_challenge": {base64.RawURLEncoding.EncodeToString(h[:])}}
}
func (f *fixture) consent(t *testing.T, v url.Values) string {
	t.Helper()
	w := f.request("GET", "/authorize?"+v.Encode(), nil, "")
	if w.Code != 200 {
		t.Fatalf("consent %d %s", w.Code, w.Body.String())
	}
	matches := regexp.MustCompile(`name="request" value="([^"]+)"`).FindStringSubmatch(w.Body.String())
	if len(matches) != 2 {
		t.Fatal("missing consent request")
	}
	return matches[1]
}
func (f *fixture) code(t *testing.T) string {
	t.Helper()
	request := f.consent(t, authValues())
	w := f.request("POST", "/authorize", url.Values{"request": {request}, "credential": {"registration-secret"}}, f.cfg.Issuer)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("authorize %d %s", w.Code, w.Body.String())
	}
	u, err := url.Parse(w.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	if u.Query().Get("state") != "client-state" || u.Query().Get("iss") != f.cfg.Issuer {
		t.Fatal("missing callback binding")
	}
	return u.Query().Get("code")
}
func exchangeValues(code string) url.Values {
	return url.Values{"grant_type": {"authorization_code"}, "client_id": {"chatgpt"}, "redirect_uri": {"https://chatgpt.com/connector/oauth/callback"}, "resource": {"https://gang.example/mcp"}, "code": {code}, "code_verifier": {verifier}}
}
func decode(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	v := map[string]any{}
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	return v
}
func (f *fixture) tokens(t *testing.T) map[string]any {
	t.Helper()
	w := f.request("POST", "/token", exchangeValues(f.code(t)), "")
	if w.Code != 200 {
		t.Fatalf("token %d %s", w.Code, w.Body.String())
	}
	return decode(t, w)
}
func refreshValues(token string) url.Values {
	return url.Values{"grant_type": {"refresh_token"}, "client_id": {"chatgpt"}, "resource": {"https://gang.example/mcp"}, "refresh_token": {token}}
}

func TestFlowPersistenceRotationAndRevocation(t *testing.T) {
	f := setup(t)
	tokens := f.tokens(t)
	access := tokens["access_token"].(string)
	refresh := tokens["refresh_token"].(string)
	if p, err := f.s.AuthenticateAccess(access); err != nil || p != "outside.generation" {
		t.Fatalf("identity %q %v", p, err)
	}
	data, err := os.ReadFile(filepath.Join(f.cfg.StateDir, "oauth.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{access, refresh, "registration-secret"} {
		if strings.Contains(string(data), secret) {
			t.Fatal("raw secret persisted")
		}
	}
	info, err := os.Stat(filepath.Join(f.cfg.StateDir, "oauth.json"))
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("non-private state")
	}
	f.s, err = New(f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.s.AuthenticateAccess(access); err != nil {
		t.Fatal(err)
	}
	w := f.request("POST", "/token", refreshValues(refresh), "")
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	rotated := decode(t, w)
	if rotated["refresh_token"] == refresh {
		t.Fatal("refresh token did not rotate")
	}
	// Public-client rotation detects reuse and revokes the active token family.
	w = f.request("POST", "/token", refreshValues(refresh), "")
	if w.Code != 400 {
		t.Fatal("refresh replay accepted")
	}
	if _, err := f.s.AuthenticateAccess(rotated["access_token"].(string)); err == nil {
		t.Fatal("replayed grant family remains active")
	}
	if f.request("POST", "/token", refreshValues(rotated["refresh_token"].(string)), "").Code != 400 {
		t.Fatal("replayed family refresh remains active")
	}
	tokens = f.tokens(t)
	f.revoked = true
	if _, err := f.s.AuthenticateAccess(tokens["access_token"].(string)); err == nil {
		t.Fatal("revoked access accepted")
	}
	if f.request("POST", "/token", refreshValues(tokens["refresh_token"].(string)), "").Code != 400 {
		t.Fatal("revoked refresh accepted")
	}
}
func TestAuthorizationRefutesRequestSubstitution(t *testing.T) {
	for _, key := range []string{"client_id", "redirect_uri", "resource", "code_challenge_method", "code_challenge", "response_type", "scope", "state"} {
		t.Run(key, func(t *testing.T) {
			f := setup(t)
			v := authValues()
			v.Set(key, "attacker")
			if key == "state" {
				v.Set(key, "")
			}
			if f.request("GET", "/authorize?"+v.Encode(), nil, "").Code != 400 {
				t.Fatal("invalid request accepted")
			}
		})
	}
	f := setup(t)
	v := authValues()
	v.Add("redirect_uri", v.Get("redirect_uri"))
	if f.request("GET", "/authorize?"+v.Encode(), nil, "").Code != 400 {
		t.Fatal("duplicate accepted")
	}
	request := f.consent(t, authValues())
	form := url.Values{"request": {request}, "credential": {"registration-secret"}}
	for _, origin := range []string{"", "https://attacker.example", "null"} {
		if f.request("POST", "/authorize", form, origin).Code != 400 {
			t.Fatal("cross-origin consent accepted")
		}
	}
	form.Set("resource", "https://attacker.example")
	if f.request("POST", "/authorize", form, f.cfg.Issuer).Code != 400 {
		t.Fatal("POST query substitution accepted")
	}
	form.Del("resource")
	w := f.request("POST", "/authorize", form, f.cfg.Issuer)
	if w.Code != 303 {
		t.Fatalf("consent: %d %s", w.Code, w.Body.String())
	}
	if f.request("POST", "/authorize", form, f.cfg.Issuer).Code != 400 {
		t.Fatal("consent replay accepted")
	}
}
func TestCodeBindingReplayAndExpiry(t *testing.T) {
	for _, key := range []string{"client_id", "redirect_uri", "resource", "code_verifier"} {
		t.Run(key, func(t *testing.T) {
			f := setup(t)
			code := f.code(t)
			v := exchangeValues(code)
			v.Set(key, strings.Repeat("x", 43))
			if f.request("POST", "/token", v, "").Code != 400 {
				t.Fatal("unbound code exchange accepted")
			}
			if f.request("POST", "/token", exchangeValues(code), "").Code != 200 {
				t.Fatal("valid code rejected")
			}
			if f.request("POST", "/token", exchangeValues(code), "").Code != 400 {
				t.Fatal("code replay accepted")
			}
		})
	}
	f := setup(t)
	code := f.code(t)
	f.now = f.now.Add(codeLifetime)
	if f.request("POST", "/token", exchangeValues(code), "").Code != 400 {
		t.Fatal("expired code accepted")
	}
	f = setup(t)
	request := f.consent(t, authValues())
	f.now = f.now.Add(consentLifetime)
	if f.request("POST", "/authorize", url.Values{"request": {request}, "credential": {"registration-secret"}}, f.cfg.Issuer).Code != 400 {
		t.Fatal("expired consent accepted")
	}
	f = setup(t)
	tokens := f.tokens(t)
	f.now = f.now.Add(accessLifetime)
	if _, err := f.s.AuthenticateAccess(tokens["access_token"].(string)); err == nil {
		t.Fatal("expired access accepted")
	}
	f.now = f.now.Add(refreshLifetime)
	if f.request("POST", "/token", refreshValues(tokens["refresh_token"].(string)), "").Code != 400 {
		t.Fatal("expired refresh accepted")
	}
}
func TestDiscoveryAndBrowserSecurity(t *testing.T) {
	f := setup(t)
	for _, path := range []string{"/.well-known/oauth-protected-resource", "/.well-known/oauth-protected-resource/mcp", "/.well-known/oauth-authorization-server"} {
		w := f.request("GET", path, nil, "")
		if w.Code != 200 {
			t.Fatal(path)
		}
		v := decode(t, w)
		if path == "/.well-known/oauth-authorization-server" {
			if v["issuer"] != f.cfg.Issuer || v["registration_endpoint"] != nil {
				t.Fatal("wrong metadata")
			}
		} else if v["resource"] != "https://gang.example/mcp" {
			t.Fatal("wrong audience")
		}
	}
	w := f.request("GET", "/authorize?"+authValues().Encode(), nil, "")
	for key, value := range map[string]string{"Cache-Control": "no-store", "Referrer-Policy": "no-referrer", "X-Frame-Options": "DENY", "X-Content-Type-Options": "nosniff"} {
		if w.Header().Get(key) != value {
			t.Fatal(key)
		}
	}
	if w.Header().Get("Set-Cookie") != "" || !strings.Contains(w.Header().Get("Content-Security-Policy"), "form-action 'self'") {
		t.Fatal("unsafe consent page")
	}
	if !strings.Contains(f.s.Challenge(), `resource_metadata="https://gang.example/.well-known/oauth-protected-resource"`) {
		t.Fatal("missing challenge")
	}
}
func TestRevokedConsentAndCode(t *testing.T) {
	f := setup(t)
	request := f.consent(t, authValues())
	f.revoked = true
	if f.request("POST", "/authorize", url.Values{"request": {request}, "credential": {"registration-secret"}}, f.cfg.Issuer).Code != 403 {
		t.Fatal("revoked consent accepted")
	}
	f.revoked = false
	code := f.code(t)
	f.revoked = true
	if f.request("POST", "/token", exchangeValues(code), "").Code != 400 {
		t.Fatal("revoked code accepted")
	}
}
func TestConfigurationAndStateFailClosed(t *testing.T) {
	f := setup(t)
	cfg := f.cfg
	cfg.Issuer = "http://gang.example"
	if _, err := New(cfg); err == nil {
		t.Fatal("HTTP accepted")
	}
	cfg = f.cfg
	cfg.Issuer += "/"
	if _, err := New(cfg); err == nil {
		t.Fatal("ambiguous issuer accepted")
	}
	_ = f.tokens(t)
	cfg = f.cfg
	cfg.ClientID = "other"
	if _, err := New(cfg); err == nil {
		t.Fatal("state client mismatch accepted")
	}
	if err := os.Chmod(filepath.Join(f.cfg.StateDir, "oauth.json"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := New(f.cfg); err == nil {
		t.Fatal("public state accepted")
	}
}

func TestConsumedCodeAndRefreshRemainConsumedAfterRestart(t *testing.T) {
	f := setup(t)
	code := f.code(t)
	// A restart between consent and exchange preserves the PKCE-bound code.
	var err error
	f.s, err = New(f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	w := f.request("POST", "/token", exchangeValues(code), "")
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	tokens := decode(t, w)
	f.s, err = New(f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	if f.request("POST", "/token", exchangeValues(code), "").Code != 400 {
		t.Fatal("consumed code resurrected")
	}
	refresh := tokens["refresh_token"].(string)
	if f.request("POST", "/token", refreshValues(refresh), "").Code != 200 {
		t.Fatal("refresh failed")
	}
	f.s, err = New(f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	if f.request("POST", "/token", refreshValues(refresh), "").Code != 400 {
		t.Fatal("consumed refresh resurrected")
	}
	if _, err := f.s.AuthenticateAccess(tokens["access_token"].(string)); err == nil {
		t.Fatal("replay revocation failed after restart")
	}
}

func TestWriteFailureDoesNotIssueCodeOrTokens(t *testing.T) {
	f := setup(t)
	if err := os.Mkdir(filepath.Join(f.cfg.StateDir, "oauth.json"), 0700); err != nil {
		t.Fatal(err)
	}
	w := f.request("GET", "/authorize?"+authValues().Encode(), nil, "")
	if w.Code != 500 || strings.Contains(w.Body.String(), "name=\"request\"") {
		t.Fatal("consent exposed without durable binding")
	}
	if err := os.Remove(filepath.Join(f.cfg.StateDir, "oauth.json")); err != nil {
		t.Fatal(err)
	}
	code := f.code(t)
	if err := os.Remove(filepath.Join(f.cfg.StateDir, "oauth.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(f.cfg.StateDir, "oauth.json"), 0700); err != nil {
		t.Fatal(err)
	}
	w = f.request("POST", "/token", exchangeValues(code), "")
	if w.Code != 500 || strings.Contains(w.Body.String(), "access_token") {
		t.Fatal("token exposed without durable consumption")
	}
}
