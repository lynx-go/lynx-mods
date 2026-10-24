package password

import (
	"strings"
	"testing"
)

func TestHashVerifyRoundtrip(t *testing.T) {
	encoded, err := Hash("correct horse battery staple")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if !strings.HasPrefix(encoded, "$argon2id$v=19$m=65536,t=3,p=4$") {
		t.Fatalf("unexpected format: %s", encoded)
	}
	ok, err := Verify("correct horse battery staple", encoded)
	if err != nil || !ok {
		t.Fatalf("verify correct: ok=%v err=%v", ok, err)
	}
	ok, err = Verify("wrong password", encoded)
	if err != nil || ok {
		t.Fatalf("verify wrong: ok=%v err=%v", ok, err)
	}
}

func TestVerifyMalformed(t *testing.T) {
	for _, bad := range []string{
		"", "not-a-hash", "$argon2i$v=19$m=1,t=1,p=1$abc$def", "$argon2id$v=19$m=1,t=1,p=1$only$two$extra",
	} {
		if _, err := Verify("x", bad); err == nil {
			t.Errorf("Verify(%q): want error", bad)
		}
	}
}

func TestUniqueSalts(t *testing.T) {
	a, _ := Hash("same-password")
	b, _ := Hash("same-password")
	if a == b {
		t.Fatal("same password must produce different hashes (random salt)")
	}
}
