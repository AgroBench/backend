-- Identidade: usuários, OTP e refresh tokens (README §6, §10).
-- CPF nunca é persistido em texto puro — só o HMAC-SHA256 com pepper.

CREATE TABLE users (
    id             UUID PRIMARY KEY,
    email          VARCHAR(255) NOT NULL UNIQUE,
    phone          VARCHAR(20)  NOT NULL,
    cpf_hmac       CHAR(64)     NOT NULL UNIQUE,
    password_hash  TEXT         NOT NULL,
    role           VARCHAR(20)  NOT NULL CHECK (role IN ('producer', 'institution', 'admin')),
    mfa_enabled    BOOLEAN      NOT NULL DEFAULT true,
    status         VARCHAR(20)  NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX users_phone_idx ON users (phone);
CREATE TRIGGER users_set_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE otp_codes (
    id           UUID PRIMARY KEY,
    user_id      UUID         NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    purpose      VARCHAR(20)  NOT NULL CHECK (purpose IN ('login', 'recovery')),
    code_hash    CHAR(64)     NOT NULL,
    expires_at   TIMESTAMPTZ  NOT NULL,
    consumed_at  TIMESTAMPTZ,
    attempts     INT          NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX otp_codes_pending_idx ON otp_codes (user_id, purpose, created_at DESC)
    WHERE consumed_at IS NULL;
CREATE TRIGGER otp_codes_set_updated_at BEFORE UPDATE ON otp_codes
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE refresh_tokens (
    id           UUID PRIMARY KEY,
    user_id      UUID         NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash   CHAR(64)     NOT NULL UNIQUE,
    expires_at   TIMESTAMPTZ  NOT NULL,
    revoked_at   TIMESTAMPTZ,
    device       VARCHAR(120),
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX refresh_tokens_user_idx ON refresh_tokens (user_id) WHERE revoked_at IS NULL;
CREATE TRIGGER refresh_tokens_set_updated_at BEFORE UPDATE ON refresh_tokens
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
