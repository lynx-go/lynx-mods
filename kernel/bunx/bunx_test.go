package bunx

import (
	"testing"
)

func TestParseOrderBy(t *testing.T) {
	fields := map[string]string{
		"createdAt": "d.created_at",
		"title":     "d.title",
	}
	cs, err := ParseOrderBy("createdAt desc, title", fields)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(cs) != 2 {
		t.Fatalf("clauses = %d, want 2", len(cs))
	}
	if cs[0].Expr != "d.created_at" || !cs[0].Desc {
		t.Errorf("clause0 = %+v", cs[0])
	}
	if cs[1].Expr != "d.title" || cs[1].Desc {
		t.Errorf("clause1 = %+v", cs[1])
	}

	if _, err := ParseOrderBy("hackerField", fields); err == nil {
		t.Error("unknown field: want error (fail-closed whitelist)")
	}
	if _, err := ParseOrderBy("title sideways", fields); err == nil {
		t.Error("invalid direction: want error")
	}
	if _, err := ParseOrderBy("title desc,, title", fields); err == nil {
		t.Error("empty clause: want error")
	}
	if cs, err := ParseOrderBy("  ", fields); err != nil || cs != nil {
		t.Errorf("empty order: cs=%v err=%v", cs, err)
	}
}

func TestSortedFields(t *testing.T) {
	got := SortedFields(map[string]string{"b": "1", "a": "2", "c": "3"})
	if len(got) != 3 || got[0] != "a" || got[2] != "c" {
		t.Fatalf("SortedFields = %v", got)
	}
}
