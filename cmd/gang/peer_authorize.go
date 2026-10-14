package main

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"os"
	"strings"
)

// peerKeyTypes are the OpenSSH public key types a peer key may have.
var peerKeyTypes = map[string]bool{
	"ssh-ed25519": true, "ssh-rsa": true,
	"ecdsa-sha2-nistp256": true, "ecdsa-sha2-nistp384": true, "ecdsa-sha2-nistp521": true,
	"sk-ssh-ed25519@openssh.com": true, "sk-ecdsa-sha2-nistp256@openssh.com": true,
}

// peerAuthorize prints the authorized_keys line that binds a peer host's key
// to peer-receive for that host and the selected team. sshd runs the line's
// command whatever the client asks for, so the key can reach only
// peer-receive, and --peer names the host from this host's own enrolment.
func (cmd command) peerAuthorize(args []string) error {
	if err := exactly(args, 2, "peer-authorize"); err != nil {
		return err
	}
	host, keyFile := args[0], args[1]
	run, err := cmd.runtime()
	if err != nil {
		return err
	}
	if _, enrolled := run.settings.Peers[host]; !enrolled {
		return refuseError("peer host %q is not enrolled in GANG_PEERS; enroll it before authorizing its key", host)
	}
	team := run.settings.Session
	if !peerLoginPattern.MatchString(team) {
		return refuseError("team %q cannot appear in a forced command; want letters, digits, '.', '_' or '-'", team)
	}
	executable := os.Executable
	if cmd.executable != nil {
		executable = cmd.executable
	}
	gang, err := executable()
	if err != nil {
		return fmt.Errorf("locate this gang: %w", err)
	}
	if !peerGangPattern.MatchString(gang) {
		return refuseError("this gang's path %q cannot appear in a forced command; want an absolute path of letters, digits, '.', '_', '+', '-' or '/'", gang)
	}
	data, err := os.ReadFile(keyFile)
	if err != nil {
		return fmt.Errorf("read public key: %w", err)
	}
	keyType, key, err := parsePeerKey(keyFile, data)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(cmd.stdout, "command=\"%s peer-receive --team %s --peer %s\",restrict %s %s gangline-peer-%s\n", gang, team, host, keyType, key, host); err != nil {
		return err
	}
	_, err = fmt.Fprintf(cmd.stderr, `Append that line to this account's ~/.ssh/authorized_keys. sshd uses the
first line whose key matches, so a line that already authorizes this key
without the forced command leaves it unbound; remove that line.
On %[1]s:
  - keep this key pair for this link only;
  - add an ssh alias for this host with its HostName, IdentityFile set to the
    private key, IdentitiesOnly yes, and ControlPath none, so ssh neither
    offers another key first nor reuses a connection another key opened;
  - set that alias as "ssh" in GANG_PEERS for this host, and GANG_HOST=%[1]s.
`, host)
	return err
}

// parsePeerKey reads a public key file of exactly one OpenSSH key line,
// TYPE BASE64 [COMMENT], with no options. It returns the type and key; the
// comment is dropped.
func parsePeerKey(name string, data []byte) (string, string, error) {
	var lines []string
	for _, line := range strings.Split(string(data), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}
	if len(lines) != 1 {
		return "", "", refuseError("%s holds %d lines; want exactly one public key", name, len(lines))
	}
	fields := strings.Fields(lines[0])
	if !peerKeyTypes[fields[0]] {
		for _, field := range fields[1:] {
			if peerKeyTypes[field] {
				return "", "", refuseError("%s gives options before its key type; give the plain public key file", name)
			}
		}
		return "", "", refuseError("%s is not an OpenSSH public key line (TYPE BASE64 [COMMENT]) of a supported type", name)
	}
	if len(fields) < 2 {
		return "", "", refuseError("%s has no key data after %s", name, fields[0])
	}
	blob, err := base64.StdEncoding.DecodeString(fields[1])
	if err != nil {
		return "", "", refuseError("%s key data is not base64: %v", name, err)
	}
	// An OpenSSH key blob starts with its type as a length-prefixed string.
	keyType := []byte(fields[0])
	if len(blob) < 4+len(keyType) || binary.BigEndian.Uint32(blob) != uint32(len(keyType)) || !bytes.Equal(blob[4:4+len(keyType)], keyType) {
		return "", "", refuseError("%s key data does not encode a %s key", name, fields[0])
	}
	return fields[0], fields[1], nil
}
