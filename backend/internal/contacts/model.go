package contacts

import (
	"errors"
	"time"
)

var (
	ErrLimit            = errors.New("contact limit")
	ErrExists           = errors.New("contact exists")
	ErrInvitationExists = errors.New("invitation exists")
	ErrInactive         = errors.New("invitation inactive")
	ErrExpired          = errors.New("invitation expired")
	ErrSelf             = errors.New("self invitation")
)

type Invitation struct {
	ID           string    `json:"id"`
	OwnerID      string    `json:"owner_id"`
	OwnerName    string    `json:"owner_name"`
	InviteeEmail string    `json:"invitee_email"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	ExpiresAt    time.Time `json:"expires_at"`
}
type Contact struct {
	ID            string    `json:"id"`
	OwnerID       string    `json:"owner_id"`
	ContactUserID string    `json:"contact_user_id"`
	DisplayName   string    `json:"display_name"`
	Label         string    `json:"label"`
	InvitationID  string    `json:"invitation_id"`
	ConfirmedAt   time.Time `json:"confirmed_at"`
}
type Inbox struct {
	Incoming []Invitation `json:"incoming"`
	Outgoing []Invitation `json:"outgoing"`
}
