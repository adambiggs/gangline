//go:build darwin

package store

import (
	"errors"
	"syscall"
	"testing"
)

func TestWatchRetriesInterruptedRegistrationAndWait(t *testing.T) {
	for _, final := range []error{nil, syscall.EBADF} {
		registered, waited := 0, 0
		events, closeWatch, err := newFileWatchKevent(t.TempDir(), func(_ int, changes, events []syscall.Kevent_t, _ *syscall.Timespec) (int, error) {
			if len(changes) != 0 {
				registered++
				if registered <= 2 {
					return -1, syscall.EINTR
				}
				return 0, nil
			}
			waited++
			if waited <= 2 {
				return -1, syscall.EINTR
			}
			return 1, final
		})
		if err != nil {
			t.Fatal(err)
		}
		err = <-events
		if registered != 3 || waited != 3 || !errors.Is(err, final) {
			t.Errorf("registration calls = %d, wait calls = %d, error = %v; want 3, 3, %v", registered, waited, err, final)
		}
		if err := closeWatch(); err != nil {
			t.Fatal(err)
		}
	}
}
