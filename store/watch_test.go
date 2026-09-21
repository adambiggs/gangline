package store

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestAppendWaitWakesOnEventLogAppend(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	wait, err := NewAppendWait(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer wait.Close()
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("event\n"); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := wait.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestAppendWaitObservesChangeDuringRegistration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	if err := os.WriteFile(path, []byte("event\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	wait, err := NewAppendWait(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer wait.Close()
	if err := wait.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestAppendWaitHonorsCancelledContext(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	wait, err := NewAppendWait(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer wait.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := wait.Wait(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("wait error = %v, want context cancellation", err)
	}
}
