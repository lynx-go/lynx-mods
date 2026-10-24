package errcode

import (
	"strings"
	"testing"
)

func TestRegisterDuplicate(t *testing.T) {
	if _, err := Register("t-dup-a"); err != nil {
		t.Fatalf("first register: %v", err)
	}
	if _, err := Register("t-dup-a"); err == nil {
		t.Fatal("duplicate register: want error")
	}
}

func TestSpaceNew(t *testing.T) {
	s := MustRegister("t-space")
	notFound := s.New("USER_NOT_FOUND", "user does not exist")

	if got := notFound.Canonical(); got != "t-space/USER_NOT_FOUND" {
		t.Errorf("Canonical = %q", got)
	}
	if got, want := notFound.Error(), "t-space/USER_NOT_FOUND: user does not exist"; got != want {
		t.Errorf("Error = %q, want %q", got, want)
	}
	if !strings.Contains(notFound.Error(), notFound.Message) {
		t.Error("Error should contain message")
	}

	// 重复定义是编程错误（panic）。
	defer func() {
		if recover() == nil {
			t.Error("duplicate code: want panic")
		}
	}()
	_ = s.New("user_not_found", "again")
}

func TestLookup(t *testing.T) {
	s := MustRegister("t-lookup")
	code := s.New("OK_CODE", "")
	if info, ok := Lookup("t-lookup/OK_CODE"); !ok || info != code {
		t.Errorf("Lookup = %v, %v", info, ok)
	}
	if _, ok := Lookup("t-lookup/NOPE"); ok {
		t.Error("Lookup unknown: want miss")
	}
}

func TestDomains(t *testing.T) {
	MustRegister("t-domains")
	found := false
	for _, d := range Domains() {
		if d == "t-domains" {
			found = true
		}
	}
	if !found {
		t.Errorf("Domains() missing t-domains: %v", Domains())
	}
}
