package security

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net/mail"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

const memory uint32 = 19 * 1024

type Passwords struct{ slots chan struct{} }

func NewPasswords() *Passwords { return &Passwords{slots: make(chan struct{}, 2)} }
func (p *Passwords) acquire(ctx context.Context) error {
	select {
	case p.slots <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (p *Passwords) Hash(ctx context.Context, password string) (string, error) {
	if err := p.acquire(ctx); err != nil {
		return "", err
	}
	defer func() { <-p.slots }()
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, 2, memory, 1, 32)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=2,p=1$%s$%s", memory, base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}
func (p *Passwords) Verify(ctx context.Context, password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return false
	}
	var m, t uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &threads); err != nil || m != memory || t != 2 || threads != 1 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) != 16 {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(want) != 32 {
		return false
	}
	if p.acquire(ctx) != nil {
		return false
	}
	defer func() { <-p.slots }()
	got := argon2.IDKey([]byte(password), salt, t, m, threads, 32)
	return subtle.ConstantTimeCompare(got, want) == 1
}
func Token() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
func ValidToken(v string) bool {
	b, err := base64.RawURLEncoding.DecodeString(v)
	return err == nil && len(b) == 32 && len(v) == 43
}
func HashToken(v string) []byte { sum := sha256.Sum256([]byte(v)); return sum[:] }
func CSRF(v string) string {
	sum := sha256.Sum256([]byte("journey-csrf-v1:" + v))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
func Equal(a, b string) bool { return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1 }
func Email(v string) (string, bool) {
	v = strings.ToLower(strings.TrimSpace(v))
	parsed, err := mail.ParseAddress(v)
	return v, err == nil && parsed.Address == v && len(v) <= 254 && !strings.ContainsAny(v, "\r\n") && strings.Contains(v, ".")
}
func ValidPassword(v string) bool {
	n := utf8.RuneCountInString(v)
	return utf8.ValidString(v) && n >= 12 && n <= 128 && len(v) <= 1024
}
func ValidName(v string, required bool) bool {
	n := utf8.RuneCountInString(v)
	return utf8.ValidString(v) && n <= 80 && (!required || n > 0) && !strings.ContainsAny(v, "\r\n\x00")
}
