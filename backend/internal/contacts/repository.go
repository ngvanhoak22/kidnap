package contacts

import (
	"context"
	"crypto/subtle"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) *Repository { return &Repository{db} }

type scanner interface{ Scan(...any) error }

func scanInvitation(row scanner) (Invitation, error) {
	var i Invitation
	err := row.Scan(&i.ID, &i.OwnerID, &i.OwnerName, &i.InviteeEmail, &i.Status, &i.CreatedAt, &i.ExpiresAt)
	return i, err
}
func scanContact(row scanner) (Contact, error) {
	var c Contact
	err := row.Scan(&c.ID, &c.OwnerID, &c.ContactUserID, &c.DisplayName, &c.InvitationID, &c.ConfirmedAt, &c.Label)
	return c, err
}

const invitationColumns = `i.id::text,i.owner_id::text,u.display_name,i.invitee_email,CASE WHEN i.status='PENDING' AND i.expires_at<=clock_timestamp() THEN 'EXPIRED' ELSE i.status END,i.created_at,i.expires_at`
const contactColumns = `c.id::text,c.owner_id::text,c.contact_user_id::text,u.display_name,c.invitation_id::text,c.confirmed_at,c.label`

func lockOwner(ctx context.Context, tx pgx.Tx, id string) error {
	var ignored string
	return tx.QueryRow(ctx, `SELECT id::text FROM users WHERE id=$1 FOR UPDATE`, id).Scan(&ignored)
}

func (r *Repository) CreateInvitation(ctx context.Context, owner, email string, hash []byte, ttl time.Duration) (Invitation, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return Invitation{}, err
	}
	defer tx.Rollback(ctx)
	if err = lockOwner(ctx, tx, owner); err != nil {
		return Invitation{}, err
	}
	var ownerEmail, ownerName string
	if err = tx.QueryRow(ctx, `SELECT email,display_name FROM users WHERE id=$1`, owner).Scan(&ownerEmail, &ownerName); err != nil {
		return Invitation{}, err
	}
	if ownerEmail == email {
		return Invitation{}, ErrSelf
	}
	if _, err = tx.Exec(ctx, `UPDATE trusted_contact_invitations SET status='EXPIRED',responded_at=now() WHERE owner_id=$1 AND status='PENDING' AND expires_at<=clock_timestamp()`, owner); err != nil {
		return Invitation{}, err
	}
	var exists bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM trusted_contacts c JOIN users u ON u.id=c.contact_user_id WHERE c.owner_id=$1 AND u.email=$2 AND c.revoked_at IS NULL)`, owner, email).Scan(&exists); err != nil {
		return Invitation{}, err
	}
	if exists {
		return Invitation{}, ErrExists
	}
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM trusted_contact_invitations WHERE owner_id=$1 AND invitee_email=$2 AND status='PENDING')`, owner, email).Scan(&exists); err != nil {
		return Invitation{}, err
	}
	if exists {
		return Invitation{}, ErrInvitationExists
	}
	var count int
	if err = tx.QueryRow(ctx, `SELECT (SELECT count(*) FROM trusted_contacts WHERE owner_id=$1 AND revoked_at IS NULL)+(SELECT count(*) FROM trusted_contact_invitations WHERE owner_id=$1 AND status='PENDING' AND expires_at>clock_timestamp())`, owner).Scan(&count); err != nil {
		return Invitation{}, err
	}
	if count >= 3 {
		return Invitation{}, ErrLimit
	}
	var i Invitation
	i.OwnerName = ownerName
	err = tx.QueryRow(ctx, `INSERT INTO trusted_contact_invitations(id,owner_id,invitee_email,token_hash,expires_at) VALUES($1,$2,$3,$4,now()+make_interval(secs => $5)) RETURNING id::text,owner_id::text,invitee_email,status,created_at,expires_at`, uuid.NewString(), owner, email, hash, ttl.Seconds()).Scan(&i.ID, &i.OwnerID, &i.InviteeEmail, &i.Status, &i.CreatedAt, &i.ExpiresAt)
	if err != nil {
		return Invitation{}, err
	}
	err = tx.Commit(ctx)
	return i, err
}
func (r *Repository) Invitations(ctx context.Context, userID, email string) (Inbox, error) {
	out := Inbox{Incoming: []Invitation{}, Outgoing: []Invitation{}}
	for _, incoming := range []bool{true, false} {
		predicate := `i.owner_id=$1`
		arg := userID
		if incoming {
			predicate = `i.invitee_email=$1`
			arg = email
		}
		rows, err := r.db.Query(ctx, `SELECT `+invitationColumns+` FROM trusted_contact_invitations i JOIN users u ON u.id=i.owner_id WHERE `+predicate+` ORDER BY i.created_at DESC LIMIT 100`, arg)
		if err != nil {
			return out, err
		}
		for rows.Next() {
			i, err := scanInvitation(rows)
			if err != nil {
				rows.Close()
				return out, err
			}
			if incoming {
				out.Incoming = append(out.Incoming, i)
			} else {
				out.Outgoing = append(out.Outgoing, i)
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return out, err
		}
	}
	return out, nil
}
func (r *Repository) Accept(ctx context.Context, id, userID, email string, hash []byte) (Contact, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return Contact{}, err
	}
	defer tx.Rollback(ctx)
	var owner string
	if err = tx.QueryRow(ctx, `SELECT owner_id::text FROM trusted_contact_invitations WHERE id=$1 AND invitee_email=$2`, id, email).Scan(&owner); err != nil {
		return Contact{}, err
	}
	if err = lockOwner(ctx, tx, owner); err != nil {
		return Contact{}, err
	}
	var status string
	var stored []byte
	var expired bool
	if err = tx.QueryRow(ctx, `SELECT status,token_hash,expires_at<=clock_timestamp() FROM trusted_contact_invitations WHERE id=$1 AND invitee_email=$2 FOR UPDATE`, id, email).Scan(&status, &stored, &expired); err != nil {
		return Contact{}, err
	}
	if subtle.ConstantTimeCompare(hash, stored) != 1 {
		return Contact{}, pgx.ErrNoRows
	}
	if status == "ACCEPTED" {
		c, err := scanContact(tx.QueryRow(ctx, `SELECT `+contactColumns+` FROM trusted_contacts c JOIN users u ON u.id=c.contact_user_id WHERE c.invitation_id=$1 AND c.contact_user_id=$2 AND c.revoked_at IS NULL`, id, userID))
		if err == pgx.ErrNoRows {
			return Contact{}, ErrInactive
		}
		return c, err
	}
	if status != "PENDING" {
		if status == "EXPIRED" {
			return Contact{}, ErrExpired
		}
		return Contact{}, ErrInactive
	}
	if expired {
		if _, err = tx.Exec(ctx, `UPDATE trusted_contact_invitations SET status='EXPIRED',responded_at=now() WHERE id=$1`, id); err != nil {
			return Contact{}, err
		}
		if err = tx.Commit(ctx); err != nil {
			return Contact{}, err
		}
		return Contact{}, ErrExpired
	}
	var count int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM trusted_contacts WHERE owner_id=$1 AND revoked_at IS NULL`, owner).Scan(&count); err != nil {
		return Contact{}, err
	}
	if count >= 3 {
		return Contact{}, ErrLimit
	}
	var c Contact
	err = tx.QueryRow(ctx, `INSERT INTO trusted_contacts(id,owner_id,contact_user_id,invitation_id) VALUES($1,$2,$3,$4) ON CONFLICT(owner_id,contact_user_id) DO UPDATE SET invitation_id=EXCLUDED.invitation_id,confirmed_at=now(),revoked_at=NULL WHERE trusted_contacts.revoked_at IS NOT NULL RETURNING id::text,owner_id::text,contact_user_id::text,invitation_id::text,confirmed_at`, uuid.NewString(), owner, userID, id).Scan(&c.ID, &c.OwnerID, &c.ContactUserID, &c.InvitationID, &c.ConfirmedAt)
	if err == pgx.ErrNoRows {
		return Contact{}, ErrExists
	}
	if err != nil {
		return Contact{}, err
	}
	if err = tx.QueryRow(ctx, `SELECT display_name FROM users WHERE id=$1`, userID).Scan(&c.DisplayName); err != nil {
		return Contact{}, err
	}
	if _, err = tx.Exec(ctx, `UPDATE trusted_contact_invitations SET status='ACCEPTED',responded_at=now() WHERE id=$1`, id); err != nil {
		return Contact{}, err
	}
	err = tx.Commit(ctx)
	return c, err
}
func (r *Repository) FinishInvitation(ctx context.Context, id, actor, email, status string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var owner string
	if err = tx.QueryRow(ctx, `SELECT owner_id::text FROM trusted_contact_invitations WHERE id=$1 AND (($2='CANCELED' AND owner_id=$3) OR ($2='DECLINED' AND invitee_email=$4))`, id, status, actor, email).Scan(&owner); err != nil {
		return err
	}
	if err = lockOwner(ctx, tx, owner); err != nil {
		return err
	}
	var current string
	var expired bool
	if err = tx.QueryRow(ctx, `SELECT status,expires_at<=clock_timestamp() FROM trusted_contact_invitations WHERE id=$1 FOR UPDATE`, id).Scan(&current, &expired); err != nil {
		return err
	}
	if current == status {
		return nil
	}
	if current != "PENDING" {
		return ErrInactive
	}
	if expired {
		return ErrExpired
	}
	if _, err = tx.Exec(ctx, `UPDATE trusted_contact_invitations SET status=$2,responded_at=now() WHERE id=$1`, id, status); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (r *Repository) List(ctx context.Context, userID string, incoming bool) ([]Contact, error) {
	predicate := `c.owner_id=$1`
	join := `c.contact_user_id`
	if incoming {
		predicate = `c.contact_user_id=$1`
		join = `c.owner_id`
	}
	rows, err := r.db.Query(ctx, `SELECT `+contactColumns+` FROM trusted_contacts c JOIN users u ON u.id=`+join+` WHERE `+predicate+` AND c.revoked_at IS NULL ORDER BY c.confirmed_at DESC LIMIT 100`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Contact{}
	for rows.Next() {
		c, err := scanContact(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
func (r *Repository) Revoke(ctx context.Context, id, userID string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var owner string
	if err = tx.QueryRow(ctx, `SELECT owner_id::text FROM trusted_contacts WHERE id=$1 AND (owner_id=$2 OR contact_user_id=$2)`, id, userID).Scan(&owner); err != nil {
		return err
	}
	if err = lockOwner(ctx, tx, owner); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE trusted_contacts SET revoked_at=COALESCE(revoked_at,now()) WHERE id=$1`, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) UpdateLabel(ctx context.Context, id, owner, label string) (Contact, error) {
	return scanContact(r.db.QueryRow(ctx, `UPDATE trusted_contacts c SET label=$3 FROM users u WHERE c.id=$1 AND c.owner_id=$2 AND c.contact_user_id=u.id AND c.revoked_at IS NULL RETURNING `+contactColumns, id, owner, label))
}
