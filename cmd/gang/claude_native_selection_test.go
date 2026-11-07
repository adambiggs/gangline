package main

import (
	"testing"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
)

func claudeNativeSelectionFixture() (core.Agent, core.Envelope, harness.Collar) {
	a := core.Agent{Status: core.Active, Activity: core.Idle, Collar: "codex"}
	e := core.Envelope{}
	c := harness.Collar{Name: "codex"}
	c.Primitives.NativeDelivery = &harness.Invocation{Name: "claude-peer"}
	return a, e, c
}

func TestClaudeNativeEligibleUsesDeclaredCapability(t *testing.T) {
	a, e, c := claudeNativeSelectionFixture()
	if !claudeNativeEligible(a, e, c) {
		t.Fatal("declared claude-peer capability should be eligible regardless of collar name")
	}

	t.Run("nil primitive", func(t *testing.T) {
		withoutCapability := c
		withoutCapability.Primitives.NativeDelivery = nil
		if claudeNativeEligible(a, e, withoutCapability) {
			t.Fatal("nil native-delivery primitive should be ineligible")
		}
	})
	t.Run("different primitive", func(t *testing.T) {
		wrongCapability := c
		wrongCapability.Primitives.NativeDelivery = &harness.Invocation{Name: "other"}
		if claudeNativeEligible(a, e, wrongCapability) {
			t.Fatal("a different native-delivery primitive should be ineligible")
		}
	})
	t.Run("Codex without capability", func(t *testing.T) {
		codex := harness.Collar{Name: "codex"}
		if claudeNativeEligible(a, e, codex) {
			t.Fatal("Codex without native-delivery capability should be ineligible")
		}
	})
}

func TestClaudeNativeEligibleAllowsOrdinarySenderClasses(t *testing.T) {
	a, _, c := claudeNativeSelectionFixture()
	for _, sender := range []struct {
		name string
		kind string
	}{
		{"agent", core.SenderAgent},
		{"self-declared", core.SenderSelfDeclared},
		{"outside", core.SenderOutside},
		{"Gangline notice", core.SenderGangline},
	} {
		t.Run(sender.name, func(t *testing.T) {
			e := core.Envelope{From: core.Sender{Kind: sender.kind}}
			if !claudeNativeEligible(a, e, c) {
				t.Fatalf("ordinary %s envelope should be eligible", sender.name)
			}
		})
	}
}

func TestClaudeNativeEligibleRejectsSpecialPurposesAndStartupSections(t *testing.T) {
	a, _, c := claudeNativeSelectionFixture()
	for _, purpose := range []string{"startup", "resume", "capacity"} {
		t.Run(purpose, func(t *testing.T) {
			e := core.Envelope{From: core.Sender{Kind: core.SenderAgent}, Purpose: purpose}
			if claudeNativeEligible(a, e, c) {
				t.Fatalf("%q purpose should be ineligible", purpose)
			}
		})
	}
	t.Run("capacity recovery without purpose", func(t *testing.T) {
		e := core.Envelope{From: core.Sender{Kind: core.SenderGangline, Name: "capacity-recovery"}}
		if claudeNativeEligible(a, e, c) {
			t.Fatal("capacity recovery must retain its dedicated admission checks")
		}
	})
	t.Run("startup sections without purpose", func(t *testing.T) {
		e := core.Envelope{From: core.Sender{Kind: core.SenderAgent}, Startup: &core.StartupSections{Contract: "contract"}}
		if claudeNativeEligible(a, e, c) {
			t.Fatal("envelope with startup sections should be ineligible even without a purpose")
		}
	})
}

func TestClaudeNativeEligibleRejectsInterruptedAndCompactingAgents(t *testing.T) {
	_, e, c := claudeNativeSelectionFixture()
	t.Run("interrupting", func(t *testing.T) {
		a := core.Agent{Status: core.Active, Activity: core.Interrupting}
		if claudeNativeEligible(a, e, c) {
			t.Fatal("interrupting agent should be ineligible")
		}
	})
	t.Run("compaction", func(t *testing.T) {
		a := core.Agent{Status: core.Active, Activity: core.Idle, Compaction: &core.Compaction{ID: "compact-1"}}
		if claudeNativeEligible(a, e, c) {
			t.Fatal("agent with a compaction should be ineligible")
		}
	})
}
