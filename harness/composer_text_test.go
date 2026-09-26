package harness

import "testing"

func TestSameComposerTextIgnoresWrap(t *testing.T) {
	submitted := "/compact Resume from /srv/state/lead/STATE.md: I am lead. Read it, confirm team via gang roster, then wait."
	wrapped := "/compact Resume from /srv/state/lead/STATE.md: I am\nlead. Read it, confirm team via gang roster,\nthen wait."
	if !SameComposerText(wrapped, submitted) {
		t.Fatal("wrapped composer read-back must match the submitted text")
	}
	if SameComposerText("unfinished draft", submitted) {
		t.Fatal("different text must not match")
	}
	if !SameComposerText("", "") || SameComposerText("", submitted) {
		t.Fatal("empty composer only matches empty text")
	}
}
