-- Instituição, assinatura, acesso a relatório, pagamentos e pool (README §6).

CREATE TABLE institutions (
    id           UUID PRIMARY KEY,
    user_id      UUID         NOT NULL UNIQUE REFERENCES users (id),
    name         VARCHAR(160) NOT NULL,
    cnpj_hmac    CHAR(64)     NOT NULL UNIQUE,
    status       VARCHAR(20)  NOT NULL CHECK (status IN ('pending', 'approved', 'rejected')),
    approved_at  TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TRIGGER institutions_set_updated_at BEFORE UPDATE ON institutions
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE subscriptions (
    id              UUID PRIMARY KEY,
    institution_id  UUID         NOT NULL REFERENCES institutions (id),
    plan            VARCHAR(20) NOT NULL CHECK (plan IN ('regional', 'national')),
    regions         UUID[],
    period_start    TIMESTAMPTZ NOT NULL,
    period_end      TIMESTAMPTZ NOT NULL,
    status          VARCHAR(20) NOT NULL CHECK (status IN ('pending', 'active', 'expired', 'cancelled')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX subscriptions_institution_idx ON subscriptions (institution_id, status);
CREATE TRIGGER subscriptions_set_updated_at BEFORE UPDATE ON subscriptions
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE report_access (
    id               UUID PRIMARY KEY,
    institution_id   UUID         NOT NULL REFERENCES institutions (id),
    subscription_id  UUID         NOT NULL REFERENCES subscriptions (id),
    cycle_id         UUID         NOT NULL REFERENCES cycles (id),
    accessed_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX report_access_month_idx ON report_access (accessed_at);
CREATE INDEX report_access_cycle_idx ON report_access (cycle_id);

CREATE TABLE payments (
    id               UUID PRIMARY KEY,
    subscription_id  UUID           NOT NULL REFERENCES subscriptions (id),
    amount           NUMERIC(14, 2) NOT NULL,
    provider         VARCHAR(20)   NOT NULL,
    provider_ref     VARCHAR(80)    NOT NULL UNIQUE,
    pool_tx          VARCHAR(90),
    confirmed_at     TIMESTAMPTZ,
    created_at       TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ   NOT NULL DEFAULT now()
);
CREATE TRIGGER payments_set_updated_at BEFORE UPDATE ON payments
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE pool_periods (
    id              UUID PRIMARY KEY,
    month           CHAR(7)         NOT NULL UNIQUE,
    gross           NUMERIC(18, 6) NOT NULL DEFAULT 0,
    infra_cost      NUMERIC(18, 6) NOT NULL DEFAULT 0,
    maintainer_fee  NUMERIC(18, 6) NOT NULL DEFAULT 0,
    net             NUMERIC(18, 6) NOT NULL DEFAULT 0,
    status          VARCHAR(20)    NOT NULL CHECK (status IN ('open', 'distributed', 'carried')),
    created_at      TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ   NOT NULL DEFAULT now()
);
CREATE TRIGGER pool_periods_set_updated_at BEFORE UPDATE ON pool_periods
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE payouts (
    id              UUID PRIMARY KEY,
    pool_period_id UUID           NOT NULL REFERENCES pool_periods (id),
    wallet_id       UUID           NOT NULL REFERENCES wallets (id),
    cycle_id        UUID           NOT NULL REFERENCES cycles (id),
    amount_usdc     NUMERIC(18, 6) NOT NULL,
    tx              VARCHAR(90)    NOT NULL,
    created_at      TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ   NOT NULL DEFAULT now()
);
CREATE INDEX payouts_wallet_idx ON payouts (wallet_id);
CREATE TRIGGER payouts_set_updated_at BEFORE UPDATE ON payouts
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
