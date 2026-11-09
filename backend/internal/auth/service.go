package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"journey-safety/backend/internal/platform/httpapi"
	"journey-safety/backend/internal/platform/security"
)

type Service struct {
	repo      *Repository
	passwords *security.Passwords
	ttl       time.Duration
	dummyHash string
}

func NewService(repo *Repository, ttl time.Duration) (*Service, error) {
	p := security.NewPasswords()
	h, err := p.Hash(context.Background(), "dummy-password-for-timing-only")
	return &Service{repo, p, ttl, h}, err
}
func (s *Service) Register(ctx context.Context, in Credentials) (LoginResult, error) {
	email, ok := security.Email(in.Email)
	name := strings.TrimSpace(in.DisplayName)
	device := strings.TrimSpace(in.DeviceName)
	if !ok || !security.ValidPassword(in.Password) || !security.ValidName(name, true) || !security.ValidName(device, false) {
		return LoginResult{}, httpapi.Invalid("Email, tên hoặc mật khẩu không hợp lệ (mật khẩu 12–128 ký tự).")
	}
	hash, err := s.passwords.Hash(ctx, in.Password)
	if err != nil {
		return LoginResult{}, err
	}
	token, err := security.Token()
	if err != nil {
		return LoginResult{}, err
	}
	u, session, err := s.repo.Create(ctx, email, name, hash, security.HashToken(token), device, s.ttl)
	var pgerr *pgconn.PgError
	if errors.As(err, &pgerr) && pgerr.Code == "23505" {
		return LoginResult{}, httpapi.NewError(409, "EMAIL_EXISTS", "Email đã được đăng ký.")
	}
	if err != nil {
		return LoginResult{}, err
	}
	return LoginResult{Principal{u, session, token}, security.CSRF(token)}, nil
}
func (s *Service) Login(ctx context.Context, in Credentials) (LoginResult, error) {
	email, ok := security.Email(in.Email)
	if !ok || len(in.Password) > 1024 || !security.ValidName(strings.TrimSpace(in.DeviceName), false) {
		return LoginResult{}, httpapi.Invalid("Thông tin đăng nhập không hợp lệ.")
	}
	u, err := s.repo.FindByEmail(ctx, email)
	hash := u.PasswordHash
	if errors.Is(err, pgx.ErrNoRows) {
		hash = s.dummyHash
	} else if err != nil {
		return LoginResult{}, err
	}
	valid := s.passwords.Verify(ctx, in.Password, hash)
	if !valid || u.ID == "" {
		return LoginResult{}, httpapi.NewError(401, "INVALID_CREDENTIALS", "Email hoặc mật khẩu không đúng.")
	}
	token, err := security.Token()
	if err != nil {
		return LoginResult{}, err
	}
	session, err := s.repo.CreateSession(ctx, u.ID, security.HashToken(token), strings.TrimSpace(in.DeviceName), s.ttl)
	if err != nil {
		return LoginResult{}, err
	}
	return LoginResult{Principal{u, session, token}, security.CSRF(token)}, nil
}
func (s *Service) Authenticate(ctx context.Context, token string) (Principal, error) {
	if !security.ValidToken(token) {
		return Principal{}, httpapi.Unauthorized()
	}
	u, session, err := s.repo.Authenticate(ctx, security.HashToken(token))
	if errors.Is(err, pgx.ErrNoRows) {
		return Principal{}, httpapi.Unauthorized()
	}
	return Principal{u, session, token}, err
}
func (s *Service) Sessions(ctx context.Context, p Principal) ([]Session, error) {
	return s.repo.Sessions(ctx, p.User.ID, p.Session.ID)
}
func (s *Service) Revoke(ctx context.Context, p Principal, id string) error {
	err := s.repo.Revoke(ctx, p.User.ID, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return httpapi.NotFound()
	}
	return err
}
func (s *Service) RevokeAll(ctx context.Context, p Principal) error {
	return s.repo.RevokeAll(ctx, p.User.ID)
}
func (s *Service) UpdateName(ctx context.Context, p Principal, name string) (User, error) {
	name = strings.TrimSpace(name)
	if !security.ValidName(name, true) {
		return User{}, httpapi.Invalid("Tên cần có 1–80 ký tự.")
	}
	return s.repo.UpdateName(ctx, p.User.ID, name)
}
