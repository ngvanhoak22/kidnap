-- +goose Up
ALTER TABLE trusted_contacts ADD COLUMN label TEXT NOT NULL DEFAULT '' CHECK (length(label)<=80);

-- +goose Down
ALTER TABLE trusted_contacts DROP COLUMN label;
