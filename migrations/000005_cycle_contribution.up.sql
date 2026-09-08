-- Ciclo, contribuição, stake, veredito, métricas validadas e atestação (README §6).

CREATE TABLE cycles (
    id               UUID PRIMARY KEY,
    culture_id       UUID         NOT NULL REFERENCES cultures (id),
    micro_region_id UUID         NOT NULL REFERENCES micro_regions (id),
    label            VARCHAR(20)  NOT NULL,
    opens_at         TIMESTAMPTZ NOT NULL,
    closes_at       TIMESTAMPTZ NOT NULL,
    status           VARCHAR(20)  NOT NULL CHECK (status IN ('open', 'closed', 'aggregated')),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (culture_id, micro_region_id, label)
);
CREATE INDEX cycles_status_idx ON cycles (status, opens_at);
CREATE TRIGGER cycles_set_updated_at BEFORE UPDATE ON cycles
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE contributions (
    id           UUID PRIMARY KEY,
    cycle_id     UUID         NOT NULL REFERENCES cycles (id),
    wallet_id    UUID         NOT NULL REFERENCES wallets (id),
    property_id  UUID         NOT NULL REFERENCES properties (id),
    level        VARCHAR(20)  NOT NULL CHECK (level IN ('basic', 'intermediate', 'advanced')),
    attempt      SMALLINT     NOT NULL CHECK (attempt IN (1, 2)),
    commit_hash  CHAR(64)     NOT NULL,
    commit_tx    VARCHAR(90)  NOT NULL,
    commit_at    TIMESTAMPTZ NOT NULL,
    ciphertext   BYTEA,
    reveal_at    TIMESTAMPTZ,
    status       VARCHAR(20)  NOT NULL CHECK (status IN ('committed', 'revealed', 'validating', 'accepted', 'rejected')),
    reject_reason TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX contributions_active_wallet_cycle
    ON contributions (cycle_id, wallet_id) WHERE status <> 'rejected';
CREATE INDEX contributions_wallet_idx ON contributions (wallet_id, created_at);
CREATE TRIGGER contributions_set_updated_at BEFORE UPDATE ON contributions
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE stakes (
    id               UUID PRIMARY KEY,
    contribution_id  UUID           NOT NULL REFERENCES contributions (id),
    amount_usdc      NUMERIC(18, 6) NOT NULL,
    lock_tx          VARCHAR(90)   NOT NULL,
    release_tx       VARCHAR(90),
    status           VARCHAR(20)   NOT NULL CHECK (status IN ('locked', 'released')),
    created_at       TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ   NOT NULL DEFAULT now()
);
CREATE INDEX stakes_contribution_idx ON stakes (contribution_id);
CREATE TRIGGER stakes_set_updated_at BEFORE UPDATE ON stakes
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- LIMITAÇÃO ASSUMIDA DO MVP (README §14, limitação 1): métricas numéricas validadas
-- ficam fora do enclave, ligadas a contribution_id. Pseudonimização, não anonimização.
CREATE TABLE validation_verdicts (
    id                 UUID PRIMARY KEY,
    contribution_id   UUID         NOT NULL REFERENCES contributions (id),
    verdict            VARCHAR(20) NOT NULL CHECK (verdict IN ('accepted', 'rejected')),
    checks             JSONB        NOT NULL,
    enclave_signature  BYTEA        NOT NULL,
    enclave_pubkey     VARCHAR(128) NOT NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX validation_verdicts_contribution_idx ON validation_verdicts (contribution_id);
CREATE TRIGGER validation_verdicts_set_updated_at BEFORE UPDATE ON validation_verdicts
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE validated_metrics (
    id               UUID PRIMARY KEY,
    contribution_id UUID           NOT NULL REFERENCES contributions (id),
    metric           VARCHAR(40)   NOT NULL,
    value            NUMERIC(14, 4) NOT NULL,
    created_at       TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ   NOT NULL DEFAULT now()
);
CREATE INDEX validated_metrics_contribution_idx ON validated_metrics (contribution_id);
CREATE TRIGGER validated_metrics_set_updated_at BEFORE UPDATE ON validated_metrics
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE attestations (
    id               UUID PRIMARY KEY,
    contribution_id UUID           NOT NULL UNIQUE REFERENCES contributions (id),
    tx               VARCHAR(90)   NOT NULL,
    reward_usdc      NUMERIC(18, 6) NOT NULL,
    paid_at          TIMESTAMPTZ   NOT NULL,
    created_at       TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ   NOT NULL DEFAULT now()
);
CREATE TRIGGER attestations_set_updated_at BEFORE UPDATE ON attestations
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
