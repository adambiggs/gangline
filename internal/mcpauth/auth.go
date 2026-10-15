// Package mcpauth provides the public-client OAuth boundary for Gangline's MCP endpoint.
package mcpauth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

const scope = "gangline"
const accessLifetime = time.Hour
const refreshLifetime = 30 * 24 * time.Hour
const codeLifetime = 5 * time.Minute
const consentLifetime = 10 * time.Minute

// Config describes one explicitly configured public OAuth client. StateDir must
// be private and have a single owning Server process. Callbacks enforce the
// registration's current generation on every use, including after a restart.
type Config struct {
	Issuer, ClientID, RedirectURI, StateDir string
	AuthenticateCredential                  func(string) (string, error)
	Authorize                               func(string) error
	Clock                                   func() time.Time
}

type grant struct {
	ClientID    string    `json:"client_id"`
	RedirectURI string    `json:"redirect_uri"`
	Resource    string    `json:"resource"`
	State       string    `json:"state,omitempty"`
	Challenge   string    `json:"challenge,omitempty"`
	Principal   string    `json:"principal,omitempty"`
	Family      string    `json:"family,omitempty"`
	Used        bool      `json:"used,omitempty"`
	Expires     time.Time `json:"expires"`
}
type diskState struct {
	Issuer, ClientID, RedirectURI   string
	Consent, Codes, Access, Refresh map[string]grant
}

type Server struct {
	cfg   Config
	mu    sync.Mutex
	state diskState
}

// New opens the private OAuth state, refusing a state directory reused for a
// different issuer or client. Credentials and raw bearer tokens are never saved.
func New(cfg Config) (*Server, error) {
	issuer, err := url.Parse(cfg.Issuer)
	if err != nil || issuer.Scheme != "https" || issuer.Host == "" || issuer.User != nil || issuer.RawQuery != "" || issuer.Fragment != "" || issuer.Path != "" || issuer.RawPath != "" {
		return nil, errors.New("OAuth issuer must be an HTTPS origin without a trailing slash")
	}
	redirect, err := url.Parse(cfg.RedirectURI)
	if err != nil || redirect.Scheme != "https" || redirect.Host == "" || redirect.User != nil || redirect.Fragment != "" {
		return nil, errors.New("OAuth redirect must be an exact HTTPS callback URL")
	}
	if strings.TrimSpace(cfg.ClientID) == "" || cfg.StateDir == "" || cfg.AuthenticateCredential == nil || cfg.Authorize == nil {
		return nil, errors.New("OAuth client, private state directory, and authorization callbacks are required")
	}
	if cfg.Clock == nil {
		cfg.Clock = time.Now
	}
	if err := os.MkdirAll(cfg.StateDir, 0700); err != nil {
		return nil, err
	}
	info, err := os.Lstat(cfg.StateDir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return nil, errors.New("OAuth state directory must be private (0700)")
	}
	s := &Server{cfg: cfg, state: diskState{Issuer: cfg.Issuer, ClientID: cfg.ClientID, RedirectURI: cfg.RedirectURI, Consent: map[string]grant{}, Codes: map[string]grant{}, Access: map[string]grant{}, Refresh: map[string]grant{}}}
	path := filepath.Join(cfg.StateDir, "oauth.json")
	info, err = os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return nil, errors.New("OAuth state file must be private (0600)")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &s.state); err != nil {
		return nil, fmt.Errorf("read OAuth state: %w", err)
	}
	if s.state.Issuer != cfg.Issuer || s.state.ClientID != cfg.ClientID || s.state.RedirectURI != cfg.RedirectURI || s.state.Consent == nil || s.state.Codes == nil || s.state.Access == nil || s.state.Refresh == nil {
		return nil, errors.New("OAuth state does not match configured issuer and client")
	}
	return s, nil
}

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
func digest(token string) string { h := sha256.Sum256([]byte(token)); return hex.EncodeToString(h[:]) }

// persist commits state before any secret or successful response is released.
// Failed writes release no token or authorization code.
func (s *Server) persist() error {
	now := s.cfg.Clock()
	for _, records := range []map[string]grant{s.state.Consent, s.state.Codes, s.state.Access, s.state.Refresh} {
		for key, g := range records {
			if !now.Before(g.Expires) {
				delete(records, key)
			}
		}
	}
	data, err := json.Marshal(s.state)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(s.cfg.StateDir, ".oauth-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err := os.Rename(name, filepath.Join(s.cfg.StateDir, "oauth.json")); err != nil {
		return err
	}
	dir, err := os.Open(s.cfg.StateDir)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func (s *Server) resource() string { return s.cfg.Issuer + "/mcp" }
func (s *Server) Challenge() string {
	return `Bearer resource_metadata="` + s.cfg.Issuer + `/.well-known/oauth-protected-resource", scope="gangline"`
}

// AuthenticateAccess validates an opaque access token and the live registration.
func (s *Server) AuthenticateAccess(token string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, ok := s.state.Access[digest(token)]
	if !ok || !s.valid(g) {
		return "", errors.New("invalid access token")
	}
	if err := s.cfg.Authorize(g.Principal); err != nil {
		return "", errors.New("registration revoked")
	}
	return g.Principal, nil
}
func (s *Server) valid(g grant) bool {
	return g.ClientID == s.cfg.ClientID && g.RedirectURI == s.cfg.RedirectURI && g.Resource == s.resource() && s.cfg.Clock().Before(g.Expires)
}

func (s *Server) securityHeaders(w http.ResponseWriter) {
	callback, _ := url.Parse(s.cfg.RedirectURI)
	callbackOrigin := callback.Scheme + "://" + callback.Host
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Referrer-Policy", "same-origin")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; form-action 'self' "+callbackOrigin+"; frame-ancestors 'none'; base-uri 'none'")
}
func jsonResponse(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
func oauthError(w http.ResponseWriter, status int, code string) {
	jsonResponse(w, status, map[string]string{"error": code})
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.securityHeaders(w)
	switch r.URL.Path {
	case "/.well-known/oauth-protected-resource", "/.well-known/oauth-protected-resource/mcp":
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		jsonResponse(w, 200, map[string]any{"resource": s.resource(), "authorization_servers": []string{s.cfg.Issuer}, "scopes_supported": []string{scope}, "bearer_methods_supported": []string{"header"}})
	case "/.well-known/oauth-authorization-server":
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		jsonResponse(w, 200, map[string]any{"issuer": s.cfg.Issuer, "authorization_endpoint": s.cfg.Issuer + "/authorize", "token_endpoint": s.cfg.Issuer + "/token", "response_types_supported": []string{"code"}, "grant_types_supported": []string{"authorization_code", "refresh_token"}, "token_endpoint_auth_methods_supported": []string{"none"}, "code_challenge_methods_supported": []string{"S256"}, "scopes_supported": []string{scope}, "authorization_response_iss_parameter_supported": true})
	case "/authorize":
		s.authorize(w, r)
	case "/token":
		s.token(w, r)
	default:
		http.NotFound(w, r)
	}
}

var pkce = regexp.MustCompile(`^[A-Za-z0-9._~-]{43,128}$`)
var challenge = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)

func single(values url.Values, keys ...string) bool {
	for _, key := range keys {
		if len(values[key]) != 1 {
			return false
		}
	}
	return true
}
func (s *Server) authorizationRequest(v url.Values) (grant, bool) {
	if !single(v, "response_type", "client_id", "redirect_uri", "resource", "state", "code_challenge", "code_challenge_method") || len(v["scope"]) > 1 {
		return grant{}, false
	}
	if v.Get("response_type") != "code" || v.Get("client_id") != s.cfg.ClientID || v.Get("redirect_uri") != s.cfg.RedirectURI || v.Get("resource") != s.resource() || v.Get("state") == "" || v.Get("code_challenge_method") != "S256" || !challenge.MatchString(v.Get("code_challenge")) || (v.Get("scope") != "" && v.Get("scope") != scope) {
		return grant{}, false
	}
	return grant{ClientID: s.cfg.ClientID, RedirectURI: s.cfg.RedirectURI, Resource: s.resource(), State: v.Get("state"), Challenge: v.Get("code_challenge"), Expires: s.cfg.Clock().Add(consentLifetime)}, true
}

var consentForm = template.Must(template.New("consent").Parse(`<!doctype html><html><head><meta charset="utf-8"><title>Connect Gangline</title></head><body><h1>Connect Gangline</h1><p>Allow {{.Client}} to send and receive messages as the outside registration identified by your credential.</p><p>Only continue if you initiated this connection in ChatGPT.</p><form method="post" action="/authorize" autocomplete="off"><input type="hidden" name="request" value="{{.Request}}"><label>Registration credential <input type="password" name="credential" required autocomplete="new-password" spellcheck="false"></label><button type="submit">Connect</button></form></body></html>`))

func (s *Server) authorize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if r.Method == http.MethodGet {
		g, ok := s.authorizationRequest(r.URL.Query())
		if !ok {
			oauthError(w, 400, "invalid_request")
			return
		}
		nonce, err := randomToken()
		if err != nil {
			oauthError(w, 500, "server_error")
			return
		}
		s.state.Consent[digest(nonce)] = g
		if err := s.persist(); err != nil {
			delete(s.state.Consent, digest(nonce))
			oauthError(w, 500, "server_error")
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = consentForm.Execute(w, struct{ Client, Request string }{s.cfg.ClientID, nonce})
		return
	}
	// The nonce binds this form to the displayed OAuth request. Origin validation
	// rejects cross-site form POSTs; authorization links can originate elsewhere.
	if r.Header.Get("Origin") != s.cfg.Issuer || (r.Header.Get("Sec-Fetch-Site") != "" && r.Header.Get("Sec-Fetch-Site") != "same-origin") || r.URL.RawQuery != "" || r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
		oauthError(w, 400, "invalid_request")
		return
	}
	if err := r.ParseForm(); err != nil || !single(r.PostForm, "request", "credential") || len(r.PostForm) != 2 {
		oauthError(w, 400, "invalid_request")
		return
	}
	key := digest(r.PostForm.Get("request"))
	g, ok := s.state.Consent[key]
	if !ok || !s.valid(g) {
		oauthError(w, 400, "invalid_request")
		return
	}
	principal, err := s.cfg.AuthenticateCredential(r.PostForm.Get("credential"))
	if err != nil || principal == "" {
		oauthError(w, 403, "access_denied")
		return
	}
	if err := s.cfg.Authorize(principal); err != nil {
		oauthError(w, 403, "access_denied")
		return
	}
	code, err := randomToken()
	if err != nil {
		oauthError(w, 500, "server_error")
		return
	}
	delete(s.state.Consent, key)
	g.Principal = principal
	g.Family = digest(code)
	g.Expires = s.cfg.Clock().Add(codeLifetime)
	s.state.Codes[digest(code)] = g
	if err := s.persist(); err != nil {
		delete(s.state.Codes, digest(code))
		oauthError(w, 500, "server_error")
		return
	}
	callback, _ := url.Parse(s.cfg.RedirectURI)
	q := callback.Query()
	q.Set("code", code)
	q.Set("state", g.State)
	q.Set("iss", s.cfg.Issuer)
	callback.RawQuery = q.Encode()
	http.Redirect(w, r, callback.String(), http.StatusSeeOther)
}

func (s *Server) token(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if r.URL.RawQuery != "" || r.Header.Get("Authorization") != "" {
		oauthError(w, 400, "invalid_request")
		return
	}
	if err := r.ParseForm(); err != nil {
		oauthError(w, 400, "invalid_request")
		return
	}
	v := r.PostForm
	if !single(v, "client_id", "resource", "grant_type") || v.Get("client_id") != s.cfg.ClientID || v.Get("resource") != s.resource() || len(v["client_secret"]) != 0 || len(v["client_assertion"]) != 0 {
		oauthError(w, 400, "invalid_request")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var g grant
	var records map[string]grant
	var key string
	switch v.Get("grant_type") {
	case "authorization_code":
		if !single(v, "code", "redirect_uri", "code_verifier") || v.Get("redirect_uri") != s.cfg.RedirectURI || !pkce.MatchString(v.Get("code_verifier")) {
			oauthError(w, 400, "invalid_grant")
			return
		}
		records = s.state.Codes
		key = digest(v.Get("code"))
		g = records[key]
		h := sha256.Sum256([]byte(v.Get("code_verifier")))
		if base64.RawURLEncoding.EncodeToString(h[:]) != g.Challenge {
			oauthError(w, 400, "invalid_grant")
			return
		}
	case "refresh_token":
		if !single(v, "refresh_token") || len(v["scope"]) > 1 || (v.Get("scope") != "" && v.Get("scope") != scope) {
			oauthError(w, 400, "invalid_grant")
			return
		}
		records = s.state.Refresh
		key = digest(v.Get("refresh_token"))
		g = records[key]
	default:
		oauthError(w, 400, "unsupported_grant_type")
		return
	}
	if g.Used && s.valid(g) {
		for _, familyRecords := range []map[string]grant{s.state.Access, s.state.Refresh} {
			for tokenKey, record := range familyRecords {
				if record.Family == g.Family {
					delete(familyRecords, tokenKey)
				}
			}
		}
		if err := s.persist(); err != nil {
			oauthError(w, 500, "server_error")
			return
		}
		oauthError(w, 400, "invalid_grant")
		return
	}
	if !s.valid(g) || g.Principal == "" {
		oauthError(w, 400, "invalid_grant")
		return
	}
	if err := s.cfg.Authorize(g.Principal); err != nil {
		oauthError(w, 400, "invalid_grant")
		return
	}
	access, err := randomToken()
	if err != nil {
		oauthError(w, 500, "server_error")
		return
	}
	refresh, err := randomToken()
	if err != nil {
		oauthError(w, 500, "server_error")
		return
	}
	if v.Get("grant_type") == "refresh_token" {
		g.Used = true
		records[key] = g
		g.Used = false
	} else {
		delete(records, key)
	}
	g.State = ""
	g.Challenge = ""
	g.Expires = s.cfg.Clock().Add(accessLifetime)
	s.state.Access[digest(access)] = g
	g.Expires = s.cfg.Clock().Add(refreshLifetime)
	s.state.Refresh[digest(refresh)] = g
	if err := s.persist(); err != nil {
		delete(s.state.Access, digest(access))
		delete(s.state.Refresh, digest(refresh))
		oauthError(w, 500, "server_error")
		return
	}
	jsonResponse(w, 200, map[string]any{"access_token": access, "refresh_token": refresh, "token_type": "Bearer", "expires_in": int(accessLifetime.Seconds()), "scope": scope, "resource": s.resource()})
}
