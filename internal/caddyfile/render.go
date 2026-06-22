package caddyfile

import (
	"fmt"
	"sort"
	"strings"
)

func (d Document) Render() string {
	var b strings.Builder
	b.WriteString(d.Above)

	if d.HasManagedRegion {
		// When user content sits above the managed region, separate it from
		// the opener with a blank line for readability. (Per spec.)
		if d.Above != "" {
			if !strings.HasSuffix(d.Above, "\n") {
				b.WriteString("\n")
			}
			if !strings.HasSuffix(d.Above, "\n\n") {
				b.WriteString("\n")
			}
		}
		b.WriteString(OpenerLine())
		b.WriteString("\n\n")

		entries := append([]Entry(nil), d.Entries...)
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
		for _, e := range entries {
			fmt.Fprintf(&b, "%s.localhost {\n    reverse_proxy localhost:%d\n}\n\n", e.Name, e.Port)
		}

		b.WriteString(CloserLine())
		b.WriteString("\n")
	}

	b.WriteString(d.Below)
	return b.String()
}
