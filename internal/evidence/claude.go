package evidence

import (
	"bytes"
	"strings"

	"github.com/adambiggs/gangline/harness"
)

// NativeView exposes selected Claude identities and a locator, never tool text.
type NativeView struct {
	Harness      string `json:"harness"`
	Session      string `json:"session"`
	Turn         string `json:"turn"`
	ToolCall     string `json:"tool_call"`
	Message      string `json:"message_id,omitempty"`
	Request      string `json:"request_id,omitempty"`
	CallRecord   string `json:"call_record,omitempty"`
	ResultRecord string `json:"result_record,omitempty"`
	ResultOffset int64  `json:"result_end_byte,omitempty"`
}

func (v *verifier) claudeRun(id string, c *Code, r *Run) {
	if r.Session == "" || r.Turn == "" || r.Implementer == "" {
		v.issue(id, "incomplete", "native run identity missing")
		return
	}
	b, ok := v.artifact(r.Native)
	if !ok {
		return
	}
	turn, err := harness.ReadClaudeTurn(bytes.NewReader(b), r.Session, r.Turn, c.NativeCall)
	if err != nil {
		v.issue(id, "fail", "native transcript identity or records invalid")
		return
	}
	// Only identity-shaped strings enter the metadata view. Unrecognized values
	// remain retained in the artifact; they cannot inject payloads into output.
	identity := func(s string) string {
		if identifier.MatchString(s) {
			return s
		}
		return ""
	}
	v.result.Native = &NativeView{Harness: "claude", Session: identity(r.Session), Turn: identity(r.Turn), ToolCall: identity(c.NativeCall), Message: identity(turn.MessageID), Request: identity(turn.RequestID), CallRecord: identity(turn.CallRecord), ResultRecord: identity(turn.ResultRecord), ResultOffset: turn.ResultOffset}
	if turn.Offset != int64(len(b)) {
		v.issue(id, "incomplete", "native transcript has a partial final record")
	}
	if !turn.Complete {
		v.issue(id, "incomplete", "native successful terminal boundary missing")
	}
	v.result.Coverage["observed_model"] = "unavailable"
	if turn.Model != "" {
		v.result.Coverage["observed_model"] = turn.Model
	}
	v.result.Coverage["requested_model"] = "unavailable"
	if r.RequestedModel != "" {
		v.result.Coverage["requested_model"] = r.RequestedModel
	}
	v.result.Coverage["requested_effort"] = "unavailable"
	if r.RequestedEffort != "" {
		v.result.Coverage["requested_effort"] = r.RequestedEffort
	}
	if turn.Witness {
		for _, token := range strings.FieldsFunc(turn.Output, func(r rune) bool { return !((r >= 'a' && r <= 'f') || (r >= '0' && r <= '9')) }) {
			if token == c.Commit {
				v.result.Coverage["session_code"] = "native_tool_result_commit"
				return
			}
			if token == c.Tree {
				v.result.Coverage["session_code"] = "native_tool_result_tree"
				return
			}
		}
	}
	v.issue(id, "incomplete", "native tool result does not witness code identity in selected turn")
}
