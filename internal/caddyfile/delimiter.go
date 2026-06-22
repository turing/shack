package caddyfile

import (
	"strings"
	"unicode/utf8"
)

const (
	LineWidth    = 80
	openerPrefix = "# ── shack "
	closerPrefix = "# ── /shack "
	fillRune     = '─'
)

func OpenerLine() string { return fillTo(openerPrefix) }
func CloserLine() string { return fillTo(closerPrefix) }

func IsOpener(line string) bool { return strings.HasPrefix(line, openerPrefix) }
func IsCloser(line string) bool { return strings.HasPrefix(line, closerPrefix) }

func fillTo(prefix string) string {
	have := utf8.RuneCountInString(prefix)
	if have >= LineWidth {
		return prefix
	}
	var b strings.Builder
	b.WriteString(prefix)
	for i := 0; i < LineWidth-have; i++ {
		b.WriteRune(fillRune)
	}
	return b.String()
}
