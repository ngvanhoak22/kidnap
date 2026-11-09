package auth

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) *Repository { return &Repository{db} }

type scanner interface{ Scan(...any) error }

func scanUser(row scanner) (User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Email, &u.DisplayName, &u.CreatedAt, &u.PasswordHash)
	return u, err
}

const userColumns = `id::text,email,display_name,created_at,password_hash`

func (r *Repository) FindByEmail(ctx context.Context, email string) (User, error) {
	return scanUser(r.db.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE email=$1`, email))
}
func (r *Repository) Create(ctx context.Context, email, name, hash string, tokenHash []byte, device string, ttl time.Duration) (User, Session, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return User{}, Session{}, err
	}
	defer tx.Rollback(ctx)
	u, err := scanUser(tx.QueryRow(ctx, `INSERT INTO users(id,email,display_name,password_hash) VALUES($1,$2,$3,$4) RETURNING `+userColumns, uuid.NewString(), email, name, hash))
	if err != nil {
		return User{}, Session{}, err
	}
	s, err := insertSession(ctx, tx, u.ID, tokenHash, device, ttl)
	if err != nil {
		return User{}, Session{}, err
	}
	err = tx.Commit(ctx)
	return u, s, err
}
func insertSession(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, userID string, hash []byte, device string, ttl time.Duration) (Session, error) {
	var s Session
	err := q.QueryRow(ctx, `INSERT INTO user_sessions(id,user_id,token_hash,device_name,expires_at) VALUES($1,$2,$3,$4,now()+make_interval(secs => $5)) RETURNING id::text,user_id::text,device_name,created_at,last_seen_at,expires_at`, uuid.NewString(), userID, hash, device, ttl.Seconds()).Scan(&s.ID, &s.UserID, &s.DeviceName, &s.CreatedAt, &s.LastSeenAt, &s.ExpiresAt)
	return s, err
}
func (r *Repository) CreateSession(ctx context.Context, userID string, hash []byte, device string, ttl time.Duration) (Session, error) {
	return insertSession(ctx, r.db, userID, hash, device, ttl)
}
func (r *Repository) Authenticate(ctx context.Context, hash []byte) (User, Session, error) {
	var u User
	var s Session
	err := r.db.QueryRow(ctx, `SELECT u.id::text,u.email,u.display_name,u.created_at,s.id::text,s.device_name,s.created_at,s.last_seen_at,s.expires_at FROM user_sessions s JOIN users u ON u.id=s.user_id WHERE s.token_hash=$1 AND s.revoked_at IS NULL AND s.expires_at>clock_timestamp()`, hash).Scan(&u.ID, &u.Email, &u.DisplayName, &u.CreatedAt, &s.ID, &s.DeviceName, &s.CreatedAt, &s.LastSeenAt, &s.ExpiresAt)
	s.UserID = u.ID
	if err != nil {
		return u, s, err
	}
	if time.Since(s.LastSeenAt) > 5*time.Minute {
		_, err = r.db.Exec(ctx, `UPDATE user_sessions SET last_seen_at=now() WHERE id=$1 AND last_seen_at<now()-interval '5 minutes'`, s.ID)
	}
	return u, s, err
}
func (r *Repository) Sessions(ctx context.Context, userID, current string) ([]Session, error) {
	rows, err := r.db.Query(ctx, `SELECT id::text,device_name,created_at,last_seen_at,expires_at FROM user_sessions WHERE user_id=$1 AND revoked_at IS NULL AND expires_at>now() ORDER BY created_at DESC LIMIT 100`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Session{}
	for rows.Next() {
		var s Session
		if err := rows.Scan(&s.ID, &s.DeviceName, &s.CreatedAt, &s.LastSeenAt, &s.ExpiresAt); err != nil {
			return nil, err
		}
		s.Current = s.ID == current
		out = append(out, s)
	}
	return out, rows.Err()
}
func (r *Repository) Revoke(ctx context.Context, userID, id string) error {
	tag, err := r.db.Exec(ctx, `UPDATE user_sessions SET revoked_at=COALESCE(revoked_at,now()) WHERE user_id=$1 AND id=$2`, userID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
func (r *Repository) RevokeAll(ctx context.Context, userID string) error {
	_, err := r.db.Exec(ctx, `UPDATE user_sessions SET revoked_at=now() WHERE user_id=$1 AND revoked_at IS NULL`, userID)
	return err
}
func (r *Repository) UpdateName(ctx context.Context, id, name string) (User, error) {
	return scanUser(r.db.QueryRow(ctx, `UPDATE users SET display_name=$2,updated_at=now() WHERE id=$1 RETURNING `+userColumns, id, name))
}
