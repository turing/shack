package caddyfile

import (
	"strings"
	"testing"
)

func TestRenderEmptyManagedRegion(t *testing.T) {
	doc := Document{HasManagedRegion: true}
	got := doc.Render()
	want := OpenerLine() + "\n\n" + CloserLine() + "\n"
	if got != want {
		t.Errorf("Render() empty managed region:\n got: %q\nwant: %q", got, want)
	}
}

func TestRenderEntriesAreSorted(t *testing.T) {
	doc := Document{
		HasManagedRegion: true,
		Entries: []Entry{
			{Name: "zeta", Port: 9000},
			{Name: "alpha", Port: 1000},
			{Name: "mike", Port: 5000},
		},
	}
	got := doc.Render()
	alphaIdx := strings.Index(got, "alpha.localhost")
	mikeIdx := strings.Index(got, "mike.localhost")
	zetaIdx := strings.Index(got, "zeta.localhost")
	if !(alphaIdx < mikeIdx && mikeIdx < zetaIdx) {
		t.Errorf("entries not sorted: indices alpha=%d mike=%d zeta=%d in:\n%s", alphaIdx, mikeIdx, zetaIdx, got)
	}
}

func TestRenderEntryShape(t *testing.T) {
	doc := Document{
		HasManagedRegion: true,
		Entries:          []Entry{{Name: "foo", Port: 3000}},
	}
	got := doc.Render()
	wantBlock := "foo.localhost {\n    reverse_proxy localhost:3000\n}\n"
	if !strings.Contains(got, wantBlock) {
		t.Errorf("Render() missing canonical block. got:\n%s", got)
	}
}

func TestRenderPreservesAboveAndBelow(t *testing.T) {
	doc := Document{
		HasManagedRegion: true,
		Above:            "# above\n",
		Below:            "# below\n",
		Entries:          []Entry{{Name: "foo", Port: 3000}},
	}
	got := doc.Render()
	if !strings.HasPrefix(got, "# above\n") {
		t.Errorf("Render() should preserve Above prefix")
	}
	if !strings.HasSuffix(got, "# below\n") {
		t.Errorf("Render() should preserve Below suffix")
	}
}

func TestRenderAppendsRegionWhenMissing(t *testing.T) {
	doc := Document{
		HasManagedRegion: false,
		Above:            "# user content\n",
	}
	doc.HasManagedRegion = true
	got := doc.Render()
	if !strings.Contains(got, "# user content") {
		t.Error("expected user content to be preserved")
	}
	if !strings.Contains(got, OpenerLine()) {
		t.Error("expected opener to be present")
	}
	if !strings.Contains(got, CloserLine()) {
		t.Error("expected closer to be present")
	}
}

func TestRenderInsertsBlankLineBeforeOpener(t *testing.T) {
	// User content that ends with one newline must get a second newline so
	// there's a visible blank line before the opener.
	doc := Document{
		HasManagedRegion: true,
		Above:            "# user stuff\n",
	}
	got := doc.Render()
	want := "# user stuff\n\n" + OpenerLine()
	if !strings.HasPrefix(got, want) {
		t.Errorf("missing blank-line separator before opener.\n got: %q\nwant prefix: %q", got, want)
	}
}

func TestRenderDoesNotDoubleBlankLineWhenAlreadyPresent(t *testing.T) {
	// Already two trailing newlines — should not add a third.
	doc := Document{
		HasManagedRegion: true,
		Above:            "# user stuff\n\n",
	}
	got := doc.Render()
	want := "# user stuff\n\n" + OpenerLine()
	if !strings.HasPrefix(got, want) {
		t.Errorf("unexpected separator handling.\n got: %q\nwant prefix: %q", got, want)
	}
	// No triple newline before opener.
	if strings.Contains(got, "\n\n\n"+OpenerLine()) {
		t.Errorf("got triple newline before opener: %q", got)
	}
}

func TestRoundTripEmpty(t *testing.T) {
	original := OpenerLine() + "\n\n" + CloserLine() + "\n"
	doc, err := Parse(original)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	got := doc.Render()
	if got != original {
		t.Errorf("round-trip mismatch:\n got: %q\nwant: %q", got, original)
	}
}

func TestRoundTripWithEntries(t *testing.T) {
	original := OpenerLine() + "\n\n" +
		"alpha.localhost {\n    reverse_proxy localhost:1000\n}\n\n" +
		"beta.localhost {\n    reverse_proxy localhost:2000\n}\n\n" +
		CloserLine() + "\n"
	doc, err := Parse(original)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	got := doc.Render()
	if got != original {
		t.Errorf("round-trip mismatch:\n got: %q\nwant: %q", got, original)
	}
}
