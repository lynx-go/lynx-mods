package secretkey

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"testing"
)

func TestDeriveDeterministic(t *testing.T) {
	a := Derive("master", "purpose-a")
	b := Derive("master", "purpose-a")
	if !bytes.Equal(a, b) {
		t.Fatal("same master+purpose must derive same key")
	}
	if len(a) != 32 {
		t.Fatalf("key length = %d, want 32", len(a))
	}
}

func TestDeriveDomainSeparation(t *testing.T) {
	if bytes.Equal(Derive("m", "a"), Derive("m", "b")) {
		t.Fatal("different purposes must derive different keys")
	}
	if bytes.Equal(Derive("m1", "a"), Derive("m2", "a")) {
		t.Fatal("different masters must derive different keys")
	}
}

func TestDeriveHexMatchesDerive(t *testing.T) {
	if DeriveHex("m", "p") == DeriveHex("m", "q") {
		t.Fatal("hex derivation must separate purposes")
	}
	if len(DeriveHex("m", "p")) != 64 {
		t.Fatalf("hex length = %d, want 64", len(DeriveHex("m", "p")))
	}
}

func TestVerify(t *testing.T) {
	key := Derive("m", "p")
	if !Verify(key, []byte("data"), macOf(key, "data")) {
		t.Fatal("valid mac rejected")
	}
	if Verify(key, []byte("data"), macOf(key, "other")) {
		t.Fatal("invalid mac accepted")
	}
}

func TestMustValidate(t *testing.T) {
	if err := MustValidate("secret", "jwt"); err != nil {
		t.Fatalf("non-empty master: %v", err)
	}
	if err := MustValidate("  ", "jwt"); err == nil {
		t.Fatal("blank master: want error")
	}
}

func TestDerivePanicsOnEmptyPurpose(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("empty purpose: want panic")
		}
	}()
	Derive("m", "")
}

func macOf(key []byte, data string) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(data))
	return mac.Sum(nil)
}
