package acceptance

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

const nativeExitReady = 0xa5

// nativeExitWitness retains a connection and its EOF even when the native
// process exits before the test accepts it. The readiness byte distinguishes
// that exit from a connection that never established the witness.
type nativeExitWitness struct {
	listener *net.UnixListener
	ready    func()
}

func newNativeExitWitness(t *testing.T) *nativeExitWitness {
	t.Helper()
	root, err := os.MkdirTemp(os.TempDir(), "native-exit-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(root); err != nil {
			t.Error(err)
		}
	})
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: filepath.Join(root, "exit.sock"), Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			t.Error(err)
		}
	})
	return &nativeExitWitness{listener: listener}
}
func (w *nativeExitWitness) address() string { return w.listener.Addr().String() }
func (w *nativeExitWitness) wait(ctx context.Context) error {
	stopAccept := context.AfterFunc(ctx, func() { _ = w.listener.Close() })
	conn, err := w.listener.AcceptUnix()
	stopAccept()
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("accept native exit witness: %w", err)
	}
	defer conn.Close()
	stopRead := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stopRead()
	err = readNativeExitReady(conn, w.ready)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}
func readNativeExit(reader io.Reader) error { return readNativeExitReady(reader, nil) }
func readNativeExitReady(reader io.Reader, readyObserved func()) error {
	var ready [1]byte
	if _, err := io.ReadFull(reader, ready[:]); err != nil {
		return fmt.Errorf("read native exit readiness: %w", err)
	}
	if ready[0] != nativeExitReady {
		return fmt.Errorf("invalid native exit readiness byte %x", ready[0])
	}
	if readyObserved != nil {
		readyObserved()
	}
	if _, err := io.Copy(io.Discard, reader); err != nil {
		return fmt.Errorf("await native exit EOF: %w", err)
	}
	return nil
}

// The raw descriptor has no finalizer and stays open until the native process
// exits. Close-on-exec prevents any child from prolonging its exit witness.
func openNativeExitWitness(address string) (int, error) {
	fd, err := syscall.Socket(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		return -1, err
	}
	syscall.CloseOnExec(fd)
	if err := syscall.Connect(fd, &syscall.SockaddrUnix{Name: address}); err != nil {
		syscall.Close(fd)
		return -1, err
	}
	if n, err := syscall.Write(fd, []byte{nativeExitReady}); err != nil || n != 1 {
		syscall.Close(fd)
		if err == nil {
			err = io.ErrShortWrite
		}
		return -1, err
	}
	return fd, nil
}

func TestNativeExitWitnessRetainsExitBeforeAccept(t *testing.T) {
	witness := newNativeExitWitness(t)
	fd, err := openNativeExitWitness(witness.address())
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.Close(fd); err != nil {
		t.Fatal(err)
	}
	if err := witness.wait(context.Background()); err != nil {
		t.Fatal(err)
	}
}
func TestNativeExitWitnessRejectsMissingReadiness(t *testing.T) {
	for _, data := range [][]byte{nil, {0}} {
		if err := readNativeExit(bytes.NewReader(data)); err == nil {
			t.Fatal("unestablished witness accepted as an exit")
		}
	}
}

type gatedExitReader struct {
	ready         bool
	reading, exit chan struct{}
}

func (r *gatedExitReader) Read(b []byte) (int, error) {
	if !r.ready {
		r.ready = true
		b[0] = nativeExitReady
		return 1, nil
	}
	close(r.reading)
	<-r.exit
	return 0, io.EOF
}
func TestNativeExitWitnessRequiresEOF(t *testing.T) {
	reader := &gatedExitReader{reading: make(chan struct{}), exit: make(chan struct{})}
	result := make(chan error, 1)
	go func() { result <- readNativeExit(reader) }()
	select {
	case <-reader.reading:
	case err := <-result:
		t.Fatalf("readiness claimed exit: %v", err)
	}
	select {
	case err := <-result:
		t.Fatalf("readiness claimed exit: %v", err)
	default:
	}
	close(reader.exit)
	if err := <-result; err != nil {
		t.Fatal(err)
	}
}

func TestNativeExitWitnessCanceledAccept(t *testing.T) {
	witness := newNativeExitWitness(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := witness.wait(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("accept cancellation: %v", err)
	}
}
func TestNativeExitWitnessCanceledRead(t *testing.T) {
	witness := newNativeExitWitness(t)
	fd, err := openNativeExitWitness(witness.address())
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.Close(fd)
	observed := make(chan struct{})
	witness.ready = func() { close(observed) }
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() { result <- witness.wait(ctx) }()
	<-observed
	cancel()
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("read cancellation: %v", err)
	}
}
