package caddyfile

type Entry struct {
	Name string
	Port int
}

type Document struct {
	Above            string
	Below            string
	Entries          []Entry
	HasManagedRegion bool
}

// Add inserts or updates an entry. Returns true if the document changed.
func (d *Document) Add(name string, port int) bool {
	d.HasManagedRegion = true
	for i, e := range d.Entries {
		if e.Name == name {
			if e.Port == port {
				return false
			}
			d.Entries[i].Port = port
			return true
		}
	}
	d.Entries = append(d.Entries, Entry{Name: name, Port: port})
	return true
}

// Remove deletes an entry by name. Returns true if the document changed.
func (d *Document) Remove(name string) bool {
	for i, e := range d.Entries {
		if e.Name == name {
			d.Entries = append(d.Entries[:i], d.Entries[i+1:]...)
			return true
		}
	}
	return false
}

// Get looks up an entry by name.
func (d *Document) Get(name string) (Entry, bool) {
	for _, e := range d.Entries {
		if e.Name == name {
			return e, true
		}
	}
	return Entry{}, false
}

// Has reports whether an entry exists.
func (d *Document) Has(name string) bool {
	_, ok := d.Get(name)
	return ok
}
