package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
)

func TestStartupDeliversStandingProseOnce(t *testing.T) {
	brief := startupProse{Contract: []byte("unique contract"), Doctrine: []byte("Whoever\ncreated this.\n\n    literal code\n"), Role: []byte("unique role")}
	for _, name := range []string{"claude-code", "codex"} {
		t.Run(name, func(t *testing.T) {
			collar, err := harness.EmbeddedCollar(name)
			if err != nil {
				t.Fatal(err)
			}
			prompt, message := startupMessages("worker", brief, "build the result", collar.Options.RolePrompt != nil)
			launch, err := harness.RenderLaunch(collar, harness.LaunchOptions{HookCommand: []string{"gang", "hook"}, RolePrompt: prompt})
			if err != nil {
				t.Fatal(err)
			}
			all := strings.Join(launch.Args, "\n") + "\n" + message
			for _, body := range []string{string(brief.Contract), string(brief.Doctrine), string(brief.Role), "build the result"} {
				if strings.Count(all, body) != 1 {
					t.Fatalf("expected one verbatim copy of %q in launch and message: %q", body, all)
				}
			}
			if strings.Contains(prompt, "build the result") {
				t.Fatal("assignment went into system prompt")
			}
			if collar.Options.RolePrompt != nil && message != "Assignment:\n\nbuild the result" {
				t.Fatalf("system-prompt collar also delivered standing prose: %q", message)
			}
		})
	}
}

func TestTasklessStartupReportsMissingAssignment(t *testing.T) {
	brief := startupProse{Contract: []byte("contract")}
	prompt, message := startupMessages("worker", brief, "", true)
	if prompt == "" || message != "No assignment was supplied." {
		t.Fatalf("taskless system startup: prompt=%q message=%q", prompt, message)
	}
	prompt, message = startupMessages("worker", brief, "", false)
	if prompt != "" || !strings.Contains(message, "contract") || !strings.HasSuffix(message, "No assignment was supplied.") || strings.Contains(message, "Assignment:") {
		t.Fatalf("taskless message startup: prompt=%q message=%q", prompt, message)
	}
}

func TestLocalProsePreservesBytesAndSubordinatesRoleToOperator(t *testing.T) {
	f := newStateFixture(t)
	config := f.env["GANG_CONFIG_DIR"]
	if err := os.MkdirAll(filepath.Join(config, "roles"), 0700); err != nil {
		t.Fatal(err)
	}
	doctrine := "Only Codex owners. Whoever\ncreated it owns it.\n\n    code\n"
	role := "Spread arcs evenly between providers.\n"
	for path, content := range map[string]string{"DOCTRINE.md": doctrine, "roles/lead.md": role} {
		if err := os.WriteFile(filepath.Join(config, path), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	brief, err := f.cmd.startupProse("lead")
	if err != nil {
		t.Fatal(err)
	}
	if string(brief.Doctrine) != doctrine || string(brief.Role) != role {
		t.Fatalf("local prose changed: %+v", brief)
	}
	message := composeStartup("lead", brief)
	if !strings.Contains(message, doctrine) || !strings.Contains(message, "Role brief (operator instructions take precedence, including provider, model, effort, and staffing policy):\n\n"+role) {
		t.Fatalf("role precedence or verbatim doctrine absent: %q", message)
	}
}

func TestObservedSenderUsesRegisteredAgentIdentity(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "lead-hitch", "lead", "codex")
	f.env["TMUX_PANE"] = a.Pane
	sender, err := f.run.observedSender()
	if err != nil || sender != (core.Sender{Kind: core.SenderAgent, Name: a.Name, HitchID: a.ID}) {
		t.Fatalf("observed sender: %+v %v", sender, err)
	}
	if _, err := f.run.sender("hitch"); err == nil {
		t.Fatal("registered agent could override its sender")
	}
	f.env["TMUX_PANE"] = "%unregistered"
	if sender, err := f.run.observedSender(); err != nil || sender.Kind != "" {
		t.Fatalf("unregistered pane claimed identity: %+v %v", sender, err)
	}
	if _, err := f.run.sender(""); err == nil {
		t.Fatal("ordinary send outside a registered pane needs --from")
	}
}
