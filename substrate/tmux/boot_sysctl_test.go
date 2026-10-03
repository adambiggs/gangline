package tmux

import (
	"encoding/binary"
	"errors"
	"testing"
)

func bootSysctlFixture(seconds uint64, microseconds uint32, uuid string) func(string) (string, error) {
	return func(name string) (string, error) {
		switch name {
		case "kern.boottime":
			// The 64-bit Darwin timeval has an eight-byte second field and
			// a four-byte microsecond field, followed by padding.
			var value [16]byte
			binary.LittleEndian.PutUint64(value[:8], seconds)
			binary.LittleEndian.PutUint32(value[8:12], microseconds)
			return string(value[:]), nil
		case "kern.bootsessionuuid":
			return uuid, nil
		default:
			return "", errors.New("unexpected sysctl: " + name)
		}
	}
}

func TestDarwinBootIdentityIgnoresBootTimeAdjustments(t *testing.T) {
	const uuid = "04283D74-5048-435E-BD31-478D0CFA2E1B"
	recorded, err := darwinBootIdentity(bootSysctlFixture(1700000000, 481376, uuid))
	if err != nil {
		t.Fatal(err)
	}
	for _, clock := range []struct {
		seconds      uint64
		microseconds uint32
	}{{1700000000, 532852}, {1700000001, 1}} {
		current, err := darwinBootIdentity(bootSysctlFixture(clock.seconds, clock.microseconds, uuid))
		if err != nil || current != recorded {
			t.Fatalf("same boot changed identity after boot-time adjustment: recorded=%q current=%q err=%v", recorded, current, err)
		}
	}
}

func TestDarwinBootIdentityDistinguishesBootSessions(t *testing.T) {
	recorded, err := darwinBootIdentity(bootSysctlFixture(1700000000, 481376, "04283D74-5048-435E-BD31-478D0CFA2E1B"))
	if err != nil {
		t.Fatal(err)
	}
	// Even identical wall-clock boot times cannot make a new boot own an old PID.
	current, err := darwinBootIdentity(bootSysctlFixture(1700000000, 481376, "C3653C74-1A6E-458D-BEF4-85E2F89845C6"))
	if err != nil || current == recorded {
		t.Fatalf("different boots share an identity: recorded=%q current=%q err=%v", recorded, current, err)
	}
}

func TestDarwinBootIdentityReadError(t *testing.T) {
	failure := errors.New("boot session unavailable")
	identity, err := darwinBootIdentity(func(name string) (string, error) {
		if name == "kern.bootsessionuuid" {
			return "", failure
		}
		return bootSysctlFixture(1700000000, 481376, "")(name)
	})
	if !errors.Is(err, failure) || identity != "" {
		t.Fatalf("boot session read failed but identity was returned: identity=%q err=%v", identity, err)
	}
}
