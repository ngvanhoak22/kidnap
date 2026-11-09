package auth

import "time"

type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	DisplayName  string    `json:"display_name"`
	CreatedAt    time.Time `json:"created_at"`
	PasswordHash string    `json:"-"`
}
type Session struct {
	ID         string    `json:"id"`
	UserID     string    `json:"-"`
	DeviceName string    `json:"device_name"`
	CreatedAt  time.Time `json:"created_at"`
	LastSeenAt time.Time `json:"last_seen_at"`
	ExpiresAt  time.Time `json:"expires_at"`
	Current    bool      `json:"current"`
}
type Credentials struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
	DeviceName  string `json:"device_name"`
}
type Principal struct {
	User    User
	Session Session
	Token   string
}
type LoginResult struct {
	Principal
	CSRFToken string
}
