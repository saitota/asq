package ansitext

import (
	"strings"
	"testing"
)

func TestStrip(t *testing.T) {
	got := Strip("\x1b[38;2;245;158;11mfoo\x1b[0mbar")
	if got != "foobar" {
		t.Fatalf("Strip() = %q, want %q", got, "foobar")
	}
}

func TestTruncateEndSkipsANSISequences(t *testing.T) {
	colored := "\x1b[38;2;245;158;11m●\x1b[0m  07-06 18:13  fix-flaky"
	got := TruncateEnd(colored, 10)
	if Width(got) != 10 {
		t.Fatalf("visible width = %d, want 10: %q", Width(got), got)
	}
	if !strings.Contains(got, "\x1b[38;2;245;158;11m●\x1b[0m") {
		t.Fatalf("color sequence was split or dropped: %q", got)
	}
	if !strings.Contains(got, "\x1b[0m...") {
		t.Fatalf("truncated styled text must reset before the ellipsis: %q", got)
	}
	if !strings.Contains(got, "07-0") {
		t.Fatalf("escape bytes were counted as width: %q", got)
	}
}

func TestCutDisplaySkipsANSISequences(t *testing.T) {
	colored := "\x1b[38;2;245;158;11m●\x1b[0m  07-06 18:13  fix-flaky"
	got := CutDisplay(colored, 10)
	if Width(got) != 10 {
		t.Fatalf("visible width = %d, want 10: %q", Width(got), got)
	}
	if !strings.Contains(got, "\x1b[38;2;245;158;11m●\x1b[0m") {
		t.Fatalf("color sequence was split or dropped: %q", got)
	}
	if !strings.HasSuffix(got, "\x1b[0m") {
		t.Fatalf("truncated styled text must end with a reset: %q", got)
	}
	if !strings.Contains(got, "07-06") {
		t.Fatalf("escape bytes were counted as width: %q", got)
	}
}

func TestCutDisplayNeverCutsMidSequence(t *testing.T) {
	value := "abc\x1b[1mBold\x1b[22mrest of the line"
	for width := 1; width <= 12; width++ {
		got := CutDisplay(value, width)
		stripped := got
		for _, seq := range []string{"\x1b[1m", "\x1b[22m", "\x1b[0m"} {
			stripped = strings.ReplaceAll(stripped, seq, "")
		}
		if strings.Contains(stripped, "\x1b") {
			t.Fatalf("width %d split an escape sequence: %q", width, got)
		}
	}
}

func TestTruncateEndNeverCutsMidSequence(t *testing.T) {
	value := "abc\x1b[1mBold\x1b[22mrest of the line"
	for width := 1; width <= 12; width++ {
		got := TruncateEnd(value, width)
		stripped := got
		for _, seq := range []string{"\x1b[1m", "\x1b[22m", "\x1b[0m"} {
			stripped = strings.ReplaceAll(stripped, seq, "")
		}
		if strings.Contains(stripped, "\x1b") {
			t.Fatalf("width %d split an escape sequence: %q", width, got)
		}
	}
}

func TestTruncateMiddleKeepsPrefixAndSuffix(t *testing.T) {
	got := TruncateMiddle("/Users/you/work/some/very/long/path/project", 20)
	if Width(got) > 20 {
		t.Fatalf("visible width = %d, want <= 20: %q", Width(got), got)
	}
	if !strings.HasPrefix(got, "/Users") {
		t.Fatalf("expected prefix preserved: %q", got)
	}
	if !strings.HasSuffix(got, "project") {
		t.Fatalf("expected suffix preserved: %q", got)
	}
}

func TestPadMeasuresDisplayWidth(t *testing.T) {
	colored := "\x1b[1mhi\x1b[0m"
	got := Pad(colored, 5)
	if Width(got) != 5 {
		t.Fatalf("padded width = %d, want 5: %q", Width(got), got)
	}
}

func TestWrapWrapsWideRunes(t *testing.T) {
	lines := Wrap("日本語のテキストです", 6)
	for _, line := range lines {
		if Width(line) > 6 {
			t.Fatalf("wrapped line too wide: %q", line)
		}
	}
	if len(lines) < 3 {
		t.Fatalf("expected multiple wrapped lines, got %v", lines)
	}
}

func TestWrapCarriesStyleAcrossWrappedLines(t *testing.T) {
	text := "\x1b[38;2;245;158;11massistant\x1b[0m plain tail here"
	lines := Wrap(text, 6)
	for i, line := range lines {
		if Width(line) > 6 {
			t.Fatalf("line %d too wide: %q", i, line)
		}
		open := strings.Count(line, "\x1b[38;2;245;158;11m")
		if open > 0 && !strings.Contains(line, "\x1b[0m") {
			t.Fatalf("line %d leaves color open: %q", i, line)
		}
	}
	if !strings.HasPrefix(lines[1], "\x1b[38;2;245;158;11m") {
		t.Fatalf("continuation line should reopen the color: %q", lines[1])
	}
}

func TestWrapCarriesMultipleLayeredStyles(t *testing.T) {
	// Two SGR sequences active at once (bold, then a color) must both
	// survive a wrap point instead of only the most recent one.
	text := "\x1b[1m\x1b[38;2;10;20;30mboldcolored\x1b[0m tail"
	lines := Wrap(text, 5)
	if len(lines) < 2 {
		t.Fatalf("expected wrap to produce multiple lines: %v", lines)
	}
	if !strings.Contains(lines[0], "\x1b[0m") {
		t.Fatalf("line 0 should close both styles at the wrap point: %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "\x1b[1m\x1b[38;2;10;20;30m") {
		t.Fatalf("continuation line should reopen both styles in order: %q", lines[1])
	}
}

func TestSequenceLenMatchesCSISequenceOnly(t *testing.T) {
	if got := SequenceLen("\x1b[38;2;1;2;3m"); got != len("\x1b[38;2;1;2;3m") {
		t.Fatalf("SequenceLen = %d, want %d", got, len("\x1b[38;2;1;2;3m"))
	}
	if got := SequenceLen("not an escape"); got != 0 {
		t.Fatalf("SequenceLen(plain text) = %d, want 0", got)
	}
}
