-- Dados de referência: microrregiões IBGE e culturas.
-- Tudo em inglês por convenção do projeto (README §2). IDs são UUID v7 gerados na aplicação.

CREATE TABLE micro_regions (
    id          UUID PRIMARY KEY,
    ibge_code   VARCHAR(5)  NOT NULL UNIQUE,   -- código IBGE da microrregião (ex.: 43012 = Passo Fundo)
    name        VARCHAR(120) NOT NULL,
    uf          CHAR(2)      NOT NULL,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX micro_regions_uf_idx ON micro_regions (uf);

CREATE TABLE cultures (
    id          UUID PRIMARY KEY,
    code        VARCHAR(30)  NOT NULL UNIQUE,   -- soybean, corn, wheat
    name        VARCHAR(80)  NOT NULL,          -- nome de exibição em português
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- Trigger genérica de updated_at, reutilizada por todas as tabelas seguintes.
CREATE OR REPLACE FUNCTION set_updated_at() RETURNS trigger AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER micro_regions_set_updated_at BEFORE UPDATE ON micro_regions
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER cultures_set_updated_at BEFORE UPDATE ON cultures
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
