package tmux

import (
	"errors"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/substrate"
)

// frames replays snapshots in order.
type frames struct {
	snapshots []paneSnapshot
	taken     int
}

func (f *frames) take() (paneSnapshot, error) {
	if f.taken == len(f.snapshots) {
		return paneSnapshot{}, errors.New("no frame left")
	}
	f.taken++
	return f.snapshots[f.taken-1], nil
}

func TestSettledCapture(t *testing.T) {
	prompt := substrate.Cursor{Row: 1, Column: 2, Visible: true}
	torn := paneSnapshot{cursor: prompt, raw: "────────\n❯ hello\n────"}
	whole := paneSnapshot{cursor: prompt, raw: "────────\n❯ hello\n────────\n  auto mode on"}
	moved := whole
	moved.cursor = substrate.Cursor{Row: 3, Visible: true}
	retitled := whole
	retitled.title = "✳ working"
	tests := []struct {
		name      string
		snapshots []paneSnapshot
		want      paneSnapshot
		taken     int
	}{
		{name: "still screen takes one extra snapshot", snapshots: []paneSnapshot{whole, whole}, want: whole, taken: 2},
		{name: "torn frame is not returned", snapshots: []paneSnapshot{torn, whole, whole}, want: whole, taken: 3},
		{name: "screen settles after repaints", snapshots: []paneSnapshot{torn, {raw: "a"}, {raw: "b"}, whole, whole}, want: whole, taken: 5},
		{name: "a cursor parked elsewhere is not settled", snapshots: []paneSnapshot{moved, whole, whole}, want: whole, taken: 3},
		{name: "a title change alone settles with the newer title", snapshots: []paneSnapshot{whole, retitled}, want: retitled, taken: 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := &frames{snapshots: test.snapshots}
			got, err := settledCapture(f.take)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("got %+v, want %+v", got, test.want)
			}
			if f.taken != test.taken {
				t.Fatalf("took %d snapshots, want %d", f.taken, test.taken)
			}
		})
	}
}

func TestSettledCaptureReturnsTheLastSnapshotOfAScreenThatNeverSettles(t *testing.T) {
	f := &frames{}
	for i := 0; i <= snapshotLimit; i++ {
		f.snapshots = append(f.snapshots, paneSnapshot{raw: strings.Repeat("x", i+1)})
	}
	got, err := settledCapture(f.take)
	if err != nil {
		t.Fatal(err)
	}
	if f.taken != snapshotLimit {
		t.Fatalf("took %d snapshots, want %d", f.taken, snapshotLimit)
	}
	if want := f.snapshots[snapshotLimit-1]; got != want {
		t.Fatalf("got %+v, want the last snapshot %+v", got, want)
	}
}

func TestSettledCaptureReturnsReadErrors(t *testing.T) {
	f := &frames{}
	if _, err := settledCapture(f.take); err == nil {
		t.Fatal("a failed first snapshot returned a screen")
	}
	f = &frames{snapshots: []paneSnapshot{{raw: "a"}}}
	if _, err := settledCapture(f.take); err == nil {
		t.Fatal("a failed second snapshot returned a screen")
	}
}

func TestParseSnapshot(t *testing.T) {
	got, dead, err := parseSnapshot("4,2,1,0,,✳ a, b\nfirst\nsecond\n")
	if err != nil {
		t.Fatal(err)
	}
	want := paneSnapshot{cursor: substrate.Cursor{Row: 2, Column: 4, Visible: true}, title: "✳ a, b", raw: "first\nsecond\n"}
	if got != want || dead != nil {
		t.Fatalf("got %+v dead %v, want %+v alive", got, dead, want)
	}
	_, dead, err = parseSnapshot("0,0,0,1,3,\nlast words\n")
	if err != nil || dead == nil || *dead != "3" {
		t.Fatalf("dead = %v, err = %v, want exit status 3", dead, err)
	}
	if _, _, err := parseSnapshot("4,2,1,0,,title"); err == nil {
		t.Fatal("a snapshot with no screen parsed")
	}
}
