package caddyfile

import (
	"reflect"
	"testing"
)

func TestParseEmptyString(t *testing.T) {
	doc, err := Parse("")
	if err != nil {
		t.Fatalf("Parse(\"\") error: %v", err)
	}
	if doc.Above != "" || doc.Below != "" {
		t.Errorf("expected empty above/below, got above=%q below=%q", doc.Above, doc.Below)
	}
	if len(doc.Entries) != 0 {
		t.Errorf("expected no entries, got %d", len(doc.Entries))
	}
	if doc.HasManagedRegion {
		t.Errorf("expected HasManagedRegion=false")
	}
}

func TestParseNoMarkers(t *testing.T) {
	input := "foo.localhost {\n\treverse_proxy localhost:9000\n}\n"
	doc, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if doc.Above != input {
		t.Errorf("Above = %q, want %q", doc.Above, input)
	}
	if doc.HasManagedRegion {
		t.Errorf("expected HasManagedRegion=false (no markers in input)")
	}
}

func TestParseEmptyManagedRegion(t *testing.T) {
	input := OpenerLine() + "\n\n" + CloserLine() + "\n"
	doc, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if !doc.HasManagedRegion {
		t.Errorf("expected HasManagedRegion=true")
	}
	if len(doc.Entries) != 0 {
		t.Errorf("expected no entries, got %d", len(doc.Entries))
	}
}

func TestParseEntriesInRegion(t *testing.T) {
	input := OpenerLine() + "\n\n" +
		"foo.localhost {\n" +
		"    reverse_proxy localhost:3000\n" +
		"}\n\n" +
		"bar.localhost {\n" +
		"    reverse_proxy localhost:8080\n" +
		"}\n\n" +
		CloserLine() + "\n"
	doc, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	want := []Entry{{Name: "foo", Port: 3000}, {Name: "bar", Port: 8080}}
	if !reflect.DeepEqual(doc.Entries, want) {
		t.Errorf("Entries = %+v, want %+v", doc.Entries, want)
	}
}

func TestParsePreservesAboveAndBelow(t *testing.T) {
	above := "# personal stuff\nlocalhost:9999 {\n\trespond \"hi\"\n}\n\n"
	below := "\n# also personal\n"
	managed := OpenerLine() + "\n\n" +
		"foo.localhost {\n    reverse_proxy localhost:3000\n}\n\n" +
		CloserLine() + "\n"
	input := above + managed + below
	doc, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if doc.Above != above {
		t.Errorf("Above = %q, want %q", doc.Above, above)
	}
	if doc.Below != below {
		t.Errorf("Below = %q, want %q", doc.Below, below)
	}
	if len(doc.Entries) != 1 || doc.Entries[0].Name != "foo" {
		t.Errorf("Entries = %+v, want one foo entry", doc.Entries)
	}
}

func TestParseDropsUnparseableLinesInRegion(t *testing.T) {
	input := OpenerLine() + "\n" +
		"foo.localhost {\n    reverse_proxy localhost:3000\n}\n" +
		"# stray comment user added\n" +
		"random garbage\n" +
		CloserLine() + "\n"
	doc, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if len(doc.Entries) != 1 || doc.Entries[0] != (Entry{Name: "foo", Port: 3000}) {
		t.Errorf("Entries = %+v, want one foo entry; junk should be dropped", doc.Entries)
	}
}
