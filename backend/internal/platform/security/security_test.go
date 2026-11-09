package security

import (
	"context"
	"testing"
)

func TestPasswordHashAndVerification(t *testing.T) {
	p := NewPasswords()
	ctx := context.Background()
	password := "mật-khẩu-thử-nghiệm-123"
	a, err := p.Hash(ctx, password)
	if err != nil {
		t.Fatal(err)
	}
	b, err := p.Hash(ctx, password)
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("password salts were reused")
	}
	if !p.Verify(ctx, password, a) || p.Verify(ctx, "wrong-password", a) {
		t.Fatal("verification incorrect")
	}
	for _, bad := range []string{"", "$argon2id$v=19$m=999999999,t=2,p=1$bad$bad", "$argon2id$v=19$m=19456,t=2,p=1$bad$bad"} {
		if p.Verify(ctx, password, bad) {
			t.Fatal("malformed hash accepted")
		}
	}
}
func TestTokensAndInput(t *testing.T) {
	a, err := Token()
	if err != nil {
		t.Fatal(err)
	}
	b, err := Token()
	if err != nil {
		t.Fatal(err)
	}
	if a == b || !ValidToken(a) || ValidToken(a+"x") || len(HashToken(a)) != 32 || CSRF(a) == CSRF(b) {
		t.Fatal("token invariant failed")
	}
	if email, ok := Email("  TEST@Example.com  "); !ok || email != "test@example.com" {
		t.Fatal("email canonicalization")
	}
	for _, email := range []string{"Alice <a@example.com>", "bad", "a@example.com\r\nBcc:x@example.com"} {
		if _, ok := Email(email); ok {
			t.Fatal("invalid email accepted")
		}
	}
	if ValidPassword("short") || !ValidPassword("mật-khẩu-dài-123") || ValidName("\x00", true) {
		t.Fatal("input validation")
	}
}
