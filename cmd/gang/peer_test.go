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
	"github.com/adambiggs/gangline/substrate/tmux"
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

// fakeProcesses is a process table that answers each read from a script.
type fakeProcesses struct {
	uid uint32
	// parents are successive getppid results; the last repeats.
	parents []int
	// table holds successive reads of each PID; the last repeats. A read
	// with PID 0 finds no process.
	table map[int][]tmux.Owner
	reads map[int]int
	calls int
}

func (p *fakeProcesses) parentPID() int {
	i := min(p.calls, len(p.parents)-1)
	p.calls++
	return p.parents[i]
}

func (p *fakeProcesses) userID() uint32 { return p.uid }

func (p *fakeProcesses) owner(pid int) (tmux.Owner, error) {
	reads := p.table[pid]
	if len(reads) == 0 {
		return tmux.Owner{}, os.ErrNotExist
	}
	i := min(p.reads[pid], len(reads)-1)
	p.reads[pid]++
	if reads[i].PID == 0 {
		return tmux.Owner{}, os.ErrNotExist
	}
	return reads[i], nil
}

// sshdChain is the process table of a command sshd runs for user 1000: PID
// 50 is the user's sshd process, under root's sshd at PID 40.
func sshdChain(session string) *fakeProcesses {
	return &fakeProcesses{uid: 1000, parents: []int{50}, reads: map[int]int{}, table: map[int][]tmux.Owner{
		50: {{PID: 50, ParentPID: 40, UID: 1000, RealUID: 1000, Command: session}},
		40: {{PID: 40, ParentPID: 1, UID: 0, RealUID: 0, Command: session}},
	}}
}

func peerFixture(t *testing.T) (*stateFixture, core.Agent) {
	t.Helper()
	f := newStateFixture(t)
	f.env["GANG_PEERS"] = `{"alpha":{"ssh":"alpha","user":"adam","gang":"/opt/gang","team":"unit"}}`
	f.cmd.processes = sshdChain("sshd")
	a := f.add(t, "worker-id", "worker", "codex")
	f.env["GANGLINE_HITCH_ID"] = string(a.ID)
	return f, a
}

func peerReceive(f *stateFixture, request string) (peerReply, error) {
	return peerReceiveAs(f, "alpha", request)
}

// peerReceiveAs supplies the host argument under an admitted ancestry fixture;
// an empty host leaves --peer out.
func peerReceiveAs(f *stateFixture, host, request string) (peerReply, error) {
	f.out.Reset()
	cmd := f.cmd
	cmd.stdin = strings.NewReader(request)
	args := []string{"peer-receive"}
	if host != "" {
		args = append(args, "--peer", host)
	}
	err := cmd.execute(args)
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
		setup   func(f *stateFixture, peer *string)
		want    string
	}{
		{"unenrolled host", func(t *testing.T) string { return peerRequestJSON(t, set("host", "beta")) }, func(f *stateFixture, peer *string) { *peer = "beta" }, "not enrolled"},
		{"no peers configured", func(t *testing.T) string { return peerRequestJSON(t, nil) }, func(f *stateFixture, peer *string) { delete(f.env, "GANG_PEERS") }, "not enrolled"},
		{"no --peer", func(t *testing.T) string { return peerRequestJSON(t, nil) }, func(f *stateFixture, peer *string) { *peer = "" }, "gang peer-authorize HOST PUBKEY_FILE"},
		{"request names another host", func(t *testing.T) string { return peerRequestJSON(t, set("host", "beta")) }, nil, `request names host "beta", but this key is bound to "alpha"`},
		{"request names another enrolled host", func(t *testing.T) string { return peerRequestJSON(t, set("host", "beta")) }, func(f *stateFixture, peer *string) {
			f.env["GANG_PEERS"] = `{"alpha":{"ssh":"alpha","gang":"/opt/gang","team":"unit"},"beta":{"ssh":"beta","gang":"/opt/gang","team":"unit"}}`
		}, "this key is bound to"},
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
		{"inside agent pane", func(t *testing.T) string { return peerRequestJSON(t, nil) }, func(f *stateFixture, peer *string) {
			f.add(t, "lead-id", "lead", "codex")
			f.env["GANG_AGENT_ID"] = "lead-id"
		}, "registered agent lead"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, a := peerFixture(t)
			peer := "alpha"
			if tc.setup != nil {
				tc.setup(f, &peer)
			}
			reply, err := peerReceiveAs(f, peer, tc.request(t))
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
		"repeated host":      `{"alpha":{"ssh":"alpha","gang":"/g","team":"t"},"alpha":{"ssh":"other","gang":"/g","team":"t"}}`,
		"case variant host":  `{"lead":{"ssh":"lead","gang":"/g","team":"t"},"Lead":{"ssh":"lead","gang":"/g","team":"t"}}`,
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
	reply, err := peerReceiveAs(f, "resume-x", peerRequestJSON(t, func(r map[string]any) {
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
		f.cmd.processes = sshdChain("sshd")
		f.env["GANGLINE_HITCH_ID"] = string(a.ID)
		reply, err := peerReceive(f, peerRequestJSON(t, func(r map[string]any) { r["body"] = placeholderQuote }))
		requirePlaceholderRefusal(t, err)
		if reply.ID != "" || reply.Error == "" {
			t.Fatalf("reply = %+v", reply)
		}
		requireNothingTyped(t, f, p)
	})
}

// Two requests with one ID can both pass the duplicate check before either
// publishes. The later one is refused at publish and the queued message keeps
// its body.
func TestPeerReceiveRefusesARacingDuplicate(t *testing.T) {
	f, a := peerFixture(t)
	p, err := f.run.team.Agent(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	id := core.EnvelopeID("peer-alpha." + peerTestID)
	rival := core.Envelope{ID: id, Token: "0123456789abcdef", Recipient: a.ID, To: a.Name, From: core.Sender{Kind: core.SenderPeer, Name: "lead", Host: "alpha"}, Message: core.Message{Text: "the first request's body"}, CreatedAt: f.cmd.now()}
	// The harness check runs between the duplicate check and publishing.
	f.input.onForeground = func() {
		f.input.onForeground = nil
		if err := p.Publish(rival); err != nil {
			t.Fatal(err)
		}
	}
	reply, err := peerReceive(f, peerRequestJSON(t, nil))
	var refused commandError
	if !errors.As(err, &refused) || refused.status != exitRefused || !strings.Contains(reply.Error, "already published here") || reply.ID != "" {
		t.Fatalf("racing duplicate: reply=%+v err=%v", reply, err)
	}
	queued, err := p.ReadEnvelope("new", id)
	if err != nil || queued.Message.Text != rival.Message.Text {
		t.Fatalf("queued message = %+v, %v; want the first request's body", queued.Message, err)
	}
	if f.input.pasted != "" {
		t.Fatalf("pasted = %q", f.input.pasted)
	}
}

func TestPeerTransportAdmitsSSHDChildren(t *testing.T) {
	for _, session := range []string{"sshd", "sshd-session"} {
		if err := checkPeerTransport(sshdChain(session)); err != nil {
			t.Errorf("%s: %v", session, err)
		}
	}
}

// The transport screen requires the expected sshd ancestry. Each
// refused table differs from an admitted one in one read.
func TestPeerReceiveRefusesAnyOtherTransport(t *testing.T) {
	edit := func(pid int, change func(*tmux.Owner)) func(*fakeProcesses) {
		return func(p *fakeProcesses) { change(&p.table[pid][0]) }
	}
	for _, tc := range []struct {
		name   string
		change func(*fakeProcesses)
		want   string
	}{
		{"parent not sshd", edit(50, func(o *tmux.Owner) { o.Command = "bash" }), `parent process 50 is "bash"`},
		{"parent named like sshd", edit(50, func(o *tmux.Owner) { o.Command = "sshd " }), `parent process 50 is "sshd "`},
		{"parent of another real user", edit(50, func(o *tmux.Owner) { o.RealUID = 1001 }), "1001 real"},
		{"parent of another effective user", edit(50, func(o *tmux.Owner) { o.UID = 0 }), "0 effective"},
		{"parent unreadable", func(p *fakeProcesses) { delete(p.table, 50) }, "read parent process 50"},
		{"grandparent not sshd", edit(40, func(o *tmux.Owner) { o.Command = "systemd" }), `process 40 above sshd is "systemd"`},
		{"grandparent real user not root", edit(40, func(o *tmux.Owner) { o.RealUID = 1000 }), "1000 real"},
		{"grandparent effective user not root", edit(40, func(o *tmux.Owner) { o.UID = 1000 }), "1000 effective"},
		{"grandparent unreadable", func(p *fakeProcesses) { delete(p.table, 40) }, "read sshd process 40"},
		{"parent moved while read", func(p *fakeProcesses) {
			p.table[50] = append(p.table[50], tmux.Owner{PID: 50, ParentPID: 1, UID: 1000, RealUID: 1000, Command: "sshd"})
		}, "moved from parent 40 to 1"},
		{"parent gone on reread", func(p *fakeProcesses) { p.table[50] = append(p.table[50], tmux.Owner{}) }, "read parent process 50 again"},
		{"own parent changed", func(p *fakeProcesses) { p.parents = []int{50, 1} }, "changed from 50 to 1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, a := peerFixture(t)
			processes := sshdChain("sshd")
			tc.change(processes)
			f.cmd.processes = processes
			stdin := strings.NewReader(peerRequestJSON(t, nil))
			f.cmd.stdin = stdin
			err := f.cmd.execute([]string{"peer-receive", "--peer", "alpha"})
			var refusal commandError
			if !errors.As(err, &refusal) || refusal.status != exitRefused || !strings.Contains(err.Error(), peerTransportRefusal) || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want the transport refusal with %q", err, tc.want)
			}
			if stdin.Len() != int(stdin.Size()) {
				t.Fatalf("refused request read %d bytes of stdin", int(stdin.Size())-stdin.Len())
			}
			p, err := f.run.team.Agent(a.ID)
			if err != nil {
				t.Fatal(err)
			}
			if pending, err := p.ListNew(); err != nil || len(pending) != 0 || f.input.submits != 0 {
				t.Fatalf("refused request published: %+v submits=%d err=%v", pending, f.input.submits, err)
			}
		})
	}
}

// The sender's label and envelope ID use the host supplied by --peer.
func TestPeerReceiveLabelsTheBoundHost(t *testing.T) {
	f, a := peerFixture(t)
	f.env["GANG_PEERS"] = `{"alpha":{"ssh":"alpha","gang":"/opt/gang","team":"unit"},"beta":{"ssh":"beta","gang":"/opt/gang","team":"unit"}}`
	reply, err := peerReceiveAs(f, "beta", peerRequestJSON(t, func(r map[string]any) { r["host"] = "beta" }))
	if err != nil || reply.ID != "peer-beta."+peerTestID {
		t.Fatalf("reply = %+v, err = %v", reply, err)
	}
	p, err := f.run.team.Agent(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	e, err := p.ReadEnvelope("cur", reply.ID)
	if err != nil || e.From != (core.Sender{Kind: core.SenderPeer, Name: "lead", Host: "beta"}) {
		t.Fatalf("sender = %+v, err = %v", e.From, err)
	}
}

func TestPeerReceiveSenderClasses(t *testing.T) {
	for _, class := range []string{"", "agent", "registered", "self_declared", "outside", "gangline", "peer", "invented"} {
		t.Run("class="+class, func(t *testing.T) {
			f, a := peerFixture(t)
			reply, err := peerReceive(f, peerRequestJSON(t, func(r map[string]any) { r["class"] = class; r["from"] = "relay" }))
			allowed := class == "" || class == "agent" || class == "registered"
			p, pathErr := f.run.team.Agent(a.ID)
			if pathErr != nil {
				t.Fatal(pathErr)
			}
			if !allowed {
				requireStatus(t, err, exitRefused, "sender class")
				pending, err := p.ListNew()
				if err != nil || len(pending) != 0 || f.input.submits != 0 || reply.ID != "" {
					t.Fatal("unsupported class published")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			e, err := p.ReadEnvelope("cur", reply.ID)
			if err != nil {
				t.Fatalf("reply=%+v pasted=%q stderr=%s: %v", reply, f.input.pasted, f.errOut.String(), err)
			}
			label := "alpha:relay"
			if class == "registered" {
				label = "alpha:registered:relay"
			}
			if senderLabel(e.From) != label || !strings.Contains(f.input.pasted, "[gang:"+label+"#") {
				t.Fatalf("sender %+v pasted %s", e.From, f.input.pasted)
			}
			// A stored envelope must retain the distinction after JSON round-trip.
			data, err := json.Marshal(e.From)
			if err != nil {
				t.Fatal(err)
			}
			var restored core.Sender
			if err := json.Unmarshal(data, &restored); err != nil {
				t.Fatal(err)
			}
			if senderLabel(restored) != label {
				t.Fatalf("class lost in storage: %s", data)
			}
			plain := core.Sender{Kind: core.SenderPeer, Name: "relay", Host: "alpha"}
			if restored.SameIdentity(plain) == (class == "registered") {
				t.Fatal("peer sender classes share identity")
			}
		})
	}
}
