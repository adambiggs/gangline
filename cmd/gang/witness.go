package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"syscall"
	"time"

	"github.com/adambiggs/gangline/core"
)

// Opening the read descriptor is the readiness barrier, before native input.
// Wait for the first readable event rather than treating the initial absence
// of a writer as EOF. Once a writer connects, its close terminates the receipt,
// including an empty or interrupted handoff. There is no keepalive writer.
func openNativeWitness(path string) (*os.File, <-chan nativeWitnessResult, error) {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return nil, nil, err
	}
	if err := syscall.Mkfifo(path, 0600); err != nil {
		return nil, nil, err
	}
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, nil, err
	}
	raw, err := file.SyscallConn()
	if err != nil {
		file.Close()
		return nil, nil, err
	}
	result := make(chan nativeWitnessResult, 1)
	go func() {
		first := make([]byte, 4096)
		n := 0
		var readErr error
		waited := false
		err := raw.Read(func(fd uintptr) bool {
			n, readErr = syscall.Read(int(fd), first)
			if errors.Is(readErr, syscall.EAGAIN) || errors.Is(readErr, syscall.EINTR) || n == 0 && readErr == nil && !waited {
				waited = true
				return false
			}
			return true
		})
		if err == nil {
			err = readErr
		}
		if err != nil {
			result <- nativeWitnessResult{err: err}
			return
		}
		if n == 0 {
			result <- nativeWitnessResult{err: io.ErrUnexpectedEOF}
			return
		}
		rest, err := io.ReadAll(io.LimitReader(file, int64(maximumHookBytes+1-n)))
		data := append(first[:n], rest...)
		if len(data) > maximumHookBytes {
			err = fmt.Errorf("native submit witness exceeds maximum size")
		}
		result <- nativeWitnessResult{data: data, err: err}
	}()
	return file, result, nil
}

func (run *runtime) publishNativeWitness(id core.HitchID, prompt string, pending bool) error {
	file, err := os.OpenFile(run.deliveryWitnessPath(id), os.O_WRONLY|syscall.O_NONBLOCK, 0)
	if errors.Is(err, os.ErrNotExist) && !pending {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open native submission witness: %w", err)
	}
	if prompt == "" {
		return errors.Join(fmt.Errorf("native submission witness carries no prompt"), file.Close())
	}
	_, writeErr := io.WriteString(file, prompt)
	err = errors.Join(writeErr, file.Close())
	if err != nil {
		return fmt.Errorf("write native submission witness: %w", err)
	}
	return nil
}

// A failed hook handoff settles its own in-flight send. The sender observes
// this terminal result at its next liveness checkpoint instead of waiting on
// a receipt that the hook has already failed to publish.
func (run *runtime) failPendingWitness(id core.HitchID, cause error) error {
	state, err := run.load()
	if err != nil {
		return err
	}
	hitch, found := state.Hitches[id]
	if !found {
		return nil
	}
	for _, delivery := range state.Deliveries {
		if delivery.Envelope.To == hitch.Name && delivery.Status == core.DeliveryDelivering {
			_, err := run.drive(core.DeliveryUnverifiedEvent{At: time.Now(), EnvelopeID: delivery.Envelope.ID, Evidence: "native submit hook failed: " + cause.Error()})
			return err
		}
	}
	return nil
}
