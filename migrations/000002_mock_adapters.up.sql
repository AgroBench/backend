-- Tabelas usadas SOMENTE pelos adapters mock (pkg/adapter/*/mock). Prefixo mock_ deixa
-- explícito que nada aqui existe em produção com adapters reais.

-- Blockchain simulada: um evento por operação do port.ChainClient.
CREATE TABLE mock_chain_events (
    id          UUID PRIMARY KEY,
    tx_ref      VARCHAR(90)  NOT NULL UNIQUE,      -- "mock_<sha256 curto>", determinístico por (kind, payload)
    kind        VARCHAR(30)  NOT NULL,             -- commit | attest | car_verification | transfer | stake_lock | stake_release
    wallet      VARCHAR(64),                       -- pubkey base58 envolvida (nullable em transfer)
    payload     JSONB        NOT NULL,             -- campos da operação (hash, level, amount, memo...)
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX mock_chain_events_wallet_idx ON mock_chain_events (wallet, created_at);
CREATE INDEX mock_chain_events_kind_idx   ON mock_chain_events (kind, created_at);

-- Saldos USDC simulados (micro-USDC, 6 casas). Treasury nasce com saldo via seed.
CREATE TABLE mock_chain_balances (
    account     VARCHAR(64)  PRIMARY KEY,
    micro_usdc  BIGINT       NOT NULL DEFAULT 0 CHECK (micro_usdc >= 0),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- Stakes travados por contribuição (o mock guarda para poder devolver o valor certo).
CREATE TABLE mock_chain_stakes (
    contribution_id UUID PRIMARY KEY,
    wallet          VARCHAR(64) NOT NULL,
    micro_usdc      BIGINT      NOT NULL,
    released        BOOLEAN     NOT NULL DEFAULT false,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- SICAR simulado: CARs fictícios. O número fica em texto puro AQUI porque é seed fictícia;
-- no domínio do AgroBench (tabela properties, fase 3) só existe o HMAC.
CREATE TABLE mock_sicar_cars (
    car_number  VARCHAR(60)  PRIMARY KEY,           -- normalizado: só A-Z0-9 (ver crypto.NormalizeIdentifier)
    active      BOOLEAN      NOT NULL DEFAULT true,
    uf          CHAR(2)      NOT NULL,
    ibge_code   VARCHAR(7)   NOT NULL               -- município IBGE (7 dígitos)
);

-- CONAB simulada: faixa esperada por métrica × cultura × microrregião.
CREATE TABLE mock_reference_ranges (
    culture_code VARCHAR(30)  NOT NULL,
    ibge_code    VARCHAR(5)   NOT NULL,             -- microrregião
    metric       VARCHAR(40)  NOT NULL,
    min_value    NUMERIC(14,4) NOT NULL,
    max_value    NUMERIC(14,4) NOT NULL,
    PRIMARY KEY (culture_code, ibge_code, metric),
    CHECK (min_value <= max_value)
);
