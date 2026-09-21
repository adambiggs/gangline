package harness

import "testing"

func TestRenderCompactAction(t *testing.T) {
	collar, err := EmbeddedCollar("claude-code")
	if err != nil {
		t.Fatal(err)
	}
	action, err := RenderAction(collar.Actions.Compact, map[string]string{"instructions": "keep the decision"})
	if err != nil {
		t.Fatal(err)
	}
	if action.Text != "/compact keep the decision" || !action.Submit {
		t.Fatalf("action = %+v", action)
	}
}

func TestRenderActionPreservesBracesInUserValue(t *testing.T) {
	action, err := RenderAction(Action{Text: "/compact {{instructions}}"}, map[string]string{
		"instructions": "keep {{literal}}",
	})
	if err != nil {
		t.Fatal(err)
	}
	if action.Text != "/compact keep {{literal}}" {
		t.Fatalf("text = %q", action.Text)
	}
}

func TestSubmitBuildsAtomicTextAndEnter(t *testing.T) {
	action, err := Submit(Invocation{Name: "enter-submit"}, "hello")
	if err != nil {
		t.Fatal(err)
	}
	if action.Text != "hello" || !action.Submit {
		t.Fatalf("action = %+v", action)
	}
}

func TestActionConvertsToSubstrateKeys(t *testing.T) {
	keys := (Action{Keys: []string{"Escape", "Enter"}}).Input()
	if len(keys.Names) != 2 || keys.Names[0] != "Escape" || keys.Names[1] != "Enter" {
		t.Fatalf("keys = %+v", keys)
	}
}
