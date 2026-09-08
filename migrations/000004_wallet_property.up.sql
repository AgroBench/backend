-- Wallet (só pubkey + blob cifrado) e propriedades (CAR só como HMAC). README §6.

CREATE TABLE wallets (
    id              UUID PRIMARY KEY,
    user_id         UUID         NOT NULL UNIQUE REFERENCES users (id),
    pubkey         VARCHAR(64)  NOT NULL UNIQUE,
    encrypted_blob  BYTEA        NOT NULL,
    blob_version    INT          NOT NULL DEFAULT 1,
    exported_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TRIGGER wallets_set_updated_at BEFORE UPDATE ON wallets
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE properties (
    id               UUID PRIMARY KEY,
    user_id          UUID         NOT NULL REFERENCES users (id),
    car_hmac         CHAR(64)     NOT NULL UNIQUE,
    micro_region_id  UUID         NOT NULL REFERENCES micro_regions (id),
    car_status       VARCHAR(20)  NOT NULL CHECK (car_status IN ('pending', 'approved', 'rejected')),
    verified_at      TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX properties_user_idx ON properties (user_id);
CREATE TRIGGER properties_set_updated_at BEFORE UPDATE ON properties
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
