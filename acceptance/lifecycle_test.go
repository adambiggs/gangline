package acceptance

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/store"
)

func TestCommandLifecycleOnPrivateTmux(t *testing.T) {
	root, err := os.MkdirTemp(os.TempDir(), "team-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	repo, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(root, "gang")
	build := exec.Command("go", "build", "-o", binary, "./cmd/gang")
	build.Dir = repo
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	collars := filepath.Join(root, "collars")
	if err := os.Mkdir(collars, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(collars, "acceptance.cue"), []byte(commandAcceptanceCollar(exe)), 0600); err != nil {
		t.Fatal(err)
	}
	const session = "gangline-rebuild-acceptance"
	socket := filepath.Join(root, "tmux.sock")
	hitchReady := filepath.Join(root, "hitch-ready")
	if output, err := exec.Command("mkfifo", hitchReady).CombinedOutput(); err != nil {
		t.Fatalf("create hitch ready pipe: %v\n%s", err, output)
	}
	environment := append(withoutEnvironment(os.Environ(), "TMUX", "TMUX_PANE", "GANG_CONFIG_DIR", "GANG_SESSION", "GANG_STATE_ROOT", "GANG_TMUX_SOCKET", "GANG_COLLARS", "GANG_COLLAR", "GANGLINE_HITCH_ID", "GANG_AGENT_ID", "GANG_AGENT_NONCE", "GANG_AGENT_TOKEN"),
		"GANG_SESSION="+session, "GANG_CONFIG_DIR="+filepath.Join(root, "config"), "GANG_STATE_ROOT="+filepath.Join(root, "state"), "GANG_TMUX_SOCKET="+socket, "GANG_COLLARS="+collars, "GANG_COLLAR=acceptance",
		"GANGLINE_ACCEPTANCE_CMD_HARNESS=1", "GANGLINE_ACCEPTANCE_TMUX=tmux", "GANGLINE_ACCEPTANCE_TMUX_SOCKET="+socket, "GANGLINE_ACCEPTANCE_LEDGER="+filepath.Join(root, "received"), "GANGLINE_ACCEPTANCE_ARGV_LEDGER="+filepath.Join(root, "argv"), "GANGLINE_ACCEPTANCE_GANG="+binary, "GANGLINE_ACCEPTANCE_HITCH_RESULT="+filepath.Join(root, "hitch-result"), "GANGLINE_ACCEPTANCE_HITCH_READY="+hitchReady)
	runner := tmuxRunner{binary: "tmux", socket: socket, env: environment}
	if out, err := runner.run("new-session", "-d", "-s", session, "-n", "control"); err != nil {
		t.Fatalf("private session: %v %s", err, out)
	}
	t.Cleanup(func() { _, _ = runner.run("kill-session", "-t", session) })
	if out, err := runner.run("list-sessions", "-F", "#{session_name}"); err != nil || strings.TrimSpace(out) != session {
		t.Fatalf("private server contains unexpected sessions: %q %v", out, err)
	}
	type result struct {
		out    string
		status int
	}
	runGang := func(input string, args ...string) result {
		cmd := exec.Command(binary, args...)
		cmd.Dir = repo
		cmd.Env = environment
		cmd.Stdin = strings.NewReader(input)
		out, err := cmd.CombinedOutput()
		if err == nil {
			return result{string(out), 0}
		}
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return result{string(out), exit.ExitCode()}
		}
		return result{fmt.Sprintf("%v: %s", err, out), -1}
	}
	t.Cleanup(func() {
		r := runGang("", "down", "--yes")
		if r.status != 0 {
			t.Errorf("private team cleanup: %s", r.out)
		}
	})
	check := func(input string, args ...string) string {
		t.Helper()
		r := runGang(input, args...)
		if r.status != 0 {
			t.Fatalf("gang %v status=%d\n%s", args, r.status, r.out)
		}
		return r.out
	}
	start := make(chan struct{})
	results := make(chan result, 2)
	for i := 0; i < 2; i++ {
		go func() { <-start; results <- runGang("", "hitch", "worker", "-t", "acceptance assignment") }()
	}
	close(start)
	success, refused := 0, 0
	for i := 0; i < 2; i++ {
		r := <-results
		switch r.status {
		case 0:
			success++
		case 3:
			refused++
		default:
			log, _ := os.ReadFile(filepath.Join(root, "state", "teams", session, "log.jsonl"))
			t.Fatalf("concurrent hitch status %d: %s\naudit:\n%s", r.status, r.out, log)
		}
	}
	if success != 1 || refused != 1 {
		t.Fatalf("hitch results: success=%d refused=%d", success, refused)
	}
	team, err := (store.Paths{Root: filepath.Join(root, "state")}).Team(session)
	if err != nil {
		t.Fatal(err)
	}
	agents, err := team.ListAgents()
	if err != nil || len(agents) != 1 {
		t.Fatalf("claimed agents: %+v %v", agents, err)
	}
	worker := agents[0]
	argv, err := os.ReadFile(filepath.Join(root, "argv"))
	if err != nil {
		t.Fatal(err)
	}
	received, err := os.ReadFile(filepath.Join(root, "received"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(argv), "# Gangline delivery contract") != 1 || strings.Contains(string(received), "# Gangline delivery contract") || strings.Contains(string(argv), "acceptance assignment") {
		t.Fatalf("standing prose or task duplicated across launch and startup: argv=%q received=%q", argv, received)
	}
	if !regexp.MustCompile(`^\[gang:gangline:hitch#[0-9a-f]{16} assignment\]`).Match(received) || !strings.Contains(string(received), "Assignment:\n\nacceptance assignment") {
		t.Fatalf("startup lacks Gangline attribution or assignment: %q", received)
	}
	if output, err := runner.run("send-keys", "-t", worker.Pane, "-l", "__GANG_HITCH_SECOND__"); err != nil {
		t.Fatalf("enter worker command: %v\n%s", err, output)
	}
	if output, err := runner.run("send-keys", "-t", worker.Pane, "Enter"); err != nil {
		t.Fatalf("submit worker command: %v\n%s", err, output)
	}
	if got, err := os.ReadFile(hitchReady); err != nil || string(got) != "x" {
		t.Fatalf("worker ready pipe=%q: %v", got, err)
	}
	hitchResult, err := os.ReadFile(filepath.Join(root, "hitch-result"))
	if err != nil || !strings.HasPrefix(string(hitchResult), "status=0\n") {
		t.Fatalf("worker hitch result=%q: %v", hitchResult, err)
	}
	sid, err := team.ResolveName("second")
	if err != nil {
		t.Fatal(err)
	}
	sp, err := team.Agent(sid)
	if err != nil {
		t.Fatal(err)
	}
	second, err := sp.Read()
	if err != nil {
		t.Fatal(err)
	}
	received, err = os.ReadFile(filepath.Join(root, "received"))
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`\[gang:gangline:startup#[0-9a-f]{16} startup\] No assignment was supplied\.`).Match(received) {
		t.Fatalf("taskless startup lacks Gangline attribution or invents an assignment: %q", received)
	}
	wp, err := team.Agent(worker.ID)
	if err != nil {
		t.Fatal(err)
	}
	held, err := wp.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	delivered := check("independent delivery", "send", "second", "--from", "operator")
	if err := held.Close(); err != nil {
		t.Fatal(err)
	}
	finishDelivery := func(result, body string) {
		t.Helper()
		outcome := strings.TrimSpace(result)
		if !strings.HasSuffix(outcome, "\tdelivered") && !strings.HasSuffix(outcome, "\tqueued") {
			t.Fatalf("other lock blocked delivery: %s", result)
		}
		barrier, err := sp.LockAgent()
		if err != nil {
			t.Fatal(err)
		}
		if err := barrier.Close(); err != nil {
			t.Fatal(err)
		}
		check("", "tick")
		received, err := os.ReadFile(filepath.Join(root, "received"))
		if err != nil || !strings.Contains(string(received), body) {
			t.Fatalf("native delivery ledger lacks %q: %q %v", body, received, err)
		}
	}
	finishDelivery(delivered, "independent delivery")
	if err := os.Chmod(team.Log, 0200); err != nil {
		t.Fatal(err)
	}
	if _, err := os.ReadFile(team.Log); err == nil {
		t.Fatal("audit log must be unreadable for this test")
	}
	check("", "roster", "--porcelain")
	check("", "status", "second")
	finishDelivery(check("audit independent delivery", "send", "second", "--from", "operator"), "audit independent delivery")
	check("", "tick")
	expired, err := wp.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	worker, err = wp.Read()
	if err != nil {
		t.Fatal(err)
	}
	worker.DropDeadline = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	worker.BootDeadline = worker.DropDeadline
	worker.InterruptDeadline = worker.DropDeadline
	if err := expired.Save(worker); err != nil {
		t.Fatal(err)
	}
	if err := expired.Close(); err != nil {
		t.Fatal(err)
	}
	check("", "roster")
	check("", "drop", "worker")
	if _, err := os.Stat(wp.Directory); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("drop did not remove agent: %v", err)
	}
	if out, err := runner.run("list-panes", "-s", "-t", session, "-F", "#{pane_id}"); err != nil || strings.Contains(out, worker.Pane+"\n") {
		t.Fatalf("expired deadline left the pane alive: %q %v", out, err)
	}
	if err := os.Chmod(team.Log, 0600); err != nil {
		t.Fatal(err)
	}
	if out := check("", "log", "--agent", string(second.ID), "--type", "delivery_succeeded"); !strings.Contains(out, "delivery_succeeded") {
		t.Fatal("delivery audit is absent")
	}
	check("", "rename", "second", "renamed")
	check("", "roster")
	check("", "down", "--yes")
	if _, err := os.Stat(team.Directory); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("team state remains: %v", err)
	}
}
