package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"

	"github.com/adambiggs/gangline/core"
)

// peerRequest is one message an enrolled peer host's gang forwards here. That
// gang verified From as its own calling agent; this host trusts the enrolled
// host for that and observes neither.
type peerRequest struct {
	Host string `json:"host"`
	Team string `json:"team"`
	ID   string `json:"id"`
	From string `json:"from"`
	To   string `json:"to"`
	Body string `json:"body"`
}

// peerReply is the one JSON line peer-receive prints. ID and Outcome mean the
// message was published here; Error means nothing was.
type peerReply struct {
	ID      core.EnvelopeID `json:"id,omitempty"`
	Outcome string          `json:"outcome,omitempty"`
	Warning string          `json:"warning,omitempty"`
	Error   string          `json:"error,omitempty"`
}

// peerMessageIDPattern is the form of a sending gang's message ID. The
// receiver prefixes the host, so a peer cannot name any ID Gangline derives
// for its own notices, startup, or resume input.
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
	if err := noArguments(args, "peer-receive"); err != nil {
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
	request, err := readPeerRequest(cmd.stdin)
	if err != nil {
		return err
	}
	if _, enrolled := run.settings.Peers[request.Host]; !enrolled {
		return refuseError("peer host %q is not enrolled in GANG_PEERS", request.Host)
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
	id := core.EnvelopeID(request.Host + "." + request.ID)
	// Publishing renames over an existing file, which would replace a
	// message that may already be in the recipient's composer.
	if published, err := envelopePublished(p, id); err != nil {
		return err
	} else if published {
		return refuseError("message %s was already published here; do not resend", id)
	}
	token, err := randomEnvelopeToken()
	if err != nil {
		return err
	}
	from := core.Sender{Kind: core.SenderPeer, Name: core.AgentName(request.From), Host: request.Host}
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
	if err != nil {
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
