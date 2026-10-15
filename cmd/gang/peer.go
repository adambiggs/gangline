package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
	"github.com/adambiggs/gangline/substrate/tmux"
)

// peerRequest is one message an enrolled peer host's gang forwards here. That
// gang verified From as its own calling agent; this host trusts the enrolled
// host for that and observes neither. Host must name the host the ssh key that
// ran peer-receive is bound to; the sender's own reply check reads it back.
type peerRequest struct {
	Host string `json:"host"`
	Team string `json:"team"`
	ID   string `json:"id"`
	From string `json:"from"`
	To   string `json:"to"`
	Body string `json:"body"`
}

// peerReply is the one JSON line peer-receive prints once its team option has
// parsed. ID and Outcome mean the message was published here; Error means
// nothing was. A malformed --team is a usage error with no reply line.
type peerReply struct {
	ID      core.EnvelopeID `json:"id,omitempty"`
	Outcome string          `json:"outcome,omitempty"`
	Warning string          `json:"warning,omitempty"`
	Error   string          `json:"error,omitempty"`
}

// peerMessageIDPattern is the form of a sending gang's message ID. The
// receiver files it as peer-HOST.ID. No ID Gangline derives for its own
// notices, startup, or resume input starts with peer-, and a derived ID
// prefixes another, so a peer cannot name one whatever its host is called.
var peerMessageIDPattern = regexp.MustCompile(`^msg-[0-9a-f]{32}$`)

// maximumPeerRequestBytes admits a body at the message budget even when the
// sender's JSON escapes every byte as six (<).
const maximumPeerRequestBytes = 6*maximumMessageBytes + 1<<16

func (cmd command) peerReceive(args []string) (result error) {
	reply := peerReply{}
	defer func() {
		if result != nil && reply.ID == "" {
			reply.Error = result.Error()
		}
		result = errors.Join(result, writeJSON(cmd.stdout, reply))
	}()
	host := ""
	flags := boundFlagSet("peer-receive", map[string]any{"peer": &host})
	positionals, err := parseOptions(flags, args)
	if err != nil {
		return usageError("peer-receive: %v", err)
	}
	if err := noArguments(positionals, "peer-receive"); err != nil {
		return err
	}
	run, err := cmd.runtime()
	if err != nil {
		return err
	}
	// The ingress stands in for a peer host's gang. A registered agent here
	// sends with its own observed identity through gang send.
	if a, err := run.observedAgent(); err != nil {
		return err
	} else if a != nil {
		return refuseError("peer-receive is a peer host's ingress; registered agent %s sends with gang send", a.Name)
	}
	// The dedicated peer key binds this argument in its forced command.
	// The request cannot choose a different host; receiving-account shell
	// and state access remain trusted.
	if host == "" {
		return refuseError("peer-receive needs --peer HOST from the forced command of that peer's key; run gang peer-authorize HOST PUBKEY_FILE on this host to set one up")
	}
	if _, enrolled := run.settings.Peers[host]; !enrolled {
		return refuseError("peer host %q is not enrolled in GANG_PEERS", host)
	}
	if err := checkPeerTransport(cmd.peerProcesses()); err != nil {
		return err
	}
	request, err := readPeerRequest(cmd.stdin)
	if err != nil {
		return err
	}
	if request.Host != host {
		return refuseError("request names host %q, but this key is bound to %q", request.Host, host)
	}
	if request.Team != run.settings.Session {
		return refuseError("request names team %q, not the selected team %q", request.Team, run.settings.Session)
	}
	if !peerMessageIDPattern.MatchString(request.ID) {
		return refuseError("request message id %q is not msg- and 32 lowercase hex digits", request.ID)
	}
	if !validAgentName(request.From) {
		return refuseError("request sender %q is not an agent name (%s)", request.From, agentNameForm)
	}
	if !validAgentName(request.To) {
		return refuseError("request recipient %q is not an agent name (%s)", request.To, agentNameForm)
	}
	// Rendering the envelope below refuses a body over the budget or with
	// terminal controls before anything is published.
	if request.Body == "" {
		return refuseError("message body is empty")
	}
	a, err := run.resolve(request.To)
	if err != nil {
		return err
	}
	if a.Status != core.Active && a.Status != core.Booting {
		return inactiveRecipient(a)
	}
	p, err := run.team.Agent(a.ID)
	if err != nil {
		return err
	}
	if _, failedStartup, err := retainedStartup(p, "failed", a.LastFailed); err != nil {
		return err
	} else if failedStartup {
		return refuseError("startup contract input is unverified; inspect the recipient and run gang hitch %s --recover before sending another message", a.Name)
	}
	id := core.EnvelopeID("peer-" + host + "." + request.ID)
	alreadyPublished := refuseError("message %s was already published here; do not resend", id)
	// This check sees the receipts this agent retains. Of two requests that
	// pass it together, publishing queues only the first, so neither replaces
	// the other's body. A repeat that passes it after the first copy is
	// delivered is delivered again: a repeated ID is at-least-once.
	if published, err := envelopePublished(p, id); err != nil {
		return err
	} else if published {
		return alreadyPublished
	}
	token, err := randomEnvelopeToken()
	if err != nil {
		return err
	}
	from := core.Sender{Kind: core.SenderPeer, Name: core.AgentName(request.From), Host: host}
	e := core.Envelope{ID: id, Token: token, Recipient: a.ID, To: a.Name, From: from, Message: core.Message{Text: request.Body}, CreatedAt: cmd.now()}
	wire, err := envelopeText(e)
	if err != nil {
		return err
	}
	c, err := loadCollar(a.Collar, run.settings)
	if err != nil {
		return err
	}
	if err := refusePasteHazard(c, wire); err != nil {
		return err
	}
	outcome, step, postPublishErr, err := run.publishMessage(a, p, nil, e)
	if errors.Is(err, store.ErrEnvelopeQueued) {
		return alreadyPublished
	} else if err != nil {
		return err
	}
	reply.ID, reply.Outcome = e.ID, outcome
	if postPublishErr != nil {
		reply.Warning = fmt.Sprintf("message %s was retained with %s status; %s failed: %v", e.ID, outcome, step, postPublishErr)
		if outcome != "failed" {
			reply.Warning += "; do not resend"
		}
		if _, err := fmt.Fprintf(cmd.stderr, "warning: %s\n", reply.Warning); err != nil {
			return err
		}
	}
	return deliveryResult(outcome)
}

// peerTransportRefusal describes the required process ancestry.
const peerTransportRefusal = "peer-receive requires this account's sshd session under a root sshd process; run gang peer-authorize to bind a peer key"

// sshdCommands are the names OpenSSH gives its per-connection processes:
// sshd, or sshd-session where OpenSSH runs each session as its own program.
var sshdCommands = map[string]bool{"sshd": true, "sshd-session": true}

// processTable is the view of this host's processes the transport check
// reads.
type processTable interface {
	// parentPID is this process's parent, as getppid reports it.
	parentPID() int
	// userID is this process's real user ID.
	userID() uint32
	owner(pid int) (tmux.Owner, error)
}

type hostProcesses struct{}

func (hostProcesses) parentPID() int                    { return os.Getppid() }
func (hostProcesses) userID() uint32                    { return uint32(os.Getuid()) }
func (hostProcesses) owner(pid int) (tmux.Owner, error) { return tmux.ProcessOwner(pid) }

func (cmd command) peerProcesses() processTable {
	if cmd.processes != nil {
		return cmd.processes
	}
	return hostProcesses{}
}

// checkPeerTransport screens for an sshd process of this user under an sshd
// process whose observed user IDs are zero. It does not identify the SSH key
// or prove a forced command: an unrestricted login can supply --peer, and
// user-namespace processes can present this ancestry with namespace-local UIDs.
// Dedicated restricted keys bind peer labels; receiving-account access is trusted.
//
// The parent is read, then its parent, then the parent again. A process whose
// parent exits is moved to another parent, so a grandparent still named by
// the parent's second read is the one read between, and a getppid unchanged
// at the end means the parent read is this process's own: a PID reused in
// between cannot pass.
//
// The environment sshd sets is not consulted; whoever runs the command sets
// it.
func checkPeerTransport(processes processTable) error {
	pid := processes.parentPID()
	parent, err := processes.owner(pid)
	if err != nil {
		return refuseError("%s (read parent process %d: %v)", peerTransportRefusal, pid, err)
	}
	uid := processes.userID()
	if !sshdCommands[parent.Command] || parent.UID != uid || parent.RealUID != uid {
		return refuseError("%s (parent process %d is %q with user IDs %d real, %d effective; want sshd or sshd-session as user %d)", peerTransportRefusal, pid, parent.Command, parent.RealUID, parent.UID, uid)
	}
	grandparent, err := processes.owner(parent.ParentPID)
	if err != nil {
		return refuseError("%s (read sshd process %d: %v)", peerTransportRefusal, parent.ParentPID, err)
	}
	if !sshdCommands[grandparent.Command] || grandparent.UID != 0 || grandparent.RealUID != 0 {
		return refuseError("%s (process %d above sshd is %q with user IDs %d real, %d effective; want sshd or sshd-session as root)", peerTransportRefusal, parent.ParentPID, grandparent.Command, grandparent.RealUID, grandparent.UID)
	}
	again, err := processes.owner(pid)
	if err != nil {
		return refuseError("%s (read parent process %d again: %v)", peerTransportRefusal, pid, err)
	}
	if again.ParentPID != parent.ParentPID {
		return refuseError("%s (parent process %d moved from parent %d to %d while it was read)", peerTransportRefusal, pid, parent.ParentPID, again.ParentPID)
	}
	if now := processes.parentPID(); now != pid {
		return refuseError("%s (parent process changed from %d to %d while it was read)", peerTransportRefusal, pid, now)
	}
	return nil
}

// readPeerRequest reads exactly one bounded JSON request with no unknown
// fields and nothing after it.
func readPeerRequest(reader io.Reader) (peerRequest, error) {
	data, err := io.ReadAll(io.LimitReader(reader, maximumPeerRequestBytes+1))
	if err != nil {
		return peerRequest{}, fmt.Errorf("read peer request: %w", err)
	}
	if len(data) > maximumPeerRequestBytes {
		return peerRequest{}, refuseError("peer request exceeds %d bytes", maximumPeerRequestBytes)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var request peerRequest
	if err := decoder.Decode(&request); err != nil {
		return peerRequest{}, refuseError("peer request is not one JSON object of host, team, id, from, to, and body: %v", err)
	}
	if _, err := decoder.Token(); err != io.EOF {
		return peerRequest{}, refuseError("peer request has data after its JSON object")
	}
	return request, nil
}
