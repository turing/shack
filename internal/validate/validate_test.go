package validate

import "testing"

func TestNameValid(t *testing.T) {
	cases := []string{"foo", "foo-bar", "foo123", "1foo", "a", "a-b-c"}
	for _, c := range cases {
		if err := Name(c); err != nil {
			t.Errorf("Name(%q) returned error: %v", c, err)
		}
	}
}

func TestNameInvalid(t *testing.T) {
	cases := []string{"", "Foo", "foo bar", "foo.bar", "foo_bar", "-foo", "foo!", "FOO"}
	for _, c := range cases {
		if err := Name(c); err == nil {
			t.Errorf("Name(%q) should have returned error", c)
		}
	}
}

func TestPortValid(t *testing.T) {
	cases := []int{1, 80, 3000, 8080, 65535}
	for _, c := range cases {
		if err := Port(c); err != nil {
			t.Errorf("Port(%d) returned error: %v", c, err)
		}
	}
}

func TestPortInvalid(t *testing.T) {
	cases := []int{0, -1, 65536, 99999}
	for _, c := range cases {
		if err := Port(c); err == nil {
			t.Errorf("Port(%d) should have returned error", c)
		}
	}
}
