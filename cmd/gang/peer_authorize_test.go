package main

import (
	"encoding/base64"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testPublicKey is the base64 of an OpenSSH key blob of keyType.
func testPublicKey(keyType string) string {
	blob := binary.BigEndian.AppendUint32(nil, uint32(len(keyType)))
	blob = append(blob, keyType...)
	blob = binary.BigEndian.AppendUint32(blob, 32)
	blob = append(blob, make([]byte, 32)...)
	return base64.StdEncoding.EncodeToString(blob)
}

func peerAuthorizeFixture(t *testing.T, key string) (*stateFixture, string) {
	t.Helper()
	f := newStateFixture(t)
	f.env["GANG_PEERS"] = `{"alpha":{"ssh":"alpha","gang":"/opt/gang","team":"unit"}}`
	f.cmd.executable = func() (string, error) { return "/home/adam/.local/bin/gang", nil }
	file := filepath.Join(t.TempDir(), "peer.pub")
	if err := os.WriteFile(file, []byte(key), 0o600); err != nil {
		t.Fatal(err)
	}
	return f, file
}

func TestPeerAuthorizePrintsTheForcedCommandLine(t *testing.T) {
	key := testPublicKey("ssh-ed25519")
	f, file := peerAuthorizeFixture(t, "ssh-ed25519 "+key+" adam@alpha laptop\n")
	if err := f.cmd.execute([]string{"peer-authorize", "alpha", file}); err != nil {
		t.Fatal(err)
	}
	want := `command="/home/adam/.local/bin/gang peer-receive --team unit --peer alpha",restrict ssh-ed25519 ` + key + " gangline-peer-alpha\n"
	if f.out.String() != want {
		t.Fatalf("line = %q, want %q", f.out.String(), want)
	}
	for _, step := range []string{"first line whose key matches", "IdentitiesOnly yes", `"ssh" in GANG_PEERS`, "GANG_HOST=alpha"} {
		if !strings.Contains(f.errOut.String(), step) {
			t.Errorf("stderr lacks %q:\n%s", step, f.errOut.String())
		}
	}
}

func TestPeerAuthorizeRefusesWhatItCannotBind(t *testing.T) {
	ed25519 := testPublicKey("ssh-ed25519")
	plain := "ssh-ed25519 " + ed25519 + " adam@alpha\n"
	for _, tc := range []struct {
		name, host, key string
		gang, team      string
		want            string
	}{
		{"unenrolled host", "beta", plain, "", "", `peer host "beta" is not enrolled`},
		{"key with options", "alpha", `command="/bin/sh",restrict ` + plain, "", "", "options before its key type"},
		{"key with one option", "alpha", "restrict " + plain, "", "", "options before its key type"},
		{"no key", "alpha", "\n", "", "", "holds 0 lines"},
		{"two keys", "alpha", plain + "ssh-rsa " + testPublicKey("ssh-rsa") + "\n", "", "", "holds 2 lines"},
		{"not a key line", "alpha", "-----BEGIN OPENSSH PRIVATE KEY-----\n", "", "", "not an OpenSSH public key line"},
		{"unsupported type", "alpha", "ssh-dss " + testPublicKey("ssh-dss") + "\n", "", "", "not an OpenSSH public key line"},
		{"no key data", "alpha", "ssh-ed25519\n", "", "", "no key data"},
		{"key data not base64", "alpha", "ssh-ed25519 AAAA\"x\n", "", "", "not base64"},
		{"key data of another type", "alpha", "ssh-ed25519 " + testPublicKey("ssh-rsa") + "\n", "", "", "does not encode a ssh-ed25519 key"},
		{"key data naming a longer type", "alpha", "ssh-ed25519 " + base64.StdEncoding.EncodeToString(append([]byte{0, 0, 0, 12}, "ssh-ed25519x"...)) + "\n", "", "", "does not encode a ssh-ed25519 key"},
		{"key data naming a same-length type", "alpha", "ssh-ed25519 " + testPublicKey("ssh-ed25518") + "\n", "", "", "does not encode a ssh-ed25519 key"},
		{"key data too short", "alpha", "ssh-ed25519 " + base64.StdEncoding.EncodeToString([]byte{0, 0, 0, 11, 's'}) + "\n", "", "", "does not encode"},
		{"gang path with a space", "alpha", plain, "/my bin/gang", "", "cannot appear in a forced command"},
		{"team with a space", "alpha", plain, "", "a b", `team "a b" cannot appear`},
		{"team with a quote", "alpha", plain, "", `x"y`, "cannot appear in a forced command"},
		{"gang path with a quote", "alpha", plain, `/bin/gang",x`, "", "cannot appear in a forced command"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, file := peerAuthorizeFixture(t, tc.key)
			if tc.team != "" {
				f.env["GANG_SESSION"] = tc.team
			}
			if tc.gang != "" {
				f.cmd.executable = func() (string, error) { return tc.gang, nil }
			}
			err := f.cmd.execute([]string{"peer-authorize", tc.host, file})
			var refusal commandError
			if !errors.As(err, &refusal) || refusal.status != exitRefused || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want a refusal containing %q", err, tc.want)
			}
			if f.out.Len() != 0 {
				t.Fatalf("refusal printed %q", f.out.String())
			}
		})
	}
}
