package auth

import "testing"

func TestHashVerifyPassword(t *testing.T) {
	hash, err := HashPassword("correct horse battery", DefaultArgon2Params())
	if err != nil {
		t.Fatal(err)
	}
	ok, err := VerifyPassword(hash, "correct horse battery")
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	ok, err = VerifyPassword(hash, "wrong")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected mismatch")
	}
}
