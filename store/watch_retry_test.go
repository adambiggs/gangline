package store

import (
	"errors"
	"syscall"
	"testing"
)

func TestWatchRetriesInterruptedCall(t *testing.T) {
	for _, final := range []error{nil, syscall.EBADF} {
		calls := 0
		err := retryInterrupted(func() error {
			calls++
			if calls <= 2 {
				return syscall.EINTR
			}
			return final
		})
		if calls != 3 || !errors.Is(err, final) {
			t.Fatalf("calls = %d, error = %v; want 3, %v", calls, err, final)
		}
	}
}
