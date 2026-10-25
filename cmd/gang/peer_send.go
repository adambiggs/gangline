package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
	"unicode"

	"github.com/adambiggs/gangline/core"
)

// peerSendTimeout bounds one forwarded send from ssh's start to the peer's
// reply. The peer's own send path finishes within its operation and collar
// settle bounds, well inside this. Too short reads a slow delivery as unknown,
// which is safe; too long holds the sender while a peer hangs.
const peerSendTimeout = 2 * time.Minute

// peerReplyBytes keeps the end of ssh's output. The reply is the last stdout
// line, so text a login shell prints before it cannot push it out.
const peerReplyBytes = 64 << 10

// sendToPeer forwards a message from this host's verified sender to agent
// o.Name on enrolled peer host o.Host. The peer's gang publishes and drains
// it, and its reply is the outcome. A reply that never arrives is unknown:
// the peer may have published the message, and nothing here can tell.
func (run *runtime) sendToPeer(o sendOptions) error {
	peer, enrolled := run.settings.Peers[o.Host]
	if !enrolled {
		return refuseError("peer host %q is not enrolled in GANG_PEERS", o.Host)
	}
	if run.settings.Host == "" {
		return refuseError("GANG_HOST is unset; set it to this host's name as its peers enroll it")
	}
	var sender core.Sender
	var err error
	if o.As != "" {
		sender, err = run.sendSender(o)
	} else {
		sender, err = run.observedSender()
	}
	if err != nil {
		return err
	}
	if sender.Kind != core.SenderAgent && sender.Kind != core.SenderRegistered {
		return refuseError("only an active registered agent or credentialed --as sender sends to a peer host; run gang send on %s instead", o.Host)
	}
	bodyReader := run.cmd.stdin
	if o.Body != nil {
		bodyReader = strings.NewReader(*o.Body)
	}
	body, err := readBody(bodyReader)
	if err != nil {
		return err
	}
	id, err := randomID("msg")
	if err != nil {
		return err
	}
	class := ""
	if sender.Kind == core.SenderRegistered {
		class = core.SenderRegistered
	}
	request, err := json.Marshal(peerRequest{Class: class, Host: run.settings.Host, Team: peer.Team, ID: id, From: string(sender.Name), To: o.Name, Body: body})
	if err != nil {
		return err
	}
	ssh, err := exec.LookPath("ssh")
	if err != nil {
		return refuseError("cannot send to peer host %s: %v", o.Host, err)
	}
	// -T keeps a forced tty from echoing or line-limiting the request.
	args := []string{"-T", "-o", "BatchMode=yes", "-o", "ConnectTimeout=10", "-o", "ServerAliveInterval=10", "-o", "ServerAliveCountMax=3"}
	if peer.User != "" {
		args = append(args, "-l", peer.User)
	}
	// The message travels only on stdin; the remote command line holds the
	// fixed words and the validated path and team.
	args = append(args, "--", peer.SSH, peer.Gang, "peer-receive", "--team", peer.Team)
	ctx, cancel := run.cmd.timeout(peerSendTimeout)
	defer cancel()
	transport := exec.CommandContext(ctx, ssh, args...)
	transport.Stdin = bytes.NewReader(request)
	stdout, stderr := &tailBuffer{limit: peerReplyBytes}, &tailBuffer{limit: peerReplyBytes}
	transport.Stdout, transport.Stderr = stdout, stderr
	// A process ssh leaves holding its output must not hold this send.
	transport.WaitDelay = 5 * time.Second
	runErr := transport.Run()

	remoteID := core.EnvelopeID("peer-" + run.settings.Host + "." + id)
	reply, replied := lastPeerReply(stdout.data, remoteID)
	// An error line counts only when the peer's gang exited with a status that
	// says it published nothing. With ssh's own failure or a cut-off run, the
	// line may be noise a login shell printed before the peer published.
	var exit *exec.ExitError
	refused := errors.As(runErr, &exit) && exit.ExitCode() >= exitError && exit.ExitCode() <= exitNative
	// The peer's log records what it received; this host's log records what
	// its sender sent, once ssh has run, whether or not it reached the peer.
	sent := core.Event{Type: "peer_sent", ID: string(remoteID), To: o.Host + ":" + o.Name}
	switch {
	case replied && reply.ID != "":
		sent.Status = reply.Outcome
		if err := run.recordPeerSend(sender, sent); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(run.cmd.stdout, "%s:%s\t%s\n", o.Host, reply.ID, reply.Outcome); err != nil {
			return err
		}
		if reply.Outcome == "accepted" {
			if _, err := fmt.Fprintln(run.cmd.stderr, "native queue accepted the message; do not resend"); err != nil {
				return err
			}
		}
		if reply.Warning != "" {
			if _, err := fmt.Fprintf(run.cmd.stderr, "warning: %s: %s\n", o.Host, printable(reply.Warning)); err != nil {
				return err
			}
		}
		// This host cannot inspect the peer's pane, and the peer sends no later
		// notice here, so the reply is the last word on an uncertain receipt.
		if reply.Outcome == "unverified" {
			return commandError{status: exitUnknown, text: fmt.Sprintf("message input on %s is unverified; this host can neither inspect the recipient on %s nor learn a later outcome, so ask %s:%s whether it arrived before sending again", o.Host, o.Host, o.Host, o.Name)}
		}
		return nil
	case replied && refused:
		sent.Status, sent.Reason = "refused", printable(reply.Error)
		if err := run.recordPeerSend(sender, sent); err != nil {
			return err
		}
		if class != "" && strings.Contains(reply.Error, `unknown field "class"`) {
			return commandError{status: exit.ExitCode(), text: fmt.Sprintf("peer host %s does not understand sender classes; upgrade Gangline on %s before sending as a registered sender: %s", o.Host, o.Host, printable(reply.Error))}
		}
		// The peer words its refusal for its own host, where any command it
		// names must run, so that is said before the reason.
		return commandError{status: exit.ExitCode(), text: fmt.Sprintf("peer host %s refused the message; any command in its reason runs on %s, not here: %s", o.Host, o.Host, printable(reply.Error))}
	}
	cause := "its last line was not a reply to this message"
	if ctx.Err() != nil {
		cause = fmt.Sprintf("no reply within %s", peerSendTimeout)
	} else if runErr != nil {
		cause = "ssh: " + runErr.Error()
	}
	if line := lastLine(stderr.data); line != "" {
		cause += ": " + line
	}
	sent.Status, sent.Reason = "unknown", cause
	if err := run.recordPeerSend(sender, sent); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(run.cmd.stdout, "%s:%s\tunknown\n", o.Host, remoteID); err != nil {
		return err
	}
	return commandError{status: exitUnknown, text: fmt.Sprintf("peer host %s gave no outcome (%s); message %s may have been published there, so resend it only once no gang peer-receive runs on %s and its gang queue and gang log both lack it", o.Host, cause, remoteID, o.Host)}
}

// recordPeerSend logs a forwarded send under the identity that sent it. The
// message has gone whatever the log says, so a failed append is a warning and
// leaves the send's own result in place.
func (run *runtime) recordPeerSend(sender core.Sender, e core.Event) error {
	err := run.record(core.Agent{ID: sender.HitchID, Name: sender.Name}, e)
	if err == nil {
		return nil
	}
	_, err = fmt.Fprintf(run.cmd.stderr, "warning: message %s to %s is missing from this host's team log: %v\n", e.ID, e.To, err)
	return err
}

// peerOutcomes are the outcomes a peer's publish reports.
var peerOutcomes = map[string]bool{"queued": true, "delivered": true, "accepted": true, "failed": true, "unverified": true}

// lastPeerReply decodes the last non-blank line of a peer's stdout. It is a
// reply only if it names message id with a known outcome, or the error that
// published nothing.
func lastPeerReply(output []byte, id core.EnvelopeID) (peerReply, bool) {
	var reply peerReply
	if err := json.Unmarshal([]byte(lastLine(output)), &reply); err != nil {
		return peerReply{}, false
	}
	if reply.ID == id && peerOutcomes[reply.Outcome] || reply.ID == "" && reply.Outcome == "" && reply.Error != "" {
		return reply, true
	}
	return peerReply{}, false
}

// lastLine is output's last non-blank line, made printable.
func lastLine(output []byte) string {
	lines := strings.Split(string(output), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if line := strings.TrimSpace(lines[i]); line != "" {
			return printable(line)
		}
	}
	return ""
}

// printable defangs envelope-shaped text and replaces remote display controls.
func printable(text string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || invisibleMessageRune(r) {
			return '?'
		}
		return r
	}, defangEnvelopeTags(text))
}

// tailBuffer keeps the last limit bytes written to it.
type tailBuffer struct {
	limit int
	data  []byte
}

func (b *tailBuffer) Write(p []byte) (int, error) {
	b.data = append(b.data, p...)
	if excess := len(b.data) - b.limit; excess > 0 {
		b.data = append(b.data[:0], b.data[excess:]...)
	}
	return len(p), nil
}
