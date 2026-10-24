package clock

import (
	"testing"
	"time"
)

func TestSystemUTC(t *testing.T) {
	now := System{}.Now()
	if now.Location() != time.UTC {
		t.Fatalf("System.Now location = %v, want UTC", now.Location())
	}
}

func TestFakeSetAndAdvance(t *testing.T) {
	start := time.Date(2026, 10, 24, 12, 0, 0, 0, time.FixedZone("CST", 8*3600))
	f := NewFake(start)
	if loc := f.Now().Location(); loc != time.UTC {
		t.Fatalf("Fake normalizes to UTC, got %v", loc)
	}
	if want := start.UTC(); !f.Now().Equal(want) {
		t.Fatalf("after init = %v, want %v", f.Now(), want)
	}
	f.Advance(90 * time.Minute)
	if want := start.UTC().Add(90 * time.Minute); !f.Now().Equal(want) {
		t.Fatalf("after advance = %v, want %v", f.Now(), want)
	}
	f.Set(time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC))
	if f.Now().Year() != 2027 {
		t.Fatal("Set did not take effect")
	}
}
