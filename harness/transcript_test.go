package harness

import (
	"strings"
	"testing"
	"time"
)

type countingTranscript struct {
	*strings.Reader
	read int
}

func (c *countingTranscript) Read(p []byte) (int, error) {
	n, err := c.Reader.Read(p)
	c.read += n
	return n, err
}
func TestNativeTranscriptWorkIsBounded(t *testing.T) {
	header := "{\"type\":\"session_meta\",\"payload\":{\"id\":\"s\"}}\n"
	line := "{\"type\":\"response_item\",\"payload\":{\"text\":\"" + strings.Repeat("x", 4096) + "\"}}\n"
	input := &countingTranscript{Reader: strings.NewReader(header + strings.Repeat(line, transcriptWindow/len(line)*3))}
	if _, err := ReadTranscript(Invocation{Name: "codex-session-log"}, input, "s", 0, time.Time{}); err != nil {
		t.Fatal(err)
	}
	// Metadata and the boundary line add a fixed overhead to the bounded tail.
	if input.read > transcriptWindow+16384 {
		t.Fatalf("native read grew with history: %d bytes", input.read)
	}
}
