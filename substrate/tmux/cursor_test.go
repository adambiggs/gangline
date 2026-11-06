package tmux

import "testing"

func TestParseCursorKeepsTitleWithCommas(t *testing.T) {
	cursor, title, dead, err := parseCursor("3,7,1,0,,[ ! ] Action Required | Reply, then stop | cwd\n")
	if err != nil || dead != nil || title != "[ ! ] Action Required | Reply, then stop | cwd" || cursor.Column != 3 || cursor.Row != 7 || !cursor.Visible {
		t.Fatalf("cursor = %+v, title = %q, dead = %v, error = %v", cursor, title, dead, err)
	}
	if _, title, _, err := parseCursor("0,0,0,0,,\n"); err != nil || title != "" {
		t.Fatalf("unset title = %q, error = %v", title, err)
	}
	if _, _, dead, err := parseCursor("0,0,0,1,2,shell\n"); err != nil || dead == nil || *dead != "2" {
		t.Fatalf("dead = %v, error = %v", dead, err)
	}
	if _, _, _, err := parseCursor("0,0,0\n"); err == nil {
		t.Fatal("short cursor reading accepted")
	}
}
