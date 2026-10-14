package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
)

const peerTestID = "msg-0123456789abcdef0123456789abcdef"

func peerRequestJSON(t *testing.T, edit func(map[string]any)) string {
	t.Helper()
	request := map[string]any{"host": "alpha", "team": "unit", "id": peerTestID, "from": "lead", "to": "worker", "body": "status report"}
	if edit != nil {
		edit(request)
	}
	data, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func peerFixture(t *testing.T) (*stateFixture, core.Agent) {
	t.Helper()
	f := newStateFixture(t)
	f.env["GANG_PEERS"] = `{"alpha":{"ssh":"alpha","user":"adam","gang":"/opt/gang","team":"unit"}}`
	a := f.add(t, "worker-id", "worker", "codex")
	f.env["GANGLINE_HITCH_ID"] = string(a.ID)
	return f, a
}

func peerReceive(f *stateFixture, request string) (peerReply, error) {
	f.out.Reset()
	cmd := f.cmd
	cmd.stdin = strings.NewReader(request)
	err := cmd.execute([]string{"peer-receive"})
	var reply peerReply
	if decodeErr := json.Unmarshal([]byte(f.out.String()), &reply); decodeErr != nil {
		return reply, errors.Join(err, decodeErr)
	}
	return reply, err
}

func TestPeerReceiveDeliversHostQualifiedSender(t *testing.T) {
	f, a := peerFixture(t)
	reply, err := peerReceive(f, peerRequestJSON(t, nil))
	if err != nil {
		t.Fatal(err)
	}
	if reply.ID != "peer-alpha."+peerTestID || reply.Outcome != "delivered" || reply.Error != "" {
		t.Fatalf("reply = %+v", reply)
	}
	p, err := f.run.team.Agent(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	e, err := p.ReadEnvelope("cur", reply.ID)
	if err != nil {
		t.Fatal(err)
	}
	if e.From != (core.Sender{Kind: core.SenderPeer, Name: "lead", Host: "alpha"}) {
		t.Fatalf("sender = %+v", e.From)
	}
	// The receiver stamps its own clock and token; the sender supplies neither.
	if !e.CreatedAt.Equal(f.cmd.now()) || len(e.Token) != 16 {
		t.Fatalf("envelope = %+v", e)
	}
	if !strings.HasPrefix(f.input.pasted, "[gang:alpha:lead#"+e.Token+"] status report") {
		t.Fatalf("pasted = %q", f.input.pasted)
	}
	if strings.Contains(f.input.pasted, "self-declared") {
		t.Fatalf("peer sender rendered as self-declared: %q", f.input.pasted)
	}
}

func TestPeerReceiveReportsUnverifiedWithItsID(t *testing.T) {
	f, _ := peerFixture(t)
	f.input.submit = func(string) error { return nil }
	// The delivery budget is already spent, so the witness wait ends at once
	// with no hook having fired.
	f.cmd.newTimeout = func(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
		if d != operationTimeout {
			return context.WithTimeout(ctx, d)
		}
		return context.WithDeadline(ctx, time.Time{})
	}
	reply, err := peerReceive(f, peerRequestJSON(t, nil))
	var status commandError
	if !errors.As(err, &status) || status.status != exitUnknown {
		t.Fatalf("error = %v", err)
	}
	if reply.ID != "peer-alpha."+peerTestID || reply.Outcome != "unverified" || reply.Error != "" {
		t.Fatalf("reply = %+v", reply)
	}
}

func TestPeerReceiveRefusesForgedOrMalformedRequests(t *testing.T) {
	set := func(key string, value any) func(map[string]any) {
		return func(r map[string]any) { r[key] = value }
	}
	for _, tc := range []struct {
		name    string
		request func(*testing.T) string
		setup   func(*stateFixture)
		want    string
	}{
		{"unenrolled host", func(t *testing.T) string { return peerRequestJSON(t, set("host", "beta")) }, nil, "not enrolled"},
		{"no peers configured", func(t *testing.T) string { return peerRequestJSON(t, nil) }, func(f *stateFixture) { delete(f.env, "GANG_PEERS") }, "not enrolled"},
		{"local name label", func(t *testing.T) string { return peerRequestJSON(t, set("from", "alpha:lead")) }, nil, "not an agent name"},
		{"self-declared label", func(t *testing.T) string { return peerRequestJSON(t, set("from", "self-declared:lead")) }, nil, "not an agent name"},
		{"reserved sender", func(t *testing.T) string { return peerRequestJSON(t, set("from", "gangline")) }, nil, "not an agent name"},
		{"other team", func(t *testing.T) string { return peerRequestJSON(t, set("team", "other")) }, nil, "not the selected team"},
		{"derived id", func(t *testing.T) string { return peerRequestJSON(t, set("id", "failed-"+peerTestID)) }, nil, "message id"},
		{"short id", func(t *testing.T) string { return peerRequestJSON(t, set("id", "msg-0123")) }, nil, "message id"},
		{"path id", func(t *testing.T) string { return peerRequestJSON(t, set("id", "../"+peerTestID)) }, nil, "message id"},
		{"bad recipient", func(t *testing.T) string { return peerRequestJSON(t, set("to", "../worker")) }, nil, "recipient"},
		{"empty body", func(t *testing.T) string { return peerRequestJSON(t, set("body", "")) }, nil, "body is empty"},
		{"control body", func(t *testing.T) string { return peerRequestJSON(t, set("body", "a\x1b[2Jb")) }, nil, "terminal controls"},
		{"oversized body", func(t *testing.T) string {
			return peerRequestJSON(t, set("body", strings.Repeat("x", maximumMessageBytes+1)))
		}, nil, "budget"},
		{"unknown field", func(t *testing.T) string { return peerRequestJSON(t, set("kind", "agent")) }, nil, "unknown field"},
		{"trailing data", func(t *testing.T) string { return peerRequestJSON(t, nil) + "{}" }, nil, "after its JSON object"},
		{"oversized request", func(t *testing.T) string { return strings.Repeat(" ", maximumPeerRequestBytes+1) }, nil, "exceeds"},
		{"inside agent pane", func(t *testing.T) string { return peerRequestJSON(t, nil) }, func(f *stateFixture) {
			f.add(t, "lead-id", "lead", "codex")
			f.env["GANG_AGENT_ID"] = "lead-id"
		}, "registered agent lead"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, a := peerFixture(t)
			if tc.setup != nil {
				tc.setup(f)
			}
			reply, err := peerReceive(f, tc.request(t))
			var refusal commandError
			if !errors.As(err, &refusal) || refusal.status != exitRefused || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want refusal containing %q", err, tc.want)
			}
			if reply.ID != "" || reply.Outcome != "" || !strings.Contains(reply.Error, tc.want) {
				t.Fatalf("reply = %+v", reply)
			}
			p, err := f.run.team.Agent(a.ID)
			if err != nil {
				t.Fatal(err)
			}
			pending, err := p.ListNew()
			if err != nil || len(pending) != 0 || f.input.submits != 0 {
				t.Fatalf("refused request published: %+v submits=%d err=%v", pending, f.input.submits, err)
			}
		})
	}
}

func TestPeerReceiveNeverReplacesAPublishedMessage(t *testing.T) {
	f, a := peerFixture(t)
	if _, err := peerReceive(f, peerRequestJSON(t, nil)); err != nil {
		t.Fatal(err)
	}
	reply, err := peerReceive(f, peerRequestJSON(t, func(r map[string]any) { r["body"] = "replacement" }))
	if err == nil || !strings.Contains(err.Error(), "already published") || reply.ID != "" {
		t.Fatalf("duplicate: reply=%+v err=%v", reply, err)
	}
	p, _ := f.run.team.Agent(a.ID)
	e, err := p.ReadEnvelope("cur", "peer-alpha."+peerTestID)
	if err != nil || e.Message.Text != "status report" || f.input.submits != 1 {
		t.Fatalf("original = %+v err=%v submits=%d", e, err, f.input.submits)
	}
}

// A peer label is HOST:NAME. No local identity can render that form.
func TestLocalSendersCannotTakeAPeerLabel(t *testing.T) {
	if _, err := parseSend([]string{"worker", "--from", "alpha:lead", "hi"}); err == nil {
		t.Fatal("--from accepted a peer label")
	}
	if validAgentName("alpha:lead") {
		t.Fatal("a local agent may be named with a peer label")
	}
	local := senderLabel(core.Sender{Kind: core.SenderAgent, Name: "alpha-lead"})
	peer := senderLabel(core.Sender{Kind: core.SenderPeer, Name: "lead", Host: "alpha"})
	if peer != "alpha:lead" || local == peer {
		t.Fatalf("local %q, peer %q", local, peer)
	}
}

func TestPeerSendersDifferByHost(t *testing.T) {
	alpha := core.Sender{Kind: core.SenderPeer, Name: "lead", Host: "alpha"}
	beta := core.Sender{Kind: core.SenderPeer, Name: "lead", Host: "beta"}
	if alpha.SameIdentity(beta) || !alpha.SameIdentity(alpha) {
		t.Fatal("peer identity ignores its host")
	}
}

func TestPeersConfiguration(t *testing.T) {
	directory := t.TempDir()
	cmd := command{
		getenv:      func(k string) string { return map[string]string{"GANG_CONFIG_DIR": directory}[k] },
		userHomeDir: func() (string, error) { return directory, nil },
	}
	// A host without the key keeps every setting it had.
	got, err := cmd.settings()
	if err != nil || got.Peers != nil || got.PeersJSON != "" {
		t.Fatalf("absent key: %+v %v", got, err)
	}
	write := func(value string) {
		if err := os.WriteFile(filepath.Join(directory, "config"), []byte("GANG_PEERS="+value+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(`{"alpha":{"ssh":"alpha","gang":"/usr/local/bin/gang","team":"gangline"},"beta":{"ssh":"beta","user":"beta","gang":"/home/beta/.local/bin/gang","team":"gang.line_2"}}`)
	got, err = cmd.settings()
	if err != nil || got.Peers["beta"] != (peerHost{SSH: "beta", User: "beta", Gang: "/home/beta/.local/bin/gang", Team: "gang.line_2"}) || got.Peers["alpha"].SSH != "alpha" || got.Origins["GANG_PEERS"] != "config" {
		t.Fatalf("peers = %+v origin=%q err=%v", got.Peers, got.Origins["GANG_PEERS"], err)
	}
	for name, value := range map[string]string{
		"self-declared host": `{"self-declared":{"ssh":"x","gang":"/g","team":"t"}}`,
		"self-declared case": `{"Self-Declared":{"ssh":"x","gang":"/g","team":"t"}}`,
		"label host":         `{"a:b":{"ssh":"x","gang":"/g","team":"t"}}`,
		"reserved host":      `{"gangline":{"ssh":"x","gang":"/g","team":"t"}}`,
		"option alias":       `{"alpha":{"ssh":"-oProxyCommand=x","gang":"/g","team":"t"}}`,
		"missing alias":      `{"alpha":{"gang":"/g","team":"t"}}`,
		"spaced user":        `{"alpha":{"ssh":"alpha","user":"a b","gang":"/g","team":"t"}}`,
		"missing gang":       `{"alpha":{"ssh":"alpha","team":"t"}}`,
		"relative gang":      `{"alpha":{"ssh":"alpha","gang":"gang","team":"t"}}`,
		"home gang":          `{"alpha":{"ssh":"alpha","gang":"~/bin/gang","team":"t"}}`,
		"shell in gang":      `{"alpha":{"ssh":"alpha","gang":"/bin/gang;id","team":"t"}}`,
		"spaced gang":        `{"alpha":{"ssh":"alpha","gang":"/my bin/gang","team":"t"}}`,
		"missing team":       `{"alpha":{"ssh":"alpha","gang":"/g"}}`,
		"option team":        `{"alpha":{"ssh":"alpha","gang":"/g","team":"-x"}}`,
		"shell in team":      `{"alpha":{"ssh":"alpha","gang":"/g","team":"$(id)"}}`,
		"unknown field":      `{"alpha":{"ssh":"alpha","gang":"/g","team":"t","kind":"x"}}`,
		"not an object":      `["alpha"]`,
		"null":               `null`,
		"trailing data":      `{"alpha":{"ssh":"alpha","gang":"/g","team":"t"}} {}`,
		"trailing bracket":   `{"alpha":{"ssh":"alpha","gang":"/g","team":"t"}}}`,
	} {
		write(value)
		if _, err := cmd.settings(); err == nil || !strings.Contains(err.Error(), "GANG_PEERS") {
			t.Errorf("%s: error = %v", name, err)
		}
	}
}

// A host named like a Gangline-derived ID prefix, sending from an agent named
// like a Gangline sender, must not make its message read as Gangline input.
func TestPeerHostCannotNameADerivedID(t *testing.T) {
	f, _ := peerFixture(t)
	f.env["GANG_PEERS"] = `{"resume-x":{"ssh":"x","gang":"/opt/gang","team":"unit"}}`
	reply, err := peerReceive(f, peerRequestJSON(t, func(r map[string]any) {
		r["host"], r["from"] = "resume-x", "compact"
	}))
	if err != nil || reply.ID != "peer-resume-x."+peerTestID || reply.Outcome != "delivered" {
		t.Fatalf("reply = %+v, err = %v", reply, err)
	}
}

func TestPeerReceiveKeepsSendRecipientChecks(t *testing.T) {
	t.Run("inactive", func(t *testing.T) {
		f, a := peerFixture(t)
		p, _ := f.run.team.Agent(a.ID)
		l, err := p.TryLock()
		if err != nil {
			t.Fatal(err)
		}
		a.Status = core.Dropping
		if err := l.Save(a); err != nil {
			t.Fatal(err)
		}
		if err := l.Close(); err != nil {
			t.Fatal(err)
		}
		reply, err := peerReceive(f, peerRequestJSON(t, nil))
		if err == nil || reply.ID != "" || reply.Error == "" || f.input.submits != 0 {
			t.Fatalf("inactive recipient: reply=%+v err=%v", reply, err)
		}
	})
	t.Run("paste hazard", func(t *testing.T) {
		f, a, p := claudeRecipient(t)
		f.env["GANG_PEERS"] = `{"alpha":{"ssh":"alpha","gang":"/opt/gang","team":"unit"}}`
		f.env["GANGLINE_HITCH_ID"] = string(a.ID)
		reply, err := peerReceive(f, peerRequestJSON(t, func(r map[string]any) { r["body"] = placeholderQuote }))
		requirePlaceholderRefusal(t, err)
		if reply.ID != "" || reply.Error == "" {
			t.Fatalf("reply = %+v", reply)
		}
		requireNothingTyped(t, f, p)
	})
}
