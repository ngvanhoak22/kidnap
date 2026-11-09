package contacts

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"journey-safety/backend/internal/auth"
	"journey-safety/backend/internal/platform/httpapi"
	"journey-safety/backend/internal/platform/security"
)

type Service struct {
	repo *Repository
	ttl  time.Duration
}

func NewService(r *Repository, ttl time.Duration) *Service { return &Service{r, ttl} }
func publicError(err error) error {
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return httpapi.NotFound()
	case errors.Is(err, ErrLimit):
		return httpapi.NewError(409, "CONTACT_LIMIT", "Tối đa 3 người thân và lời mời đang chờ.")
	case errors.Is(err, ErrExists):
		return httpapi.NewError(409, "CONTACT_EXISTS", "Quan hệ tin cậy đã tồn tại.")
	case errors.Is(err, ErrInvitationExists):
		return httpapi.NewError(409, "INVITATION_EXISTS", "Đã có lời mời đang chờ cho email này.")
	case errors.Is(err, ErrInactive):
		return httpapi.NewError(409, "INVITATION_INACTIVE", "Lời mời không còn hiệu lực.")
	case errors.Is(err, ErrExpired):
		return httpapi.NewError(410, "INVITATION_EXPIRED", "Lời mời đã hết hạn.")
	case errors.Is(err, ErrSelf):
		return httpapi.Invalid("Không thể tự mời chính mình.")
	default:
		return err
	}
}
func (s *Service) Invite(ctx context.Context, p auth.Principal, email string) (Invitation, string, error) {
	email, ok := security.Email(email)
	if !ok {
		return Invitation{}, "", httpapi.Invalid("Email không hợp lệ.")
	}
	token, err := security.Token()
	if err != nil {
		return Invitation{}, "", err
	}
	i, err := s.repo.CreateInvitation(ctx, p.User.ID, email, security.HashToken(token), s.ttl)
	return i, token, publicError(err)
}
func (s *Service) Invitations(ctx context.Context, p auth.Principal) (Inbox, error) {
	return s.repo.Invitations(ctx, p.User.ID, p.User.Email)
}
func (s *Service) Accept(ctx context.Context, p auth.Principal, id, token string) (Contact, error) {
	if !security.ValidToken(token) {
		return Contact{}, httpapi.NotFound()
	}
	c, err := s.repo.Accept(ctx, id, p.User.ID, p.User.Email, security.HashToken(token))
	return c, publicError(err)
}
func (s *Service) Finish(ctx context.Context, p auth.Principal, id, status string) error {
	return publicError(s.repo.FinishInvitation(ctx, id, p.User.ID, p.User.Email, status))
}
func (s *Service) List(ctx context.Context, p auth.Principal, incoming bool) ([]Contact, error) {
	return s.repo.List(ctx, p.User.ID, incoming)
}
func (s *Service) Revoke(ctx context.Context, p auth.Principal, id string) error {
	return publicError(s.repo.Revoke(ctx, id, p.User.ID))
}

func (s *Service) UpdateLabel(ctx context.Context, p auth.Principal, id, label string) (Contact, error) {
	label = strings.TrimSpace(label)
	if !security.ValidName(label, false) {
		return Contact{}, httpapi.Invalid("Tên gợi nhớ tối đa 80 ký tự.")
	}
	c, err := s.repo.UpdateLabel(ctx, id, p.User.ID, label)
	return c, publicError(err)
}
