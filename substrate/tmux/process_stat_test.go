//go:build linux

package tmux

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func statFixture(pid, parent, group, foreground int, command, start string) string {
	fields := make([]string, 20)
	for i := range fields {
		fields[i] = "0"
	}
	fields[0], fields[1], fields[2], fields[5], fields[19] = "S", fmt.Sprint(parent), fmt.Sprint(group), fmt.Sprint(foreground), start
	return fmt.Sprintf("%d (%s) %s", pid, command, strings.Join(fields, " "))
}

func TestStatTablePreservesTreeForegroundAndIdentity(t *testing.T) {
	stats := map[int]string{
		100: statFixture(100, 1, 100, 200, "shell", "123400"),
		200: statFixture(200, 100, 200, 200, "name with ) (", "123401"),
		201: statFixture(201, 200, 200, 200, "helper", "123402"),
		300: statFixture(300, 200, 300, 200, "background", "123403"),
		400: statFixture(400, 1, 200, 200, "unrelated", "123404"),
	}
	records, err := collectProcessRecords(context.Background(), 100, []int{100, 200, 201, 300, 400}, false, func(pid int) (processRecord, error) { return parseProcStat(stats[pid]) })
	if err != nil {
		t.Fatal(err)
	}
	if records[200].Command != "name with ) (" || records[200].started != "123401" {
		t.Fatalf("record = %+v", records[200])
	}
	foreground, err := selectForegroundProcesses(100, records)
	if err != nil || len(foreground) != 2 || foreground[0].PID != 200 || foreground[1].PID != 201 {
		t.Fatalf("foreground = %+v, %v", foreground, err)
	}
	pane, err := selectPaneProcesses(100, records)
	if err != nil || len(pane) != 4 || pane[3].PID != 300 {
		t.Fatalf("pane = %+v, %v", pane, err)
	}
	changed, err := parseProcStat(statFixture(200, 100, 200, 200, "name with ) (", "123402"))
	if err != nil || changed.started == records[200].started {
		t.Fatalf("subsecond identity lost: %+v, %v", changed, err)
	}
}

func TestStatGroupsRejectMalformedFields(t *testing.T) {
	valid := statFixture(200, 100, 200, -1, "worker", "1234")
	if _, err := parseProcStat(valid); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{
		strings.Replace(valid, "S 100 200", "S 100 bad", 1),
		strings.Replace(valid, " 0 0 -1 ", " 0 0 bad ", 1),
		statFixture(200, -1, 200, 200, "worker", "1234"),
		statFixture(200, 100, -2, 200, "worker", "1234"),
		statFixture(200, 100, 200, -2, "worker", "1234"),
	} {
		if _, err := parseProcStat(bad); err == nil {
			t.Fatalf("accepted malformed stat %q", bad)
		}
	}
}

func TestStatTableSkipsProcessExitingDuringRead(t *testing.T) {
	read := func(path string) ([]byte, error) {
		switch path {
		case "/proc/100/stat":
			return []byte(statFixture(100, 1, 100, -1, "root", "1234")), nil
		case "/proc/200/stat":
			return nil, nil
		default:
			t.Fatalf("unexpected process read %q", path)
			return nil, nil
		}
	}
	records, err := collectProcessRecords(context.Background(), 100, []int{100, 200}, false, func(pid int) (processRecord, error) { return readProcessStat(pid, read) })
	if err != nil || len(records) != 1 || records[100].PID != 100 {
		t.Fatalf("records = %+v, %v", records, err)
	}
	_, err = collectProcessRecords(context.Background(), 100, []int{100}, false, func(pid int) (processRecord, error) {
		return readProcessStat(pid, func(string) ([]byte, error) { return []byte("malformed"), nil })
	})
	if err == nil {
		t.Fatal("nonempty malformed stat was treated as an exited process")
	}
}
