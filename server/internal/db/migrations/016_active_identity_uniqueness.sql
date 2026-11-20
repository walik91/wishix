-- +goose NO TRANSACTION
-- +goose Up
-- Install active-only uniqueness before removing the old global constraints.
CREATE UNIQUE INDEX IF NOT EXISTS users_active_username ON users (username) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS users_active_email ON users (email) WHERE deleted_at IS NULL;
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_username_key;
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_email_key;

ALTER TABLE auth_google_identities ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
ALTER TABLE auth_google_identities ALTER PRIMARY KEY USING COLUMNS (subject, user_id);
CREATE UNIQUE INDEX IF NOT EXISTS google_active_subject ON auth_google_identities (subject) WHERE deleted_at IS NULL;
DROP INDEX IF EXISTS auth_google_identities@auth_google_identities_subject_key;
UPDATE auth_google_identities SET deleted_at = (SELECT u.deleted_at FROM users u WHERE u.id = auth_google_identities.user_id)
WHERE deleted_at IS NULL AND EXISTS (SELECT 1 FROM users u WHERE u.id = auth_google_identities.user_id AND u.deleted_at IS NOT NULL);

ALTER TABLE auth_facebook_identities ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
ALTER TABLE auth_facebook_identities ALTER PRIMARY KEY USING COLUMNS (subject, user_id);
CREATE UNIQUE INDEX IF NOT EXISTS facebook_active_subject ON auth_facebook_identities (subject) WHERE deleted_at IS NULL;
DROP INDEX IF EXISTS auth_facebook_identities@auth_facebook_identities_subject_key;
UPDATE auth_facebook_identities SET deleted_at = (SELECT u.deleted_at FROM users u WHERE u.id = auth_facebook_identities.user_id)
WHERE deleted_at IS NULL AND EXISTS (SELECT 1 FROM users u WHERE u.id = auth_facebook_identities.user_id AND u.deleted_at IS NOT NULL);

ALTER TABLE username_registry ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
ALTER TABLE username_registry ALTER PRIMARY KEY USING COLUMNS (username, user_id);
CREATE UNIQUE INDEX IF NOT EXISTS username_registry_active ON username_registry (username) WHERE deleted_at IS NULL;
DROP INDEX IF EXISTS username_registry@username_registry_username_key;
UPDATE username_registry SET deleted_at = (SELECT u.deleted_at FROM users u WHERE u.id = username_registry.user_id)
WHERE deleted_at IS NULL AND EXISTS (SELECT 1 FROM users u WHERE u.id = username_registry.user_id AND u.deleted_at IS NOT NULL);

-- +goose Down
-- Refuse rollback before making schema changes if reused identities cannot fit
-- the previous global uniqueness rules. Never discard historical rows.
SELECT 1 / CASE WHEN
  EXISTS (SELECT username FROM users GROUP BY username HAVING count(*) > 1) OR
  EXISTS (SELECT email FROM users WHERE email IS NOT NULL GROUP BY email HAVING count(*) > 1) OR
  EXISTS (SELECT subject FROM auth_google_identities GROUP BY subject HAVING count(*) > 1) OR
  EXISTS (SELECT subject FROM auth_facebook_identities GROUP BY subject HAVING count(*) > 1) OR
  EXISTS (SELECT username FROM username_registry GROUP BY username HAVING count(*) > 1)
THEN 0 ELSE 1 END;
CREATE UNIQUE INDEX IF NOT EXISTS users_username_key ON users (username);
CREATE UNIQUE INDEX IF NOT EXISTS users_email_key ON users (email);
CREATE UNIQUE INDEX IF NOT EXISTS auth_google_identities_subject_key ON auth_google_identities (subject);
CREATE UNIQUE INDEX IF NOT EXISTS auth_facebook_identities_subject_key ON auth_facebook_identities (subject);
CREATE UNIQUE INDEX IF NOT EXISTS username_registry_username_key ON username_registry (username);
ALTER TABLE auth_google_identities ALTER PRIMARY KEY USING COLUMNS (subject);
ALTER TABLE auth_facebook_identities ALTER PRIMARY KEY USING COLUMNS (subject);
ALTER TABLE username_registry ALTER PRIMARY KEY USING COLUMNS (username);
DROP INDEX IF EXISTS users@users_active_username;
DROP INDEX IF EXISTS users@users_active_email;
DROP INDEX IF EXISTS auth_google_identities@google_active_subject;
DROP INDEX IF EXISTS auth_facebook_identities@facebook_active_subject;
DROP INDEX IF EXISTS username_registry@username_registry_active;
ALTER TABLE auth_google_identities DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE auth_facebook_identities DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE username_registry DROP COLUMN IF EXISTS deleted_at;
