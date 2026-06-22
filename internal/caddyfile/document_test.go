package caddyfile

import "testing"

func TestAddNewEntry(t *testing.T) {
	doc := Document{HasManagedRegion: true}
	changed := doc.Add("foo", 3000)
	if !changed {
		t.Error("Add should return true when entry is new")
	}
	got, ok := doc.Get("foo")
	if !ok {
		t.Fatal("Get should find newly added entry")
	}
	if got != (Entry{Name: "foo", Port: 3000}) {
		t.Errorf("Get(foo) = %+v, want {foo 3000}", got)
	}
}

func TestAddSamePortNoOp(t *testing.T) {
	doc := Document{
		HasManagedRegion: true,
		Entries:          []Entry{{Name: "foo", Port: 3000}},
	}
	changed := doc.Add("foo", 3000)
	if changed {
		t.Error("Add with same name+port should return false (no change)")
	}
	if len(doc.Entries) != 1 {
		t.Errorf("entries should remain 1, got %d", len(doc.Entries))
	}
}

func TestAddDifferentPortReplaces(t *testing.T) {
	doc := Document{
		HasManagedRegion: true,
		Entries:          []Entry{{Name: "foo", Port: 3000}},
	}
	changed := doc.Add("foo", 3001)
	if !changed {
		t.Error("Add with same name but different port should return true")
	}
	got, _ := doc.Get("foo")
	if got.Port != 3001 {
		t.Errorf("port not updated: got %d, want 3001", got.Port)
	}
	if len(doc.Entries) != 1 {
		t.Errorf("should still be 1 entry, got %d", len(doc.Entries))
	}
}

func TestRemoveExisting(t *testing.T) {
	doc := Document{
		HasManagedRegion: true,
		Entries:          []Entry{{Name: "foo", Port: 3000}},
	}
	changed := doc.Remove("foo")
	if !changed {
		t.Error("Remove of existing entry should return true")
	}
	if _, ok := doc.Get("foo"); ok {
		t.Error("Get should not find removed entry")
	}
}

func TestRemoveNonexistent(t *testing.T) {
	doc := Document{HasManagedRegion: true}
	changed := doc.Remove("nope")
	if changed {
		t.Error("Remove of nonexistent entry should return false")
	}
}

func TestAddInitializesManagedRegion(t *testing.T) {
	doc := Document{HasManagedRegion: false}
	doc.Add("foo", 3000)
	if !doc.HasManagedRegion {
		t.Error("Add should set HasManagedRegion=true")
	}
}
