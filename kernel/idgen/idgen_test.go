package idgen

import (
	"regexp"
	"testing"
)

func TestNew(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		id := New()
		if seen[id] {
			t.Fatalf("duplicate id %s", id)
		}
		seen[id] = true
		if !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(id) {
			t.Fatalf("format = %q", id)
		}
	}
}

func TestPrefixed(t *testing.T) {
	id := Prefixed("usr")
	if !regexp.MustCompile(`^usr_[0-9a-f]{32}$`).MatchString(id) {
		t.Fatalf("format = %q", id)
	}
	for _, bad := range []string{"", "Usr", "1usr", "usr-id", "usr.id"} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("prefix %q: want panic", bad)
				}
			}()
			Prefixed(bad)
		}()
	}
}

func TestSecret(t *testing.T) {
	s := Secret(32)
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(s) {
		t.Fatalf("format = %q", s)
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Error("Secret(0): want panic")
			}
		}()
		Secret(0)
	}()
}
