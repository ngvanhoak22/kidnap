-- +goose Up
CREATE TABLE users (
    id UUID PRIMARY KEY,
    email TEXT NOT NULL UNIQUE CHECK (email=lower(btrim(email)) AND length(email) BETWEEN 3 AND 254),
    password_hash TEXT NOT NULL,
    display_name TEXT NOT NULL CHECK (length(display_name) BETWEEN 1 AND 80),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE user_sessions (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash BYTEA NOT NULL UNIQUE CHECK (octet_length(token_hash)=32),
    device_name TEXT NOT NULL DEFAULT '' CHECK (length(device_name)<=80),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL CHECK (expires_at>created_at),
    revoked_at TIMESTAMPTZ
);
CREATE INDEX user_sessions_user_idx ON user_sessions(user_id,created_at DESC);
CREATE TABLE trusted_contact_invitations (
    id UUID PRIMARY KEY,
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    invitee_email TEXT NOT NULL CHECK (invitee_email=lower(btrim(invitee_email)) AND length(invitee_email) BETWEEN 3 AND 254),
    token_hash BYTEA NOT NULL UNIQUE CHECK (octet_length(token_hash)=32),
    status TEXT NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING','ACCEPTED','DECLINED','CANCELED','EXPIRED')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL CHECK (expires_at>created_at),
    responded_at TIMESTAMPTZ,
    CHECK ((status='PENDING')=(responded_at IS NULL))
);
CREATE UNIQUE INDEX invitations_pending_owner_email_idx ON trusted_contact_invitations(owner_id,invitee_email) WHERE status='PENDING';
CREATE INDEX invitations_owner_created_idx ON trusted_contact_invitations(owner_id,created_at DESC);
CREATE INDEX invitations_email_created_idx ON trusted_contact_invitations(invitee_email,created_at DESC);
CREATE TABLE trusted_contacts (
    id UUID PRIMARY KEY,
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    contact_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    invitation_id UUID NOT NULL UNIQUE REFERENCES trusted_contact_invitations(id),
    confirmed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at TIMESTAMPTZ,
    UNIQUE(owner_id,contact_user_id),
    CHECK (owner_id<>contact_user_id)
);
CREATE INDEX contacts_recipient_idx ON trusted_contacts(contact_user_id) WHERE revoked_at IS NULL;

-- +goose Down
DROP TABLE trusted_contacts;
DROP TABLE trusted_contact_invitations;
DROP TABLE user_sessions;
DROP TABLE users;
