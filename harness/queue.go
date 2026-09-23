package harness

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/adambiggs/gangline/substrate"
)

// NativeQueueAccepted identifies queue ownership, not full-text submission.
// Previews may truncate the body; the entire one-time envelope opener must fit.
func NativeQueueAccepted(c Collar, screen substrate.Screen, opener string) (bool, error) {
	if c.Primitives.QueueWitness == nil {
		return false, nil
	}
	if c.Primitives.QueueWitness.Name != "codex-pending-input" {
		return false, fmt.Errorf("unknown queue witness %q", c.Primitives.QueueWitness.Name)
	}
	if !strings.HasPrefix(opener, "[gang:") || !strings.HasSuffix(opener, "]") {
		return false, fmt.Errorf("queue witness requires a complete envelope opener")
	}
	if _, blocked, err := InputBlocked(c, screen); err != nil || blocked {
		return false, err
	}
	composer, err := ReadComposer(c.Primitives.Composer, screen)
	if err != nil || composer.Text != "" {
		return false, nil
	}
	lines := screenLines(screen, true)
	end := -1
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.HasPrefix(lines[i], "›") {
			end = i
			break
		}
	}
	if end < 0 {
		return false, nil
	}
	// Only accept the structured preview immediately above the composer. Stop
	// at any unrelated content rather than searching the conversation history.
	start := end
	for start > 0 {
		line := lines[start-1]
		if strings.TrimSpace(line) == "" || queueHeader(line) || strings.HasPrefix(line, "  ↳ ") || strings.HasPrefix(line, "    ") || strings.HasPrefix(line, "  (press ") {
			start--
			continue
		}
		break
	}
	header := false
	for i := start; i < end; i++ {
		if queueHeader(lines[i]) {
			header = true
			continue
		}
		if !header || !strings.HasPrefix(lines[i], "  ↳ ") {
			continue
		}
		text := strings.TrimPrefix(lines[i], "  ↳ ")
		dim := dimContent(screen.Rows[i])
		for i+1 < end && strings.HasPrefix(lines[i+1], "    ") {
			// Wrapping can split an opener. Only the opener is used as proof;
			// preview truncation before its closing bracket does not qualify.
			if strings.HasPrefix(text, opener) {
				break
			}
			i++
			text += strings.TrimPrefix(lines[i], "    ")
			dim = dim && dimContent(screen.Rows[i])
		}
		if dim && strings.HasPrefix(text, opener) {
			return true, nil
		}
	}
	return false, nil
}

func queueHeader(line string) bool {
	switch strings.TrimSpace(line) {
	case "• Messages to be submitted after next tool call", "• Messages to be submitted at end of turn", "• Queued follow-up inputs":
		return true
	}
	// Wide terminals keep this hint on the header line.
	return strings.HasPrefix(strings.TrimSpace(line), "• Messages to be submitted after next tool call (press ") && strings.HasSuffix(strings.TrimSpace(line), " to interrupt and send immediately)")
}

func dimContent(row []substrate.Cell) bool {
	for _, cell := range row {
		if strings.TrimFunc(cell.Text, unicode.IsSpace) != "" && !cell.Attributes.Dim {
			return false
		}
	}
	return true
}
