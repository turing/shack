package caddyfile

import (
	"regexp"
	"strconv"
	"strings"
)

var entryRegex = regexp.MustCompile(`^([a-z0-9][a-z0-9-]*)\.localhost\s*\{\s*$`)
var proxyRegex = regexp.MustCompile(`^\s*reverse_proxy\s+localhost:(\d+)\s*$`)

func Parse(content string) (Document, error) {
	doc := Document{}
	if content == "" {
		return doc, nil
	}

	lines := strings.SplitAfter(content, "\n")

	openerIdx, closerIdx := -1, -1
	for i, line := range lines {
		trimmed := strings.TrimRight(line, "\n")
		if openerIdx == -1 && IsOpener(trimmed) {
			openerIdx = i
			continue
		}
		if openerIdx != -1 && IsCloser(trimmed) {
			closerIdx = i
			break
		}
	}

	if openerIdx == -1 || closerIdx == -1 {
		doc.Above = content
		return doc, nil
	}

	doc.HasManagedRegion = true
	doc.Above = strings.Join(lines[:openerIdx], "")
	doc.Below = strings.Join(lines[closerIdx+1:], "")
	doc.Entries = parseEntries(lines[openerIdx+1 : closerIdx])
	return doc, nil
}

func parseEntries(lines []string) []Entry {
	var entries []Entry
	i := 0
	for i < len(lines) {
		trimmed := strings.TrimRight(lines[i], "\n")
		m := entryRegex.FindStringSubmatch(trimmed)
		if m == nil {
			i++
			continue
		}
		name := m[1]
		port := 0
		// Look ahead up to a few lines for the proxy directive and the closing brace.
		closed := false
		for j := i + 1; j < len(lines) && j < i+10; j++ {
			line := strings.TrimRight(lines[j], "\n")
			if pm := proxyRegex.FindStringSubmatch(line); pm != nil {
				p, err := strconv.Atoi(pm[1])
				if err == nil {
					port = p
				}
				continue
			}
			if strings.TrimSpace(line) == "}" {
				closed = true
				i = j + 1
				break
			}
		}
		if closed && port > 0 {
			entries = append(entries, Entry{Name: name, Port: port})
		} else {
			i++
		}
	}
	return entries
}
