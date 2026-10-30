// Package mcpevents provides authenticated MCP webhook subscriptions for mailbox wakeups.
package mcpevents

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const EventName = "message.created"
const defaultTTL = 24 * time.Hour
const rotationWindow = 5 * time.Minute
const verificationLifetime = 5 * time.Minute

// The MCP Events integration accepts request bodies of at most 256 KiB.
const maxBody = 256 * 1024

type Config struct {
	StatePath string
	Now       func() time.Time
	// Authorize checks the registration and credential generation, returning false only for confirmed revocation.
	Authorize func(context.Context, string) (bool, error)
}

type Error struct {
	Code    int               `json:"code"`
	Message string            `json:"message"`
	Data    map[string]string `json:"data,omitempty"`
}

func (e *Error) Error() string                { return e.Message }
func (e *Error) RPCError() (int, string, any) { return e.Code, e.Message, e.Data }
func invalid(message string) error            { return &Error{Code: -32602, Message: message} }
func callbackError(reason string) error {
	return &Error{Code: -32015, Message: "Callback endpoint verification failed", Data: map[string]string{"reason": reason}}
}

type subscription struct {
	ID          string            `json:"id"`
	Principal   string            `json:"principal"`
	URL         string            `json:"url"`
	Secret      string            `json:"secret"`
	OldSecret   string            `json:"oldSecret,omitempty"`
	RotateUntil time.Time         `json:"rotateUntil,omitempty"`
	CreatedAt   time.Time         `json:"createdAt"`
	ExpiresAt   *time.Time        `json:"expiresAt"`
	Receipts    map[string]string `json:"receipts"`
}
type state struct {
	Key           []byte                  `json:"key"`
	Subscriptions map[string]subscription `json:"subscriptions"`
}
type Manager struct {
	mu         sync.Mutex
	deliveryMu sync.Mutex
	config     Config
	state      state
	client     *http.Client
	verified   map[string]time.Time
	wait       func(context.Context, time.Duration) error
}

// New opens private subscription state. One Manager owns a state path at a time.
func New(config Config) (*Manager, error) {
	if config.StatePath == "" || config.Authorize == nil {
		return nil, errors.New("events require a state path and authorization callback")
	}
	if config.Now == nil {
		config.Now = time.Now
	}
	m := &Manager{config: config, client: callbackClient(), verified: map[string]time.Time{}, wait: waitContext}
	raw, err := os.ReadFile(config.StatePath)
	if err == nil {
		if err = json.Unmarshal(raw, &m.state); err != nil {
			return nil, fmt.Errorf("read event subscriptions: %w", err)
		}
		if len(m.state.Key) != 32 || m.state.Subscriptions == nil {
			return nil, errors.New("invalid event subscription state")
		}
		for id, s := range m.state.Subscriptions {
			if s.ID != id || s.Principal == "" || s.CreatedAt.IsZero() || validateURL(s.URL) != nil || validSecret(s.Secret) != nil || (s.OldSecret != "" && validSecret(s.OldSecret) != nil) {
				return nil, errors.New("invalid stored event subscription")
			}
			if s.Receipts == nil {
				s.Receipts = map[string]string{}
				m.state.Subscriptions[id] = s
			}
		}
	} else if errors.Is(err, os.ErrNotExist) {
		m.state = state{Key: make([]byte, 32), Subscriptions: map[string]subscription{}}
		if _, err = rand.Read(m.state.Key); err != nil {
			return nil, err
		}
		if err = m.saveLocked(); err != nil {
			return nil, err
		}
	} else {
		return nil, err
	}
	return m, nil
}
func waitContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (m *Manager) saveLocked() error {
	raw, err := json.Marshal(m.state)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(m.config.StatePath), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(m.config.StatePath), ".events-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(raw)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(name, m.config.StatePath)
	}
	return err
}
func (m *Manager) identity(principal, callback string) string {
	h := hmac.New(sha256.New, m.state.Key)
	// A JSON tuple makes component boundaries unambiguous.
	raw, _ := json.Marshal([]string{principal, callback, EventName, "{}"})
	h.Write(raw)
	return "sub_" + hex.EncodeToString(h.Sum(nil))
}
func validSecret(secret string) error {
	if !strings.HasPrefix(secret, "whsec_") {
		return invalid("delivery.secret must be a whsec_ signing key")
	}
	key, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(secret, "whsec_"))
	if err != nil || len(key) < 24 || len(key) > 64 {
		return invalid("delivery.secret must encode 24–64 bytes")
	}
	return nil
}
func validateURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
		return invalid("delivery.url must be an HTTPS callback without credentials or fragment")
	}
	if port := u.Port(); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return invalid("delivery.url has an invalid port")
		}
	}
	return nil
}

type parameters struct {
	Meta      json.RawMessage `json:"_meta"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
	Delivery  struct {
		Mode   string `json:"mode"`
		URL    string `json:"url"`
		Secret string `json:"secret,omitempty"`
	} `json:"delivery"`
	TTL    json.RawMessage `json:"ttlMs"`
	Cursor json.RawMessage `json:"cursor"`
}

func decode(raw json.RawMessage, v any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return invalid("invalid event parameters")
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return invalid("invalid event parameters")
	}
	return nil
}
func parseParameters(raw json.RawMessage) (parameters, error) {
	var p parameters
	if err := decode(raw, &p); err != nil {
		return p, err
	}
	if p.Name != EventName {
		return p, invalid("unknown event name")
	}
	if len(p.Arguments) > 0 {
		var args map[string]any
		if err := json.Unmarshal(p.Arguments, &args); err != nil || args == nil || len(args) != 0 {
			return p, invalid("message.created accepts an empty arguments object")
		}
	}
	if p.Delivery.Mode != "webhook" {
		return p, invalid("only webhook delivery is supported")
	}
	if err := validateURL(p.Delivery.URL); err != nil {
		return p, err
	}
	if len(p.Cursor) > 0 && string(bytes.TrimSpace(p.Cursor)) != "null" {
		return p, invalid("message.created does not support replay cursors")
	}
	return p, nil
}

func (m *Manager) authorize(ctx context.Context, principal string) error {
	if principal != "" {
		authorized, err := m.config.Authorize(ctx, principal)
		if err != nil {
			return err
		}
		if authorized {
			return nil
		}
	}
	return &Error{Code: -32001, Message: "Registration is not authorized"}
}

func (m *Manager) RPC(ctx context.Context, principal, method string, raw json.RawMessage) (any, error) {
	if err := m.authorize(ctx, principal); err != nil {
		return nil, err
	}
	if method == "events/list" {
		var p struct {
			Meta   json.RawMessage `json:"_meta"`
			Cursor json.RawMessage `json:"cursor"`
		}
		if len(raw) > 0 && string(raw) != "null" {
			if err := decode(raw, &p); err != nil {
				return nil, err
			}
		}
		if len(p.Cursor) > 0 && string(p.Cursor) != "null" {
			return nil, invalid("invalid event catalog cursor")
		}
		return map[string]any{"resultType": "complete", "events": []any{map[string]any{
			"name": EventName, "description": "New mail is available in this registration's mailbox. Fetch pending mail to read it.", "delivery": []string{"webhook"},
			"inputSchema":   map[string]any{"type": "object", "properties": map[string]any{}, "additionalProperties": false},
			"payloadSchema": map[string]any{"type": "object", "properties": map[string]any{"id": map[string]any{"type": "string"}}, "required": []string{"id"}, "additionalProperties": false},
		}}}, nil
	}
	if method != "events/subscribe" && method != "events/unsubscribe" {
		return nil, &Error{Code: -32601, Message: "Method not found"}
	}
	p, err := parseParameters(raw)
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	id := m.identity(principal, p.Delivery.URL)
	if method == "events/unsubscribe" {
		old, exists := m.state.Subscriptions[id]
		delete(m.state.Subscriptions, id)
		if err := m.saveLocked(); err != nil {
			if exists {
				m.state.Subscriptions[id] = old
			}
			return nil, err
		}
		return map[string]any{"resultType": "complete"}, nil
	}
	if err := validSecret(p.Delivery.Secret); err != nil {
		return nil, err
	}
	now := m.config.Now()
	var expires *time.Time
	if len(p.TTL) == 0 {
		t := now.Add(defaultTTL)
		expires = &t
	} else if string(bytes.TrimSpace(p.TTL)) != "null" {
		var ms int64
		if json.Unmarshal(p.TTL, &ms) != nil || ms <= 0 || ms > int64((time.Duration(1<<63-1))/time.Millisecond) {
			return nil, invalid("ttlMs must be a positive millisecond duration or null")
		}
		t := now.Add(time.Duration(ms) * time.Millisecond)
		expires = &t
	}
	for key, until := range m.verified {
		if !until.After(now) {
			delete(m.verified, key)
		}
	}
	cacheKey := principal + "\x00" + p.Delivery.URL
	if !m.verified[cacheKey].After(now) {
		if err := m.verify(ctx, id, p.Delivery.URL, p.Delivery.Secret); err != nil {
			return nil, err
		}
		m.verified[cacheKey] = m.config.Now().Add(verificationLifetime)
	}
	if err := m.authorize(ctx, principal); err != nil {
		return nil, err
	}
	old, exists := m.state.Subscriptions[id]
	s := old
	if !exists || expired(s, now) {
		s = subscription{ID: id, Principal: principal, URL: p.Delivery.URL, CreatedAt: now, Receipts: map[string]string{}}
	}
	if s.Secret != "" && s.Secret != p.Delivery.Secret {
		s.OldSecret = s.Secret
		s.RotateUntil = now.Add(rotationWindow)
	}
	s.Secret = p.Delivery.Secret
	s.ExpiresAt = expires
	if !s.RotateUntil.After(now) {
		s.OldSecret = ""
		s.RotateUntil = time.Time{}
	}
	m.state.Subscriptions[id] = s
	if err := m.saveLocked(); err != nil {
		if exists {
			m.state.Subscriptions[id] = old
		} else {
			delete(m.state.Subscriptions, id)
		}
		return nil, err
	}
	return map[string]any{"resultType": "complete", "id": id, "refreshBefore": expires, "cursor": nil, "truncated": false}, nil
}
func expired(s subscription, now time.Time) bool {
	return s.ExpiresAt != nil && !s.ExpiresAt.After(now)
}

// ActivePrincipals returns registrations with an unexpired, authorized subscription.
func (m *Manager) ActivePrincipals(ctx context.Context) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	seen := map[string]bool{}
	changed := false
	old := m.state.Subscriptions
	next := make(map[string]subscription, len(old))
	for id, s := range old {
		if expired(s, m.config.Now()) {
			changed = true
			continue
		}
		authorized, err := m.config.Authorize(ctx, s.Principal)
		if err != nil {
			return nil, err
		}
		if !authorized {
			changed = true
			continue
		}
		if s.OldSecret != "" && !s.RotateUntil.After(m.config.Now()) {
			s.OldSecret = ""
			s.RotateUntil = time.Time{}
			changed = true
		}
		next[id] = s
		seen[s.Principal] = true
	}
	if changed {
		m.state.Subscriptions = next
		if err := m.saveLocked(); err != nil {
			m.state.Subscriptions = old
			return nil, err
		}
	}
	names := make([]string, 0, len(seen))
	for p := range seen {
		names = append(names, p)
	}
	sort.Strings(names)
	return names, nil
}

// Prune removes delivery receipts for mail that is no longer pending.
func (m *Manager) Prune(principal string, pendingIDs []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	pending := map[string]bool{}
	for _, id := range pendingIDs {
		pending[id] = true
	}
	old := m.state.Subscriptions
	next := make(map[string]subscription, len(old))
	changed := false
	for id, s := range old {
		if s.Principal == principal {
			delivered := map[string]string{}
			for eventID := range s.Receipts {
				if pending[eventID] {
					delivered[eventID] = s.Receipts[eventID]
				} else {
					changed = true
				}
			}
			s.Receipts = delivered
		}
		next[id] = s
	}
	if !changed {
		return nil
	}
	m.state.Subscriptions = next
	if err := m.saveLocked(); err != nil {
		m.state.Subscriptions = old
		return err
	}
	return nil
}

// Deliver sends a wakeup to matching subscriptions, preserving event identity on retries.
// Accepted and terminal-failure receipts suppress repeated pump attempts.
// Prune removes receipts after the corresponding mail leaves the mailbox.
func (m *Manager) Deliver(ctx context.Context, principal, eventID string, timestamp time.Time) error {
	if eventID == "" || timestamp.IsZero() {
		return errors.New("event ID and occurrence time are required")
	}
	m.deliveryMu.Lock()
	defer m.deliveryMu.Unlock()
	m.mu.Lock()
	ids := []string{}
	for id, s := range m.state.Subscriptions {
		if s.Principal == principal {
			ids = append(ids, id)
		}
	}
	m.mu.Unlock()
	sort.Strings(ids)
	body, err := json.Marshal(map[string]any{"eventId": eventID, "name": EventName, "timestamp": timestamp.UTC().Format(time.RFC3339Nano), "data": map[string]string{"id": eventID}, "cursor": nil})
	if err != nil {
		return err
	}
	if len(body) > maxBody {
		return errors.New("event exceeds MCP callback body limit")
	}
	var failures []error
	for _, id := range ids {
		for attempt := 0; attempt < 3; attempt++ {
			m.mu.Lock()
			s, exists := m.state.Subscriptions[id]
			m.mu.Unlock()
			if !exists || expired(s, m.config.Now()) || s.Receipts[eventID] != "" || timestamp.Before(s.CreatedAt) {
				break
			}
			authorized, err := m.config.Authorize(ctx, principal)
			if err != nil {
				failures = append(failures, err)
				return errors.Join(failures...)
			}
			if !authorized {
				break
			}
			status, _, err := m.post(ctx, s, eventID, body)
			if err == nil && status >= 200 && status < 300 {
				if saveErr := m.settle(id, eventID, "accepted", false); saveErr != nil {
					failures = append(failures, saveErr)
				}
				break
			}
			transient := err != nil || status == 408 || status == 429 || status >= 500
			if !transient || attempt == 2 {
				reason := responseReason(status, err)
				failures = append(failures, fmt.Errorf("callback delivery failed: %s", reason))
				if saveErr := m.settle(id, eventID, reason, status == 410); saveErr != nil {
					failures = append(failures, saveErr)
				}
				break
			}
			if err := m.wait(ctx, time.Second*time.Duration(1<<attempt)); err != nil {
				return err
			}
		}
	}
	return errors.Join(failures...)
}
func (m *Manager) settle(id, eventID, outcome string, remove bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	current, ok := m.state.Subscriptions[id]
	if !ok {
		return nil
	}
	if remove {
		delete(m.state.Subscriptions, id)
	} else {
		receipts := make(map[string]string, len(current.Receipts)+1)
		for k, v := range current.Receipts {
			receipts[k] = v
		}
		receipts[eventID] = outcome
		next := current
		next.Receipts = receipts
		m.state.Subscriptions[id] = next
	}
	if err := m.saveLocked(); err != nil {
		m.state.Subscriptions[id] = current
		return err
	}
	return nil
}

func (m *Manager) verify(ctx context.Context, id, callback, secret string) error {
	challengeBytes := make([]byte, 32)
	if _, err := rand.Read(challengeBytes); err != nil {
		return err
	}
	challenge := base64.RawURLEncoding.EncodeToString(challengeBytes)
	body, _ := json.Marshal(map[string]string{"type": "verification", "challenge": challenge})
	status, reply, err := m.post(ctx, subscription{ID: id, URL: callback, Secret: secret}, "msg_verification_"+challenge, body)
	if err != nil {
		return callbackError(responseReason(status, err))
	}
	if status < 200 || status >= 300 {
		return callbackError("http_status")
	}
	var echoed struct {
		Challenge string `json:"challenge"`
	}
	if json.Unmarshal(reply, &echoed) != nil || subtle.ConstantTimeCompare([]byte(echoed.Challenge), []byte(challenge)) != 1 {
		return callbackError("challenge_failed")
	}
	return nil
}
func (m *Manager) post(ctx context.Context, s subscription, id string, body []byte) (int, []byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, s.URL, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	now := m.config.Now()
	stamp := strconv.FormatInt(now.Unix(), 10)
	signature := sign(s.Secret, id, stamp, body)
	if s.OldSecret != "" && s.RotateUntil.After(now) {
		signature += " " + sign(s.OldSecret, id, stamp, body)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("webhook-id", id)
	request.Header.Set("webhook-timestamp", stamp)
	request.Header.Set("webhook-signature", signature)
	request.Header.Set("X-MCP-Subscription-Id", s.ID)
	response, err := m.client.Do(request)
	if err != nil {
		return 0, nil, err
	}
	defer response.Body.Close()
	reply, err := io.ReadAll(io.LimitReader(response.Body, maxBody+1))
	if err != nil {
		return response.StatusCode, nil, err
	}
	if len(reply) > maxBody {
		return response.StatusCode, nil, errors.New("callback response exceeds body limit")
	}
	return response.StatusCode, reply, nil
}
func sign(secret, id, stamp string, body []byte) string {
	key, _ := base64.StdEncoding.DecodeString(strings.TrimPrefix(secret, "whsec_"))
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(id + "." + stamp + "."))
	mac.Write(body)
	return "v1," + base64.StdEncoding.EncodeToString(mac.Sum(nil))
}
func responseReason(status int, err error) string {
	if err == nil {
		return "http_status_" + strconv.Itoa(status)
	}
	var denied *blockedAddressError
	if errors.As(err, &denied) {
		return "blocked_address"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	var timeout interface{ Timeout() bool }
	if errors.As(err, &timeout) && timeout.Timeout() {
		return "timeout"
	}
	return "unreachable"
}
