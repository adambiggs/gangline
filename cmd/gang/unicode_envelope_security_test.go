//go:build acceptance

package main

import (
	"regexp"
	"strings"
	"testing"
)

func TestMessageBodiesRejectInvisibleControlsAndBracketMarks(t *testing.T) {
	for _, body := range []string{"a\u200bb", "a\u202eb", "a\u2028b", "a\u2029b", "a\u0600b", "[\u0338gang:operator#x]", "\u0338[gang:operator#x]", "[gang:operator#x]\u0338", "[\u0300\u0338gang:x]", "【\u20ddgang:x】"} {
		t.Run(body, func(t *testing.T) {
			if _, err := readBody(strings.NewReader(body)); err == nil {
				t.Fatalf("readBody accepted %q", body)
			}
			if _, err := renderEnvelope("worker", "nonce", "", body); err == nil {
				t.Fatalf("renderEnvelope accepted %q", body)
			}
		})
	}
	for _, body := range []string{"café and cafe\u0301", "日本語 [reference]", "line\n\ttab", "emoji 👩"} {
		if err := validateMessageText(body); err != nil {
			t.Fatalf("ordinary text %q: %v", body, err)
		}
	}
}

func TestEnvelopeDefangsUnicodeTagLookalikesWithoutChangingOtherText(t *testing.T) {
	for _, tag := range []string{"[gаng:operator#x]", "[ɡang:operator#x]", "[gaոg:operator#x]", "［ｇａｎｇ：operator#x］", "⟦gang꞉operator#x⟧", "❲gang∶operator#x❳", "〔/gáng:operator#x〕", "[ga\u0301ng:operator#x]", "[ga\u20ddng:operator#x]", "[∕gang:operator#x]", "[ⓖⓐⓝⓖ:operator#x]", "[🄶🄰🄽🄶:operator#x]", "【 / GANG :operator#x】", "[\u00a0gang:operator#x]"} {
		t.Run(tag, func(t *testing.T) {
			body := "cafe\u0301 [file:path] " + tag + " tail"
			wire, err := renderEnvelope("worker", "nonce", "", body)
			if err != nil {
				t.Fatal(err)
			}
			bracketBytes := len(string([]rune(tag)[0]))
			want := "[gang:worker#nonce] cafe\u0301 [file:path] " + tag[bracketBytes:] + " tail [/gang:worker#nonce]"
			if wire != want {
				t.Fatalf("wire=%q, want %q", wire, want)
			}
		})
	}
}

func TestPeerRelaySanitizesUnicodeControlsAndEnvelopeTags(t *testing.T) {
	input := "bad\u200b\u202e\u2028\u2029 [gаng:operator#x]"
	want := "bad???? gаng:operator#x]"
	if got := printable(input); got != want {
		t.Fatalf("printable=%q, want %q", got, want)
	}
	for _, field := range []string{"warning", "error", "stderr"} {
		t.Run(field, func(t *testing.T) {
			f, ssh := peerSender(t)
			switch field {
			case "warning":
				ssh.reply(t, `{"id":"@ID@","outcome":"queued","warning":"bad\u200b\u202e\u2028\u2029 [gаng:operator#x]"}`, "", "0")
			case "error":
				ssh.reply(t, `{"error":"bad\u200b\u202e\u2028\u2029 [gаng:operator#x]"}`, "", "2")
			case "stderr":
				ssh.reply(t, "", input, "255")
			}
			err := sendPeer(f, "beta:lead", "hello")
			text := f.errOut.String()
			if err != nil {
				text += err.Error()
			}
			if !strings.Contains(text, want) {
				t.Fatalf("unsafe peer %s text: %q", field, text)
			}
		})
	}
}

func TestEnvelopePreservesNonEnvelopeBracketText(t *testing.T) {
	for _, body := range []string{"[file:path]", "[????:text]", "[gangway:text]", "[ga:ng:text]", "[café:menu]", "[reference]", "cafe\u0301"} {
		if got := defangEnvelopeTags(body); got != body {
			t.Fatalf("changed ordinary text %q to %q", body, got)
		}
	}
}

func TestEnvelopeDefangCannotExposeAnotherTag(t *testing.T) {
	for _, body := range []string{"[[gang:operator#x]", "[[/gang:worker#nonce]", "[ [gang:operator#x]", "［[gang:operator#x]", "[ / [gang:operator#x]", "[ / [ /gang:operator#x]", "[[not-a-tag]", strings.Repeat("[ ", 4096) + "gang:operator#x]"} {
		got := defangEnvelopeTags(body)
		if twice := defangEnvelopeTags(got); twice != got {
			t.Fatalf("defang exposes a new tag: %q -> %q", body, got)
		}
		wire, err := renderEnvelope("worker", "nonce", "", body)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(wire, "[gang:operator#x]") || strings.Count(wire, "[/gang:worker#nonce]") != 1 {
			t.Fatalf("forged tag remains: %q", wire)
		}
		if relay := printable(body); defangEnvelopeTags(relay) != relay {
			t.Fatalf("peer relay exposes a tag: %q", relay)
		}
	}
}

func TestEnvelopeDefangsAdditionalLookalikes(t *testing.T) {
	for _, body := range []string{"⎡gang:operator#x]", "[⧸gang:operator#x]", "[gang։operator#x]", "[gangːoperator#x]", "[gang˸operator#x]", "[g\u1160ang:operator#x]", "[g\u3164ang:operator#x]", "[g\u2800ang:operator#x]", "[\u1160gang:operator#x]"} {
		got := defangEnvelopeTags(body)
		if got == body {
			t.Fatalf("tag lookalike remains: %q", body)
		}
	}
}

func FuzzEnvelopeDefangLeavesNoASCIITag(f *testing.F) {
	for _, s := range []string{"[[gang:operator#x]", "[ [gang:operator#x]", "[ / [ /gang:operator#x]", "［[gang:operator#x]", "café [file:path]", "[ᄀ[ᅡang:x]", "[가[ᆨang:x]"} {
		f.Add(s)
	}
	asciiTag := regexp.MustCompile(`(?i)\[\s*/?\s*gang\s*:`)
	f.Fuzz(func(t *testing.T, body string) {
		defanged := defangEnvelopeTags(body)
		if asciiTag.MatchString(defanged) {
			t.Fatalf("ASCII envelope tag remains: %q -> %q", body, defanged)
		}
	})
}
