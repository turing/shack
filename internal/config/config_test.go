package config

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestProjectRoundTrip(t *testing.T) {
	p := Project{
		Name: "fmdplanner",
		Commands: []Command{
			{
				Label: "dev:server",
				Cmd:   "pnpm dev:server",
				Listeners: []Listener{
					{Match: Match{Script: "dev:server"}, As: "fmdplanner-api"},
				},
			},
		},
	}
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got Project
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !reflect.DeepEqual(p, got) {
		t.Errorf("round-trip mismatch:\n got: %+v\nwant: %+v", got, p)
	}
}

func TestMatchKindReturnsExactlyOne(t *testing.T) {
	cases := []struct {
		m    Match
		want string
	}{
		{Match{Script: "dev"}, "script"},
		{Match{Package: "web"}, "package"},
		{Match{Comm: "vite"}, "comm"},
		{Match{}, ""},
	}
	for _, c := range cases {
		if got := c.m.Kind(); got != c.want {
			t.Errorf("Kind(%+v) = %q, want %q", c.m, got, c.want)
		}
	}
}

func TestMatchValidateRejectsMultipleKeys(t *testing.T) {
	if err := (Match{Script: "dev", Package: "web"}).Validate(); err == nil {
		t.Error("expected error when multiple match keys are set")
	}
}

func TestMatchValidateRejectsEmpty(t *testing.T) {
	if err := (Match{}).Validate(); err == nil {
		t.Error("expected error on empty Match")
	}
}

func TestCommandPreRoundTrip(t *testing.T) {
	p := Project{
		Name: "myapp",
		Commands: []Command{
			{
				Label: "start",
				Cmd:   "pnpm start",
				Pre:   "pnpm build",
				Listeners: []Listener{
					{Match: Match{Script: "start"}, As: "myapp"},
				},
			},
		},
	}
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got Project
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !reflect.DeepEqual(p, got) {
		t.Errorf("round-trip mismatch:\n got: %+v\nwant: %+v", got, p)
	}
	if got.Commands[0].Pre != "pnpm build" {
		t.Errorf("Pre = %q, want %q", got.Commands[0].Pre, "pnpm build")
	}
}

func TestCommandPreOmitEmpty(t *testing.T) {
	p := Project{
		Name: "myapp",
		Commands: []Command{
			{
				Label:     "dev",
				Cmd:       "pnpm dev",
				Listeners: []Listener{{Match: Match{Script: "dev"}, As: "myapp"}},
			},
		},
	}
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if strings.Contains(string(b), `"pre"`) {
		t.Errorf("expected 'pre' to be omitted when empty, got: %s", b)
	}
}

func TestProjectDefaultsRoundTrip(t *testing.T) {
	p := Project{
		Name:     "myapp",
		Defaults: []string{"dev:server", "dev:web"},
		Commands: []Command{
			{
				Label: "dev:server",
				Cmd:   "pnpm dev:server",
				Listeners: []Listener{
					{Match: Match{Script: "dev:server"}, As: "myapp-server"},
				},
			},
			{
				Label:     "dev:web",
				Cmd:       "pnpm dev:web",
				Listeners: []Listener{{Match: Match{Script: "dev:web"}, As: "myapp-web"}},
			},
		},
	}
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got Project
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !reflect.DeepEqual(p, got) {
		t.Errorf("round-trip mismatch:\n got: %+v\nwant: %+v", got, p)
	}
	// Verify omitempty: empty Defaults should not appear in JSON
	pEmpty := Project{Name: "x", Commands: []Command{}}
	bEmpty, _ := json.Marshal(pEmpty)
	if strings.Contains(string(bEmpty), "defaults") {
		t.Errorf("expected 'defaults' to be omitted for empty slice, got: %s", bEmpty)
	}
}
