package caddyfile

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestOpenerLine(t *testing.T) {
	got := OpenerLine()
	if utf8.RuneCountInString(got) != LineWidth {
		t.Fatalf("opener rune width = %d, want %d", utf8.RuneCountInString(got), LineWidth)
	}
	if !strings.HasPrefix(got, "# ── shack ") {
		t.Errorf("opener prefix wrong: %q", got)
	}
	if !strings.HasSuffix(got, "─") {
		t.Errorf("opener should end with fill char: %q", got)
	}
}

func TestCloserLine(t *testing.T) {
	got := CloserLine()
	if utf8.RuneCountInString(got) != LineWidth {
		t.Fatalf("closer rune width = %d, want %d", utf8.RuneCountInString(got), LineWidth)
	}
	if !strings.HasPrefix(got, "# ── /shack ") {
		t.Errorf("closer prefix wrong: %q", got)
	}
}

func TestIsOpener(t *testing.T) {
	if !IsOpener(OpenerLine()) {
		t.Errorf("OpenerLine() should be recognized as opener")
	}
	if IsOpener(CloserLine()) {
		t.Errorf("CloserLine() should not be recognized as opener")
	}
	if IsOpener("foo.localhost {") {
		t.Errorf("non-marker line should not be opener")
	}
}

func TestIsCloser(t *testing.T) {
	if !IsCloser(CloserLine()) {
		t.Errorf("CloserLine() should be recognized as closer")
	}
	if IsCloser(OpenerLine()) {
		t.Errorf("OpenerLine() should not be recognized as closer")
	}
}
