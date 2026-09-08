-- Agregados por ciclo × nível × métrica e pesos para o split (README §6).

CREATE TABLE aggregates (
    id        UUID PRIMARY KEY,
    cycle_id  UUID           NOT NULL REFERENCES cycles (id),
    level     VARCHAR(20)   NOT NULL CHECK (level IN ('basic', 'intermediate', 'advanced')),
    metric    VARCHAR(40)    NOT NULL,
    mean      NUMERIC(14, 4) NOT NULL,
    median    NUMERIC(14, 4) NOT NULL,
    p25       NUMERIC(14, 4) NOT NULL,
    p75       NUMERIC(14, 4) NOT NULL,
    n         INT            NOT NULL,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT now(),
    UNIQUE (cycle_id, level, metric)
);
CREATE TRIGGER aggregates_set_updated_at BEFORE UPDATE ON aggregates
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE contribution_weights (
    id         UUID PRIMARY KEY,
    cycle_id   UUID           NOT NULL REFERENCES cycles (id),
    wallet_id  UUID           NOT NULL REFERENCES wallets (id),
    weight     NUMERIC(10, 6) NOT NULL,
    created_at TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ   NOT NULL DEFAULT now(),
    UNIQUE (cycle_id, wallet_id)
);
CREATE TRIGGER contribution_weights_set_updated_at BEFORE UPDATE ON contribution_weights
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
