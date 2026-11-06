package harness

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// HookEvent is a decoded native callback. Absent holds mapped payload targets
// that the callback left missing, null or empty.
type HookEvent struct {
	NativeEvent string
	Kind        string
	Payload     map[string]string
	Absent      map[string]bool
}

type TurnBoundary string

const (
	TurnStarted            TurnBoundary = "turn-started"
	TurnFinished           TurnBoundary = "turn-finished"
	TurnFailed             TurnBoundary = "turn-failed"
	TurnCompactionStarted  TurnBoundary = "compaction-started"
	TurnCompactionFinished TurnBoundary = "compaction-finished"
)

// OpenTurnQuiet is how long an idle-looking screen must stay unchanged before
// a submitted turn with no finish boundary counts as over, as after Escape or
// a turn that dies without its Stop hook. Zero leaves turn state to the screen.
func OpenTurnQuiet(invocation Invocation) (time.Duration, error) {
	if invocation.Name != "hook-boundary" {
		return 0, fmt.Errorf("unknown turn-boundary primitive %q", invocation.Name)
	}
	value := invocation.Params["open_turn_quiet"]
	if value == "" {
		return 0, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return 0, fmt.Errorf("turn boundary open_turn_quiet %q is not a positive duration", value)
	}
	return duration, nil
}

func DetectTurnBoundary(collar Collar, data []byte) (TurnBoundary, HookEvent, error) {
	if collar.Primitives.TurnBoundary.Name != "hook-boundary" {
		return "", HookEvent{}, fmt.Errorf("unknown turn-boundary primitive %q", collar.Primitives.TurnBoundary.Name)
	}
	event, err := DecodeHook(collar, data)
	if err != nil {
		return "", HookEvent{}, err
	}
	switch TurnBoundary(event.Kind) {
	case TurnStarted, TurnFinished, TurnFailed, TurnCompactionStarted, TurnCompactionFinished:
		return TurnBoundary(event.Kind), event, nil
	default:
		return "", event, nil
	}
}

func DecodeHook(collar Collar, data []byte) (HookEvent, error) {
	if collar.Hooks == nil {
		return HookEvent{}, fmt.Errorf("collar %q declares no hooks", collar.Name)
	}
	var body map[string]any
	if err := json.Unmarshal(data, &body); err != nil {
		return HookEvent{}, fmt.Errorf("decode hook payload: %w", err)
	}
	native, ok := stringField(body, "hook_event_name")
	if !ok {
		native, ok = stringField(body, "event")
	}
	if !ok || native == "" {
		return HookEvent{}, errors.New("hook payload carries no event name")
	}
	wiring, ok := collar.Hooks.Events[normalizeEventName(native)]
	if !ok {
		return HookEvent{}, fmt.Errorf("collar %q does not map native hook event %q", collar.Name, native)
	}
	payload := make(map[string]string, len(wiring.Payload))
	var absent map[string]bool
	for target, source := range wiring.Payload {
		value, ok := nestedString(body, strings.Split(source, "."))
		if ok {
			payload[target] = value
		}
		if value == "" {
			if absent == nil {
				absent = make(map[string]bool, len(wiring.Payload))
			}
			absent[target] = true
		}
	}
	return HookEvent{NativeEvent: native, Kind: wiring.Event, Payload: payload, Absent: absent}, nil
}

// SubmittedPromptMatches compares the text Gangline sent with the prompt a
// native submit hook observed. Claude Code wraps a long bracketed paste in a
// pasted_content element before exposing it to UserPromptSubmit; the wrapper
// identifier is harness-owned, but the pasted bytes remain authoritative,
// wrapped or not, apart from the escape described at consumePastedBody.
func SubmittedPromptMatches(primitive Invocation, sent, witnessed string) (bool, error) {
	switch primitive.Name {
	case "exact-prompt":
		return witnessed == sent, nil
	case "claude-pasted-content":
		if rest, ok := consumePastedBody(witnessed, sent); ok && rest == "" {
			return true, nil
		}
		framed := witnessed
		if strings.HasPrefix(framed, "\n\n") {
			framed = strings.TrimPrefix(framed, "\n\n")
		} else {
			framed = strings.TrimPrefix(framed, "\n")
		}
		const prefix = "<pasted_content id=\""
		if !strings.HasPrefix(framed, prefix) {
			return false, nil
		}
		identifier, wrapped, found := strings.Cut(strings.TrimPrefix(framed, prefix), "\">\n")
		if !found || identifier == "" || strings.ContainsAny(identifier, "\"\r\n<>") {
			return false, nil
		}
		suffix := "\n</pasted_content id=\"" + identifier + "\">"
		wrapped = strings.TrimSuffix(wrapped, "\n")
		if !strings.HasSuffix(wrapped, suffix) {
			return false, nil
		}
		rest, ok := consumePastedBody(strings.TrimSuffix(wrapped, suffix), sent)
		return ok && rest == "", nil
	default:
		return false, fmt.Errorf("unknown submit-witness primitive %q", primitive.Name)
	}
}

// pastePlaceholder is the reference pattern Claude Code expands in a
// submitted prompt: a bracketed label for pasted text, an image, or truncated
// text, a paste number, an optional line count, and any trailing dots. Claude
// Code substitutes any such token whose number names a live text paste,
// whatever its label.
var pastePlaceholder = regexp.MustCompile(`\[(?:Pasted text|Image|\.\.\.Truncated text) #\d+(?: \+\d+ lines)?\.*\]`)

// heldByClaudePaste reports a character that Claude Code drops from a paste,
// or turns into a newline, and whose paste the next Enter does not submit: a
// format character other than a prepended concatenation mark, or a Unicode
// line or paragraph separator.
func heldByClaudePaste(r rune) bool {
	return r == '\u2028' || r == '\u2029' || unicode.Is(unicode.Cf, r) && !unicode.Is(unicode.Prepended_Concatenation_Mark, r)
}

// PasteHazard returns why text cannot be typed into a harness unchanged.
// Claude Code replaces a placeholder token with the content of a live earlier
// paste carrying that number, and a paste stays live after its
// composer is cleared. Gangline cannot see which numbers are live, so every
// such token is refused.
func PasteHazard(primitive Invocation, text string) (string, error) {
	switch primitive.Name {
	case "exact-prompt":
		return "", nil
	case "claude-pasted-content":
		if pastePlaceholder.MatchString(text) {
			return "text contains a numbered paste placeholder in brackets, for pasted text, an image, or truncated text; Claude Code can replace it with the content of an earlier paste, so describe it in words", nil
		}
		var held []string
		for _, r := range text {
			if name := fmt.Sprintf("U+%04X", r); heldByClaudePaste(r) && !slices.Contains(held, name) {
				held = append(held, name)
			}
		}
		if len(held) != 0 {
			return "text contains " + strings.Join(held, ", ") + ", which Claude Code drops or turns into a newline, and it then leaves the paste unsubmitted; remove these invisible format characters or line and paragraph separators", nil
		}
		return "", nil
	default:
		return "", fmt.Errorf("unknown submit-witness primitive %q", primitive.Name)
	}
}

// submitWitnessPrimitives names every submit-witness primitive a collar may use.
var submitWitnessPrimitives = []string{"exact-prompt", "claude-pasted-content"}

// AnyPasteHazard returns why text cannot be typed unchanged into every
// harness, for text whose recipient is chosen only when it is delivered.
func AnyPasteHazard(text string) (string, error) {
	for _, name := range submitWitnessPrimitives {
		if reason, err := PasteHazard(Invocation{Name: name}, text); err != nil || reason != "" {
			return reason, err
		}
	}
	return "", nil
}

// SubmittedPromptStartsWith compares the queued envelope alone. Bytes after
// its closing tag are session keyboard input, including without a newline.
func SubmittedPromptStartsWith(primitive Invocation, sent, witnessed string) (bool, error) {
	matched, _, err := SubmittedEnvelopeMatches(primitive, sent, witnessed)
	return matched, err
}

// SubmittedEnvelopeMatches also returns the bytes outside the queued envelope,
// excluding native paste framing. The entire rendered envelope must match;
// this includes all sections of a startup paste.
func SubmittedEnvelopeMatches(primitive Invocation, sent, witnessed string) (bool, string, error) {
	switch primitive.Name {
	case "exact-prompt":
		rest, ok := strings.CutPrefix(witnessed, sent)
		return ok, rest, nil
	case "claude-pasted-content":
		// Claude Code turns each tab of a paste into four spaces, whatever
		// its column; the paste matches as sent or so rewritten. Its witness
		// can also drop generated line breaks, so the context stamp line may
		// arrive joined to the closing tag before it.
		variants := []string{sent}
		if joined, ok := joinStampLine(sent); ok {
			variants = append(variants, joined)
		}
		for _, variant := range variants {
			for _, text := range []string{variant, strings.ReplaceAll(variant, "\t", "    ")} {
				if rest, ok := matchClaudePaste(text, witnessed); ok {
					return true, rest, nil
				}
			}
		}
		return false, "", nil
	default:
		return false, "", fmt.Errorf("unknown submit-witness primitive %q", primitive.Name)
	}
}

// joinStampLine removes the line break before a trailing "[gang ... /]"
// context stamp line, which Gangline renders after an envelope's closing tag.
func joinStampLine(sent string) (string, bool) {
	i := strings.LastIndex(sent, "\n[gang ")
	if i < 0 || !strings.HasSuffix(sent, " /]") || strings.Contains(sent[i+1:], "\n") || !strings.HasSuffix(sent[:i], "]") {
		return "", false
	}
	return sent[:i] + sent[i+1:], true
}

// matchClaudePaste matches sent against a prompt Claude Code reports for a
// paste, wrapped in its pasted_content element or not, and returns the bytes
// outside the paste.
func matchClaudePaste(sent, witnessed string) (string, bool) {
	if rest, ok := consumePastedBody(witnessed, sent); ok {
		return rest, true
	}
	framed := witnessed
	if strings.HasPrefix(framed, "\n\n") {
		framed = strings.TrimPrefix(framed, "\n\n")
	} else {
		framed = strings.TrimPrefix(framed, "\n")
	}
	const prefix = "<pasted_content id=\""
	if !strings.HasPrefix(framed, prefix) {
		return "", false
	}
	identifier, wrapped, found := strings.Cut(strings.TrimPrefix(framed, prefix), "\">\n")
	if !found || identifier == "" || strings.ContainsAny(identifier, "\"\r\n<>") {
		return "", false
	}
	suffix := "\n</pasted_content id=\"" + identifier + "\">"
	rest, ok := consumePastedBody(wrapped, sent)
	if !ok {
		return "", false
	}
	inside, after, closed := strings.Cut(rest, suffix)
	if !closed {
		return "", false
	}
	// Claude's one trailing newline belongs to the paste wrapper.
	after = strings.TrimPrefix(after, "\n")
	return inside + after, true
}

// consumePastedBody matches sent against the start of pasted text as Claude
// Code reports it and returns the rest. Claude Code escapes pasted text that
// could read as its own wrapper tag, whether or not it wraps the paste: it
// replaces the tag's opener, an ASCII '<' or a non-ASCII lookalike, with an
// ASCII `<\`. That pair may stand for one such sent opener; every other
// byte must match.
func consumePastedBody(body, sent string) (string, bool) {
	for sent != "" {
		r, size := utf8.DecodeRuneInString(sent)
		if strings.HasPrefix(body, `<\`) && !strings.HasPrefix(sent, `<\`) && (r == '<' || r >= utf8.RuneSelf) {
			body, sent = body[2:], sent[size:]
			continue
		}
		if !strings.HasPrefix(body, sent[:size]) {
			return "", false
		}
		body, sent = body[size:], sent[size:]
	}
	return body, true
}

func normalizeEventName(name string) string {
	return strings.Map(func(char rune) rune {
		if char == '-' || char == '_' || char == ' ' {
			return -1
		}
		return char
	}, strings.ToLower(name))
}

func stringField(body map[string]any, field string) (string, bool) {
	value, ok := body[field].(string)
	return value, ok
}

func nestedString(value any, path []string) (string, bool) {
	value, ok := nestedValue(value, path)
	if !ok {
		return "", false
	}
	result, ok := value.(string)
	return result, ok
}

func nestedValue(value any, path []string) (any, bool) {
	for len(path) > 1 {
		object, ok := value.(map[string]any)
		if !ok {
			return nil, false
		}
		value, ok = object[path[0]]
		if !ok {
			return nil, false
		}
		path = path[1:]
	}
	object, ok := value.(map[string]any)
	if !ok || len(path) == 0 {
		return nil, false
	}
	result, ok := object[path[0]]
	return result, ok
}
