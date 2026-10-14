package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"testing"
	"time"
)

// fakeSSH puts an ssh on PATH that records its arguments and stdin and plays
// back a reply the test wrote, with @ID@ standing for the forwarded message's
// ID as the peer files it. It calls only /bin/cat and shell builtins by
// absolute path, never ssh or gang, and refuses to run inside itself.
type fakeSSH struct{ dir string }

func newFakeSSH(t *testing.T) fakeSSH {
	t.Helper()
	dir := t.TempDir()
	script := `#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
[ -z "$GANG_FAKE_SSH_DEPTH" ] || exit 97
GANG_FAKE_SSH_DEPTH=1
export GANG_FAKE_SSH_DEPTH
d='` + dir + `'
for word in "$@"; do printf '%s\n' "$word"; done > "$d/argv"
/bin/cat > "$d/stdin"
if [ -p "$d/started" ]; then
	printf x > "$d/started"
	read never < "$d/never"
fi
[ ! -f "$d/stderr" ] || /bin/cat "$d/stderr" >&2
request=$(/bin/cat "$d/stdin")
id=${request#*'"id":"'}
id=peer-ser5.${id%%'"'*}
out=$(/bin/cat "$d/stdout"; printf x)
out=${out%x}
while :; do
	case $out in
	*@ID@*) out=${out%%@ID@*}$id${out#*@ID@} ;;
	*) break ;;
	esac
done
printf '%s' "$out"
exit "$(/bin/cat "$d/status")"
`
	if err := os.WriteFile(filepath.Join(dir, "ssh"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return fakeSSH{dir: dir}
}

func (s fakeSSH) reply(t *testing.T, stdout, stderr string, status string) {
	t.Helper()
	for name, content := range map[string]string{"stdout": stdout, "stderr": stderr, "status": status} {
		if err := os.WriteFile(filepath.Join(s.dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func (s fakeSSH) ran() bool {
	_, err := os.Stat(filepath.Join(s.dir, "argv"))
	return err == nil
}

func (s fakeSSH) request(t *testing.T) peerRequest {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(s.dir, "stdin"))
	if err != nil {
		t.Fatal(err)
	}
	request, err := readPeerRequest(strings.NewReader(string(data)))
	if err != nil {
		t.Fatalf("forwarded request %q: %v", data, err)
	}
	return request
}

// peerSender is a verified agent lead on host ser5 with peer tombo enrolled.
func peerSender(t *testing.T) (*stateFixture, fakeSSH) {
	t.Helper()
	f := newStateFixture(t)
	f.env["GANG_HOST"] = "ser5"
	f.env["GANG_PEERS"] = `{"tombo":{"ssh":"tombo-alias","user":"tombo","gang":"/home/tombo/.local/bin/gang","team":"gangline"}}`
	f.add(t, "lead-id", "lead", "codex")
	f.env["GANG_AGENT_ID"] = "lead-id"
	return f, newFakeSSH(t)
}

func sendPeer(f *stateFixture, args ...string) error {
	f.out.Reset()
	f.errOut.Reset()
	return f.cmd.execute(append([]string{"send"}, args...))
}

func requireStatus(t *testing.T, err error, status int, text string) {
	t.Helper()
	var c commandError
	if !errors.As(err, &c) || c.status != status || !strings.Contains(err.Error(), text) {
		t.Fatalf("error = %v, want status %d containing %q", err, status, text)
	}
}

// forwardedID is the ID the peer files the forwarded message under.
func (s fakeSSH) forwardedID(t *testing.T) string {
	t.Helper()
	return "peer-ser5." + s.request(t).ID
}

func TestPeerSendForwardsOverSSHAndRelaysTheOutcome(t *testing.T) {
	f, ssh := peerSender(t)
	ssh.reply(t, "Last login: rc noise\n"+`{"id":"@ID@","outcome":"delivered"}`+"\n", "", "0")
	if err := sendPeer(f, "tombo:reviewer", "status: it's $(done); `really`"); err != nil {
		t.Fatal(err)
	}
	if got := f.out.String(); got != "tombo:"+ssh.forwardedID(t)+"\tdelivered\n" {
		t.Fatalf("stdout = %q", got)
	}
	argv, err := os.ReadFile(filepath.Join(ssh.dir, "argv"))
	if err != nil {
		t.Fatal(err)
	}
	want := "-T\n-o\nBatchMode=yes\n-o\nConnectTimeout=10\n-o\nServerAliveInterval=10\n-o\nServerAliveCountMax=3\n-l\ntombo\n--\ntombo-alias\n/home/tombo/.local/bin/gang\npeer-receive\n--team\ngangline\n"
	if string(argv) != want {
		t.Fatalf("ssh argv =\n%s\nwant\n%s", argv, want)
	}
	request := ssh.request(t)
	if request.Host != "ser5" || request.Team != "gangline" || request.From != "lead" || request.To != "reviewer" || request.Body != "status: it's $(done); `really`" || !peerMessageIDPattern.MatchString(request.ID) {
		t.Fatalf("request = %+v", request)
	}
}

func TestPeerSendRelaysEveryPeerOutcome(t *testing.T) {
	for _, tc := range []struct {
		name, stdout, status string
		exit                 int
		line, stderr         string
	}{
		{"queued", `{"id":"@ID@","outcome":"queued"}`, "0", exitOK, "queued", ""},
		{"accepted", `{"id":"@ID@","outcome":"accepted"}`, "0", exitOK, "accepted", "native queue accepted the message; do not resend"},
		{"failed", `{"id":"@ID@","outcome":"failed","warning":"message x was retained\u001b[2J with failed status"}`, "0", exitOK, "failed", "warning: tombo: message x was retained?[2J with failed status\n"},
		{"unverified", `{"id":"@ID@","outcome":"unverified"}`, "5", exitUnknown, "unverified", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, ssh := peerSender(t)
			ssh.reply(t, tc.stdout+"\n", "", tc.status)
			err := sendPeer(f, "tombo:lead", "hi")
			if tc.exit == exitOK && err != nil || tc.exit != exitOK && errorStatus(err) != tc.exit {
				t.Fatalf("error = %v, want status %d", err, tc.exit)
			}
			if got := f.out.String(); got != "tombo:"+ssh.forwardedID(t)+"\t"+tc.line+"\n" {
				t.Fatalf("stdout = %q", got)
			}
			if !strings.Contains(f.errOut.String(), tc.stderr) {
				t.Fatalf("stderr = %q, want %q", f.errOut.String(), tc.stderr)
			}
		})
	}
}

func TestPeerSendRelaysAPeerRefusal(t *testing.T) {
	for _, tc := range []struct {
		status string
		exit   int
	}{{"3", exitRefused}, {"2", exitUsage}, {"1", exitError}, {"4", exitNative}} {
		t.Run(tc.status, func(t *testing.T) {
			f, ssh := peerSender(t)
			ssh.reply(t, `{"error":"agent \"lead\" is not registered\u001b[2J in team gangline"}`+"\n", "", tc.status)
			requireStatus(t, sendPeer(f, "tombo:lead", "hi"), tc.exit, `tombo: agent "lead" is not registered?[2J in team`)
			if f.out.String() != "" {
				t.Fatalf("stdout = %q", f.out.String())
			}
		})
	}
}

// Without a reply nothing here knows whether the peer published the message,
// so the outcome is unknown and names the ID to look up there.
func TestPeerSendReportsALostReplyAsUnknown(t *testing.T) {
	for _, tc := range []struct{ name, stdout, stderr, status, cause string }{
		{"connection lost", "", "Connection to tombo closed by remote host.\n", "255", "ssh: exit status 255: Connection to tombo closed by remote host."},
		{"gang missing", "", "sh: 1: /home/tombo/.local/bin/gang: not found\n", "127", "exit status 127"},
		{"garbled reply", "Welcome\n{\"id\":\n", "", "0", "its last line was not a reply to this message"},
		{"empty reply", "", "", "0", "its last line was not a reply to this message"},
		{"reply without outcome", `{"id":"@ID@"}` + "\n", "", "0", "its last line was not a reply to this message"},
		{"another message's reply", `{"id":"peer-ser5.msg-0123456789abcdef0123456789abcdef","outcome":"delivered"}` + "\n", "", "0", "its last line was not a reply to this message"},
		{"unknown outcome", `{"id":"@ID@","outcome":"read"}` + "\n", "", "0", "its last line was not a reply to this message"},
		{"controls in stderr", "", "bad\x1b[2J\n", "255", "bad?[2J"},
		{"error-shaped noise, then connection lost", `{"error":"login rc warning"}` + "\n", "", "255", "ssh: exit status 255"},
		{"error-shaped noise, then peer unknown", `{"error":"login rc warning"}` + "\n", "", "5", "ssh: exit status 5"},
		{"error line with success status", `{"error":"login rc warning"}` + "\n", "", "0", "its last line was not a reply to this message"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, ssh := peerSender(t)
			ssh.reply(t, tc.stdout, tc.stderr, tc.status)
			err := sendPeer(f, "tombo:lead", "hi")
			requireStatus(t, err, exitUnknown, "resend it only once no gang peer-receive runs on tombo and its gang queue and gang log both lack it")
			if !strings.Contains(err.Error(), tc.cause) {
				t.Fatalf("error = %v, want cause %q", err, tc.cause)
			}
			id := "peer-ser5." + ssh.request(t).ID
			if got := f.out.String(); got != "tombo:"+id+"\tunknown\n" || !strings.Contains(err.Error(), "message "+id+" may have been published") {
				t.Fatalf("stdout = %q, error = %v", got, err)
			}
		})
	}
}

// A peer that never answers is cut off at the deadline, and its outcome is
// unknown. The test expires the deadline once the fake ssh holds the request.
func TestPeerSendStopsWaitingAtTheDeadline(t *testing.T) {
	f, ssh := peerSender(t)
	ssh.reply(t, "", "", "0")
	for _, fifo := range []string{"started", "never"} {
		if err := syscall.Mkfifo(filepath.Join(ssh.dir, fifo), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var asked time.Duration
	f.cmd.newTimeout = func(parent context.Context, d time.Duration) (context.Context, context.CancelFunc) {
		asked = d
		ctx, cancel := context.WithCancel(parent)
		go func() {
			if _, err := os.ReadFile(filepath.Join(ssh.dir, "started")); err == nil {
				cancel()
			}
		}()
		return ctx, cancel
	}
	err := sendPeer(f, "tombo:lead", "hi")
	requireStatus(t, err, exitUnknown, "no reply within "+peerSendTimeout.String())
	if asked != peerSendTimeout || !regexp.MustCompile(`^tombo:peer-ser5\.msg-[0-9a-f]{32}\tunknown\n$`).MatchString(f.out.String()) {
		t.Fatalf("deadline = %s, stdout = %q", asked, f.out.String())
	}
}

func TestPeerSendRefusesBeforeContactingThePeer(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(*stateFixture)
		args  []string
		exit  int
		want  string
	}{
		{"unenrolled host", nil, []string{"mac:lead", "hi"}, exitRefused, `peer host "mac" is not enrolled`},
		{"no peers", func(f *stateFixture) { delete(f.env, "GANG_PEERS") }, []string{"tombo:lead", "hi"}, exitRefused, "not enrolled"},
		{"no host name", func(f *stateFixture) { delete(f.env, "GANG_HOST") }, []string{"tombo:lead", "hi"}, exitRefused, "GANG_HOST is unset"},
		{"operator", func(f *stateFixture) { delete(f.env, "GANG_AGENT_ID") }, []string{"tombo:lead", "hi"}, exitRefused, "only a registered agent sends to a peer host"},
		{"forged pane token", func(f *stateFixture) { f.env["GANG_AGENT_NONCE"] = "copied" }, []string{"tombo:lead", "hi"}, exitRefused, "hitch token does not match"},
		{"empty body", nil, []string{"tombo:lead", ""}, exitRefused, "body is empty"},
		{"control body", nil, []string{"tombo:lead", "a\x1b[2Jb"}, exitRefused, "control"},
		{"--from", nil, []string{"tombo:lead", "--from", "x", "hi"}, exitUsage, "HOST:NAME recipient takes no"},
		{"--at", nil, []string{"tombo:lead", "--at", "5m", "hi"}, exitUsage, "HOST:NAME recipient takes no"},
		{"--live-only", nil, []string{"tombo:lead", "--live-only", "hi"}, exitUsage, "HOST:NAME recipient takes no"},
		{"--supersede", nil, []string{"tombo:lead", "--supersede", "hi"}, exitUsage, "HOST:NAME recipient takes no"},
		{"--clear", nil, []string{"tombo:lead", "--clear"}, exitUsage, "HOST:NAME recipient takes no"},
		{"bad host", nil, []string{"to;mbo:lead", "hi"}, exitUsage, "invalid peer host"},
		{"self-declared host", nil, []string{"self-declared:lead", "hi"}, exitUsage, "invalid peer host"},
		{"empty host", nil, []string{":lead", "hi"}, exitUsage, "invalid peer host"},
		{"bad name", nil, []string{"tombo:le ad", "hi"}, exitUsage, "invalid agent name"},
		{"nested label", nil, []string{"tombo:ser5:lead", "hi"}, exitUsage, "invalid agent name"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, ssh := peerSender(t)
			ssh.reply(t, `{"id":"@ID@","outcome":"delivered"}`+"\n", "", "0")
			if tc.setup != nil {
				tc.setup(f)
			}
			requireStatus(t, sendPeer(f, tc.args...), tc.exit, tc.want)
			if ssh.ran() || f.out.String() != "" {
				t.Fatalf("refused send reached ssh=%v stdout=%q", ssh.ran(), f.out.String())
			}
		})
	}
}

func TestPeerSendReadsTheBodyFromStdin(t *testing.T) {
	f, ssh := peerSender(t)
	ssh.reply(t, `{"id":"@ID@","outcome":"delivered"}`+"\n", "", "0")
	f.cmd.stdin = strings.NewReader("line one\nline two\n")
	if err := sendPeer(f, "tombo:lead"); err != nil {
		t.Fatal(err)
	}
	if body := ssh.request(t).Body; body != "line one\nline two" {
		t.Fatalf("body = %q", body)
	}
}

func TestHostConfiguration(t *testing.T) {
	directory := t.TempDir()
	cmd := command{
		getenv:      func(k string) string { return map[string]string{"GANG_CONFIG_DIR": directory}[k] },
		userHomeDir: func() (string, error) { return directory, nil },
	}
	for value, ok := range map[string]bool{"ser5": true, "mac-mini.local": true, "self-declared": false, "Self-Declared": false, "a:b": false, "gangline": false, "-x": false} {
		if err := os.WriteFile(filepath.Join(directory, "config"), []byte("GANG_HOST="+value+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		got, err := cmd.settings()
		if ok && (err != nil || got.Host != value || got.Origins["GANG_HOST"] != "config") || !ok && (err == nil || !strings.Contains(err.Error(), "GANG_HOST")) {
			t.Errorf("GANG_HOST=%s: host=%q err=%v", value, got.Host, err)
		}
	}
}

func TestLastPeerReplyTakesTheFinalLine(t *testing.T) {
	const id = "peer-ser5.msg-0123456789abcdef0123456789abcdef"
	published, _ := json.Marshal(peerReply{ID: id, Outcome: "delivered"})
	for _, tc := range []struct {
		output string
		ok     bool
	}{
		{string(published) + "\n", true},
		{"motd\r\n" + string(published) + "\r\n\n", true},
		{string(published) + "\n  \n\t\n", true},
		{string(published) + "\ntrailing noise\n", false},
		{`{"error":"refused"}`, true},
		{`{"id":"x","error":"refused"}`, false},
		{`{"outcome":"delivered"}`, false},
		{`{}`, false},
		{"", false},
	} {
		if _, ok := lastPeerReply([]byte(tc.output), id); ok != tc.ok {
			t.Errorf("lastPeerReply(%q) = %v, want %v", tc.output, ok, tc.ok)
		}
	}
	b := &tailBuffer{limit: 4}
	b.Write([]byte("abc"))
	b.Write([]byte("defg"))
	if string(b.data) != "defg" {
		t.Fatalf("tail = %q", b.data)
	}
}
