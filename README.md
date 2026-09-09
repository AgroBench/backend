# AgroBench — Backend

API em Go do AgroBench: coleta de dados agronômicos com commit-reveal, validação em ambiente
isolado (TEE), agregação regional, atestação e recompensa on-chain, e painel de benchmarking.

Este documento é a **fonte de verdade** de arquitetura, decisões e contratos do backend. É
autocontido: nada aqui depende de outro arquivo. O [PLANO.md](PLANO.md) contém apenas o roadmap
de implementação e aponta para cá; os comentários do código também.

Referências de seção usadas no código: `README §2` (decisões), `§5` (ports e mock × real),
`§6` (modelo de dados), `§14` (limitações). Quando um comentário citar o README **da raiz**
(`../README.md`), ele diz explicitamente "README raiz" — aquele é o documento de produto/pitch.

Sumário:

1. [Ambiente de desenvolvimento](#1-ambiente-de-desenvolvimento)
2. [Decisões arquiteturais](#2-decisões-arquiteturais)
3. [Stack e bibliotecas](#3-stack-e-bibliotecas)
4. [Estrutura de pastas](#4-estrutura-de-pastas)
5. [Ports externas e convenção mock × real](#5-ports-externas-e-convenção-mock--real)
6. [Modelo de dados](#6-modelo-de-dados)
7. [Endpoints](#7-endpoints)
8. [Fluxos principais](#8-fluxos-principais)
9. [Jobs](#9-jobs)
10. [Autenticação](#10-autenticação)
11. [Configuração](#11-configuração)
12. [Testes](#12-testes)
13. [Seeds](#13-seeds)
14. [Limitações assumidas](#14-limitações-assumidas)

---

## 1. Ambiente de desenvolvimento

**Requisitos**

- Docker + Docker Compose (o ambiente de dev roda 100% em container)
- Go 1.26 só se quiser rodar `make build` / `make test` fora do Docker

**Subindo**

```bash
cp .env.example .env      # ajuste os segredos
make dev                  # api com hot-reload (Air) + Postgres 17
```

- API: http://localhost:8080 — `GET /healthz`, `GET /readyz`
- Postgres: `localhost:5432`, user/senha/db `agrobench` (exposto pro host, DBeaver etc.)
- Migrations rodam automaticamente no boot em dev (`database.auto_migrate: true`)

**Composição** — `docker-compose.yaml` tem dois serviços: `api` (build de `Dockerfile.dev`:
golang:1.26 + air + migrate CLI, volume `.:/api`, porta 8080, depende de `db` healthy) e `db`
(`postgres:17-alpine`, volume nomeado, healthcheck com `pg_isready`).

**Imagem de produção** — `Dockerfile` multi-stage: builder `golang:1.26-alpine` com
`CGO_ENABLED=0`, imagem final `scratch` com tzdata + ca-certs + binário. As migrations vão
embutidas no binário (`embed` + `iofs` do golang-migrate), sem CLI externa.

**Hot-reload** — `.air.toml` compila `./cmd` para `./dev/main` e roda `./dev/main serve`,
ignorando `dev/`, `docs/`, `migrations/`, `seeds/`.

**Makefile** (`make help` lista tudo)

| Target | Faz |
|---|---|
| `make dev` / `dev-d` | sobe api + db (foreground / detached) |
| `make down` / `down-v` | derruba (e apaga o volume do banco) |
| `make logs` / `sh` | logs da api / shell no container |
| `make build` / `build-image` / `run` | binário em `bin/agrobench` / imagem / roda local |
| `make migrate-up` / `migrate-down` / `migrate-version` / `migrate-create name=x` | migrations |
| `make seed` / `seed-demo` | dados base / dados da demo |
| `make test` / `test-integration` | unit (`-short`) / integração (testcontainers) |
| `make lint` / `fmt` / `tidy` | golangci-lint / gofmt / go mod tidy |

---

## 2. Decisões arquiteturais

| Tema | Decisão |
|---|---|
| Escopo | MVP para demo de banca. **Solana Devnet está ativa** (Memo + USDC-SPL). TEE, SICAR, CONAB, SMS e pagamento seguem mock; adapters reais ao lado |
| Linguagem / versão | Go 1.26 (módulo `github.com/AgroBench/backend`) |
| Idioma | Identificadores, tabelas, rotas e JSON em **inglês**. Comentários, docs e mensagens de erro em português |
| Layout do repo | Monorepo: `AgroBench/backend/` com `go.mod`, `Makefile` e `docker-compose.yaml` próprios |
| Arquitetura | Ports & Adapters no padrão cortex-go (contract / domain / types / repository / usecase / handler / di / rest), um arquivo por operação, DI manual, **sem** deps privadas `cgisoftware/initializers` |
| HTTP | chi v5 + cors + httprate |
| Banco | PostgreSQL 17, sqlx + driver pgx v5 |
| Migrations | golang-migrate (SQL `.up`/`.down`), rodadas via `make` e no boot da API em dev |
| IDs | UUID v7 (`google/uuid`), gerados na aplicação |
| Jobs / fila | River (fila durável em Postgres) — sem Redis. Cron embutido pro split mensal |
| Config / log | viper (json + env override) + `log/slog` |
| Auth | CPF → HMAC-SHA256 com pepper (determinístico, indexável). Senha → argon2id. MFA por SMS (OTP). Access JWT curto + refresh opaco longo no Postgres |
| Wallet | App gera a keypair; backend guarda só `pubkey` + blob cifrado para recuperação. Backend **nunca** vê a chave privada |
| Cripto do payload | NaCl sealed box (x25519 + XSalsa20-Poly1305). Commit = SHA-256 do JSON canônico |
| TEE real | AWS Nitro Enclaves (host via vsock + attestation document). Binário do enclave em `cmd/enclave` |
| Chain | **Ativa na Devnet.** Solana via `gagliardetto/solana-go` + programa Anchor `agrobench` (`EytN8UaXrfTQc6Pq4AdQbQyJwUX37ddXsV7URayBBLrN`). Commit/atestação/CAR em Memo; stake (`lock_stake`/`release_stake`), crédito e split do pool on-chain; recompensa USDC-SPL da treasury. |
| Pagamento real | Stripe (`stripe-go`, checkout + webhook) semi-pronto |
| Ciclo | Ciclo = safra por cultura × região, aberto/fechado pelo admin. Fechamento dispara agregação. Split do pool é job mensal independente |
| Contribuição | 1 por wallet por ciclo. Rejeitada permite **1 nova tentativa** no mesmo ciclo; stake fica travado até a nova validação |
| Stake | Devolvido após **2 ciclos consecutivos** aceitos. Rejeição zera a contagem |
| Painel grátis | Liberado após **3 ciclos consecutivos** aceitos (mesma cultura × região, ordenados por `opens_at`). Só nível básico |
| Instituição | Auto-cadastro com CNPJ → status `pending` → admin aprova → pode assinar |
| Planos | `regional` (até 5 microrregiões escolhidas na assinatura) e `national` (sem restrição). Preços em config |
| Split do pool | `net` do mês dividido entre agregados **proporcionalmente às consultas registradas** (`report_access`); dentro do agregado, por `contribution_weights` |
| Payload | Structs Go por nível + JSON canônico em `internal/core/payload`, compartilhado por API e enclave. Documentado em `docs/payload.md` |
| Tokens | JWT HS256, access 15 min, refresh opaco 30 dias com rotação, OTP 5 min |
| Economia (defaults) | Recompensa base 5 USDC · multiplicadores 1×/2×/3.5× · stake 10 USDC · taxa mantenedores 15% · infra 2 USDC/contribuição |
| Externos | SICAR e CONAB: port + mock lendo tabelas seed + adapter HTTP real esboçado com TODO nos endpoints |
| Observabilidade | slog estruturado + request-id + `/healthz` + `/readyz`. Sem OTel por agora |
| Docs de API | Coleção Bruno versionada em `docs/bruno/` |
| Testes | Unit com mocks das ports + integração com testcontainers-go (Postgres real) |

### Por que o CPF usa HMAC e não argon2id

Argon2id é salteado e não-determinístico, então **não serve para lookup** (recuperar conta por
CPF, checar se o CAR já está vinculado a outro CPF). Por isso o CPF usa HMAC-SHA256 com pepper
em variável de ambiente: irreversível sem o pepper, mas indexável. Argon2id fica só na senha,
que é verificada e nunca buscada.

---

## 3. Stack e bibliotecas

| Lib | Uso |
|---|---|
| `github.com/go-chi/chi/v5`, `go-chi/cors`, `go-chi/httprate` | Router, CORS, rate limit |
| `github.com/jmoiron/sqlx` + `github.com/jackc/pgx/v5/stdlib` | Acesso ao Postgres |
| `github.com/golang-migrate/migrate/v4` | Migrations (lib + CLI na imagem dev) |
| `github.com/riverqueue/river` + `riverdriver/riverpgxv5` | Fila de jobs e cron |
| `github.com/spf13/viper`, `github.com/spf13/cobra` | Config e CLI (`serve`, `migrate`, `seed`, `worker`) |
| `github.com/google/uuid` | UUID v7 |
| `github.com/golang-jwt/jwt/v5` | Access token |
| `golang.org/x/crypto/argon2`, `crypto/hmac` | Senha e CPF |
| `golang.org/x/crypto/nacl/box` | Sealed box do payload |
| `github.com/go-playground/validator/v10` | Validação de input |
| `github.com/booscaaa/go-paginate/v3` | Paginação (mesmo padrão cortex) |
| `github.com/gagliardetto/solana-go` | Adapter real Solana |
| `github.com/mdlayher/vsock`, `github.com/fxamacker/cbor/v2`, `github.com/veraison/go-cose` | Adapter real Nitro (vsock + attestation doc) |
| `github.com/stripe/stripe-go` | Adapter real de pagamento |
| `github.com/testcontainers/testcontainers-go/modules/postgres` | Testes de integração |
| `github.com/stretchr/testify` | Asserts e mocks |

---

## 4. Estrutura de pastas

```
backend/
├── cmd/
│   ├── main.go
│   ├── cmd/
│   │   ├── root.go
│   │   ├── serve.go            # API HTTP + workers River no mesmo processo (dev)
│   │   ├── worker.go           # só workers (prod, opcional)
│   │   ├── migrate.go          # up / down / create
│   │   └── seed.go             # dados de demo
│   └── enclave/
│       └── main.go             # binário que roda DENTRO do Nitro Enclave (real)
├── internal/
│   ├── apperrors/              # erro tipado, catálogo e mapeamento HTTP
│   ├── core/
│   │   ├── config/             # viper + slog
│   │   ├── auth/               # jwt, argon2, hmac-cpf, middleware, claims no context
│   │   ├── crypto/             # sealed box, ed25519, HMAC+pepper, argon2id
│   │   ├── payload/            # structs por nível (basic/intermediate/advanced), validação, JSON canônico + sha256
│   │   ├── validate/           # motor de validação (o MESMO código no enclave mock e no real)
│   │   ├── httpx/              # decode/encode JSON, validator, request-id, logger
│   │   └── domain/             # Role, Money, MicroRegion, Culture, ContributionLevel
│   ├── testutil/               # Postgres via testcontainers
│   ├── identity/               # cadastro, login, MFA, refresh, recuperação
│   ├── wallet/                 # pubkey + blob cifrado, export
│   ├── car/                    # verificação de CAR (oráculo SICAR)
│   ├── cycle/                  # ciclos por cultura × região (admin)
│   ├── contribution/           # commit + reveal + staking
│   ├── validation/             # job: decifra no enclave, cruza referência, aceita/rejeita
│   ├── aggregation/            # job: média/mediana por ciclo × cultura × região × nível
│   ├── attestation/            # atestação on-chain + recompensa base
│   ├── benchmark/              # painel do produtor (grátis) e da instituição (pago)
│   ├── institution/            # instituições, planos, assinaturas, pagamentos
│   ├── pool/                   # pool de receita, split mensal, ledger de payouts
│   └── admin/                  # endpoints operacionais da demo
├── pkg/
│   ├── adapter/
│   │   ├── registry/           # escolhe mock × real por config e loga a escolha no boot
│   │   ├── database/           # conexão sqlx, tx helper
│   │   ├── rest/               # rest.go: middlewares globais, health e registro dos routers
│   │   ├── queue/              # client River, registro de workers, cron
│   │   ├── chain/{mock,solana}/
│   │   ├── enclave/{mock,nitro}/
│   │   ├── sicar/{mock,http}/
│   │   ├── refdata/{mock,conab}/
│   │   ├── sms/{mock,twilio}/
│   │   └── payment/{mock,stripe}/
│   └── port/                   # interfaces das ports externas (ChainClient, Enclave, ...)
├── migrations/                 # SQL embutido no binário. Tabelas mock_* só existem para os mocks
├── seeds/                      # sql de demo: culturas, microrregiões, CARs válidos, faixas CONAB
├── docs/
│   ├── bruno/                  # coleção Bruno por módulo
│   └── payload.md              # formato do payload por nível e regra do hash canônico (pro app)
├── config/env/config.dev.json
├── .air.toml
├── .env.example
├── Dockerfile                  # multi-stage, scratch
├── Dockerfile.dev              # golang + air + migrate
├── docker-compose.yaml         # api + postgres
├── Makefile
└── go.mod
```

Cada módulo em `internal/<modulo>/` segue o padrão cortex: `contract/`, `domain/`,
`types/input`, `types/output`, `repository/`, `usecase/`, `handler/`, `di/`, `rest/router.go`.
Módulos que são só job (validation, aggregation, pool split) têm `worker/` no lugar de
`handler/`.

---

## 5. Ports externas e convenção mock × real

Toda dependência externa tem uma interface em `pkg/port` e **duas** implementações. O que está
ativo é decidido em `config.dev.json` → `adapters.*` e aparece no log do boot
(`adapter selecionado ... mock=true|false`). Na demo, **só a chain é real** (Solana Devnet);
o resto permanece `mock`.

```
pkg/port/<nome>.go                          ← interface + comentário obrigatório
pkg/adapter/<nome>/mock/<nome>.go           ← testes / ports ainda mock (enclave, SICAR, …)
pkg/adapter/<nome>/<real>/<nome>.go         ← integração real (chain = solana, ativa)
```

Comentário obrigatório no topo de cada interface:

```go
// ChainClient abstrai a blockchain Solana.
//
// Implementações:
//   - pkg/adapter/chain/mock   → testes e seed-demo (Postgres).
//   - pkg/adapter/chain/solana → ATIVA NA DEMO. Devnet via gagliardetto/solana-go.
//
// A seleção é feita em pkg/adapter/registry pela config `adapters.chain` (mock|solana).
type ChainClient interface { ... }
```

`pkg/adapter/registry` é o **único** lugar que decide mock × real, uma chave por port:

```json
"adapters": {
  "chain": "solana", "enclave": "mock", "sicar": "mock",
  "reference_data": "mock", "sms": "mock", "payment": "mock"
}
```

| Port | Métodos principais | Mock | Real (demo) |
|---|---|---|---|
| `ChainClient` | `Commit`, `Attest`, `RecordCARVerification`, `TransferUSDC`, `LockStake / ReleaseStake`, `Balance`, `Treasury()`, `Pool()`, `InitializeProgram` | Postgres (`mock_chain_*`). Só nos testes | **Ativa.** Devnet: Memo pra commit/atestação/CAR; programa Anchor pra lock/release/credit/distribute (USDC na PDA); USDC-SPL da treasury (cria ATA se preciso). Fallback Memo só se `CHAIN_SOLANA_PROGRAM_ID` estiver vazio |
| `Enclave` | `Identity()`, `Validate(req) → Verdict assinado` | Decifra em processo com x25519 da config (ou gerada no boot), assina veredito com ed25519. Motor de regras em `internal/core/validate`, o MESMO do enclave real | Host fala com `cmd/enclave` via vsock; verifica attestation document (COSE_Sign1 ES384, cadeia até a raiz AWS, PCRs, user_data = chaves públicas) antes de confiar. `cmd/enclave` gera chaves em memória e pede atestação ao NSM |
| `SicarClient` | `Lookup(car) → {exists, active, uf, ibge}` | Tabela seed `mock_sicar_cars` | Client HTTP com `pathTemplate` e `apiResponse` placeholders — `TODO(sicar)` no endpoint |
| `ReferenceDataClient` | `ExpectedRanges(culture, ibge) → [{metric, min, max}]` | Tabela seed `mock_reference_ranges` | Client HTTP CONAB convertendo média ± tolerância em faixa — `TODO(conab)` no endpoint |
| `SmsSender` | `Send(phone, text)` | Loga no slog e guarda outbox em memória (`LastTo`) | Twilio Messages API por HTTP direto, sem SDK |
| `PaymentGateway` | `CreateCheckout(req) → {ref, url}`, `ParseWebhook(body, sig) → PaymentEvent` | Checkout retorna URL fake; endpoint admin "confirmar pagamento" monta o webhook com `BuildWebhook` | stripe-go v83: Checkout Session (pagamento único) + `webhook.ConstructEvent` em `checkout.session.completed` |

---

## 6. Modelo de dados

PostgreSQL 17. Tipos monetários em `NUMERIC(14,2)`; valores USDC em `NUMERIC(18,6)`. Todas as
tabelas com `id UUID PK` (v7 gerado na aplicação), `created_at`, `updated_at`.

**Identidade**
- `users` — `email`, `phone`, `cpf_hmac` (unique), `password_hash`, `role` (producer|institution|admin), `mfa_enabled`, `status`
- `otp_codes` — `user_id`, `purpose` (login|recovery), `code_hash`, `expires_at`, `consumed_at`
- `refresh_tokens` — `user_id`, `token_hash`, `expires_at`, `revoked_at`, `device`

**Wallet e propriedade**
- `wallets` — `user_id` (unique), `pubkey` (unique), `encrypted_blob`, `blob_version`, `exported_at`
- `properties` — `user_id`, `car_hmac` (unique), `micro_region_id`, `car_status` (pending|approved|rejected), `verified_at`
- `micro_regions` — `ibge_code`, `name`, `uf`
- `cultures` — `code`, `name`

**Ciclo e contribuição**
- `cycles` — `culture_id`, `micro_region_id`, `label` ('2025/26'), `opens_at`, `closes_at`, `status` (open|closed|aggregated)
- `contributions` — `cycle_id`, `wallet_id`, `property_id`, `level` (basic|intermediate|advanced), `attempt` (1|2), `commit_hash`, `commit_tx`, `commit_at`, `ciphertext` (bytea), `reveal_at`, `status` (committed|revealed|validating|accepted|rejected), `reject_reason`
  - unique parcial `(cycle_id, wallet_id) WHERE status <> 'rejected'` — garante 1 ativa por ciclo
  - `attempt = 2` só é permitido se existir uma `rejected` no mesmo ciclo
- `stakes` — `contribution_id`, `amount_usdc`, `lock_tx`, `release_tx`, `status`
- `validation_verdicts` — `contribution_id`, `verdict`, `checks` (jsonb, só nome do check e pass/fail), `enclave_signature`, `enclave_pubkey`
- `validated_metrics` — `contribution_id`, `metric`, `value` — **ponto de exposição assumido do MVP** (§14). A migration e o struct levam comentário explicando a limitação
- `attestations` — `contribution_id`, `tx`, `reward_usdc`, `paid_at`

**Agregação e painel**
- `aggregates` — `cycle_id`, `level`, `metric` (fertilizer_cost_ha, yield_sc_ha, ...), `mean`, `median`, `p25`, `p75`, `n`
- `contribution_weights` — `cycle_id`, `wallet_id`, `weight` (granularidade × raridade), usado pelo split

**Instituição e pool**
- `institutions` — `user_id`, `name`, `cnpj_hmac` (unique), `status` (pending|approved|rejected), `approved_at`
- `subscriptions` — `institution_id`, `plan` (regional|national), `regions` (uuid[] — só no regional, máx. `plans.regional.max_regions` = 5), `period_start`, `period_end`, `status`
- `report_access` — `institution_id`, `subscription_id`, `cycle_id`, `accessed_at` — uma linha por `GET /benchmark/report`; base do split mensal
- `payments` — `subscription_id`, `amount`, `provider`, `provider_ref`, `pool_tx`, `confirmed_at`
- `pool_periods` — `month`, `gross`, `infra_cost`, `maintainer_fee`, `net`, `status` (open|distributed|carried)
- `payouts` — `pool_period_id`, `wallet_id`, `cycle_id`, `amount_usdc`, `tx`

**Infra**
- `mock_chain_events`, `mock_chain_balances`, `mock_chain_stakes` — usadas só pelo adapter mock de chain
- `mock_sicar_cars`, `mock_reference_ranges` — seeds dos mocks de SICAR e CONAB
- O prefixo `mock_` é **obrigatório** em toda tabela que só existe para um adapter mock
- Tabelas do River (`river_job`, ...) via migration própria da lib

**O que nunca fica em texto puro no banco**: CPF, CAR, CNPJ (só HMAC), payload do produtor (só
ciphertext), chave privada da wallet (só blob cifrado pelo app).

---

## 7. Endpoints

Prefixo `/api/v1`. Auth via `Authorization: Bearer <jwt>`. Roles: `producer`, `institution`,
`admin`. Coleção Bruno em `docs/bruno/`. Contrato de consumo HTTP para o app:
[docs/frontend-api.md](docs/frontend-api.md).

**identity**
- `POST /auth/register` — email, phone, cpf, password → cria user + dispara OTP
- `POST /auth/login` — email + password → `mfa_token` + envia OTP
- `POST /auth/mfa/verify` — mfa_token + code → access + refresh
- `POST /auth/refresh`, `POST /auth/logout`
- `POST /auth/recovery/start` (cpf + phone) / `POST /auth/recovery/confirm` (code + nova senha)
- `GET /me`

**wallet** (producer)
- `POST /wallet` — pubkey + encrypted_blob (onboarding)
- `GET /wallet` — pubkey, saldo USDC (via ChainClient), recompensa acumulada
- `GET /wallet/export` — devolve o blob cifrado (exige OTP)

**car** (producer)
- `POST /property` — car + micro_region → dispara verificação SICAR (síncrona no mock)
- `GET /property`

**cycle** (leitura pública / escrita admin)
- `GET /cycles?culture=&region=&status=`
- `POST /admin/cycles`, `POST /admin/cycles/{id}/close` → enfileira `aggregate_cycle`

**contribution** (producer)
- `POST /contributions/commit` — cycle_id, level, hash → `ChainClient.Commit`, `LockStake`
- `POST /contributions/{id}/reveal` — ciphertext (base64) → enfileira `validate_contribution`
- `GET /contributions`, `GET /contributions/{id}` — status, veredito (sem detalhes internos)
- `GET /enclave/public-key` — x25519 pubkey + attestation (mock devolve doc fake)

**benchmark**
- `GET /benchmark/me?cycle=` (producer) — só nível básico, só a própria região/cultura, exige 3 ciclos consecutivos validados. Caso contrário 403 com `cycles_validated` e `cycles_required`
- `GET /benchmark/report?cycle=&region=&culture=` (institution com assinatura ativa) — todos os níveis. Plano regional fora do escopo → 403. Cada chamada grava `report_access`

**institution**
- `POST /institutions/register` — email, senha, CNPJ, nome → user `institution` + institution `pending` (sem MFA por SMS; login só senha)
- `GET /institutions/me`
- `POST /admin/institutions/{id}/approve` / `reject`
- `POST /institutions/subscribe` — `plan` + `regions[]` (regional) → `PaymentGateway.CreateCheckout`. Exige institution `approved`
- `POST /webhooks/payment` — webhook do gateway (real)
- `POST /admin/payments/{id}/confirm` — simula o webhook (mock)

**pool** (admin / leitura pública)
- `GET /pool/periods`, `GET /pool/periods/{month}` — gross, custos, taxa, net, payouts
- `POST /admin/pool/distribute?month=` — força o job mensal (a demo não espera o mês virar)
- `GET /wallet/payouts` (producer)

**admin (demo)**
- `POST /admin/seed/demo` — seed da banca: logins oficiais, agricultores extras (média), ciclos e Cotrijal
- `GET /admin/otp/{user_id}` — último OTP pendente. **Só existe quando `adapters.sms = mock`**
- `POST /admin/jobs/run/{kind}` — dispara qualquer job manualmente

**infra**
- `GET /healthz`, `GET /readyz`

---

## 8. Fluxos principais

**Onboarding**
1. `register` → user com `cpf_hmac`, OTP enviado (mock: código aparece no log e em `otp_codes`)
2. `mfa/verify` → tokens
3. App gera keypair → `POST /wallet` com pubkey + blob cifrado
4. `POST /property` com CAR → `SicarClient.Lookup` → checa `car_hmac` único → `approved`
5. `ChainClient` registra o resultado da verificação (evento `car_verified`, sem o CAR)

**Commit-reveal → validação → atestação**
1. `commit`: valida ciclo aberto, property aprovada, level ∈ {basic, intermediate, advanced},
   nenhuma contribuição ativa no ciclo (ou 1 rejeitada → `attempt = 2`);
   `ChainClient.Commit(hash)`; `LockStake(amount)` só na primeira tentativa; status `committed`
2. `reveal`: recebe ciphertext; o `sha256(canonical(plaintext))` só será conferido **dentro do
   enclave**; status `revealed`; enfileira `validate_contribution`
3. Worker `validate_contribution`: `Enclave.Validate(ciphertext, refs)` → dentro do enclave:
   decifra, recalcula o hash e compara com o commit, valida o schema do nível, compara com
   `ReferenceDataClient.ExpectedRange` → veredito assinado. Fora do enclave só chega
   `{accepted|rejected, checks[], signature}` e os valores agregáveis já anonimizados (métricas
   numéricas sem identificador), que vão para `validated_metrics` (§14)
4. Se aceito: `ChainClient.Attest(...)`, `TransferUSDC(reward_base × multiplicador do nível)`,
   grava `attestations`; `ReleaseStake` quando completar 2 ciclos consecutivos aceitos
5. Se rejeitado: `reject_reason` genérico, a contagem de consecutivos zera, o stake é mantido.
   O produtor pode fazer 1 novo commit no mesmo ciclo

**Fechamento de ciclo → agregação**
1. Admin `close` → status `closed`, enfileira `aggregate_cycle`
2. Worker calcula mean/median/p25/p75/n por métrica e nível em `aggregates`; calcula
   `contribution_weights` (peso do nível × 1/n do agregado); status `aggregated`
3. O painel do produtor passa a responder para esse ciclo

**Pool mensal**
1. Pagamento confirmado → `ChainClient.TransferUSDC(pool)` → `payments.pool_tx`
2. Cron River (dia 1) ou `POST /admin/pool/distribute`: soma pagamentos confirmados do mês →
   `gross`; desconta `infra_cost` (gas registrado + `infra_cost_per_contribution` ×
   contribuições validadas no mês) e `maintainer_fee_pct` → `net`
3. Lê `report_access` do mês agrupado por `cycle_id`: fatia do agregado =
   `net × acessos_do_ciclo / total_acessos`. Ciclo sem acesso não recebe. Mês sem acesso nenhum:
   `net` fica acumulado (`pool_periods.status = carried`) e entra no mês seguinte
4. Dentro de cada ciclo, divide a fatia por `contribution_weights` (peso do nível: 1/2/3.5,
   normalizado entre as wallets aceitas do ciclo); `TransferUSDC` por wallet; grava `payouts`

---

## 9. Jobs

Fila durável River, em Postgres — sem Redis.

| Job | Disparo | Idempotência |
|---|---|---|
| `validate_contribution` | reveal | unique por `contribution_id` |
| `aggregate_cycle` | close do ciclo | unique por `cycle_id` |
| `distribute_pool` | cron mensal + admin | unique por `month` |
| `verify_car` | criação de property (real é assíncrono; mock resolve na hora) | unique por `property_id` |

Em dev, `serve` sobe API e workers no mesmo processo. `worker` existe pra separar em prod.

---

## 10. Autenticação

- **Senha**: argon2id (t=3, m=64MB, p=2), salt de 16 bytes, formato PHC string
- **CPF/CAR/CNPJ**: `HMAC-SHA256(pepper, digits)` em hex; o pepper vem de `AUTH_PEPPER` (env),
  nunca do json de config (o porquê está em §2)
- **OTP**: 6 dígitos, hash sha256 em `otp_codes`, expira em 5 min, 5 tentativas
- **Access JWT**: HS256 com `JWT_SECRET`, 15 min, claims `sub`, `role`, `wallet` (pubkey), `jti`
- **Refresh**: 32 bytes random, sha256 no banco, 30 dias, rotação a cada uso, revogação em
  cascata se reuso for detectado
- **MFA**: obrigatório para `producer` (SMS). `institution` e `admin` logam só com senha no MVP
- **Middleware**: `auth.Require(roles...)` injeta as claims no context;
  `auth.RequireSubscription()` pro relatório pago (checa status, período e escopo de regiões)

---

## 11. Configuração

`config/env/config.dev.json` é a **fonte de verdade das chaves**. Qualquer chave pode ser
sobrescrita por variável de ambiente trocando `.` por `_` em maiúsculas
(`auth.jwt_secret` → `AUTH_JWT_SECRET`, `database.url` → `DATABASE_URL`). viper lê o json e faz
override com `AutomaticEnv` + `SetEnvKeyReplacer`.

Blocos principais: `server.http`, `database`, `log`, `adapters` (§5), `auth`, e a economia —
`rewards.base_usdc`, `rewards.level_multipliers`, `stake.amount_usdc`,
`stake.release_after_cycles`, `benchmark.free_after_cycles`, `pool.maintainer_fee_pct`,
`pool.infra_cost_per_contribution_usdc`, `plans.regional.{price_usdc,max_regions}`,
`plans.national.price_usdc`. Cada adapter real tem seu próprio bloco (`chain.solana`,
`enclave.nitro`, `sicar.http`, `reference_data.conab`, `sms.twilio`, `payment.stripe`).

**Segredos ficam só no `.env`, nunca no json**: `DATABASE_URL`, `AUTH_JWT_SECRET`, `AUTH_PEPPER`,
`CHAIN_SOLANA_TREASURY_PRIVATE_KEY`, `ENCLAVE_MOCK_*`, `STRIPE_*`, `TWILIO_*`,
`ADMIN_EMAIL`/`ADMIN_PASSWORD`. Ver `.env.example`.

---

## 12. Testes

- **Unit**: usecases com mocks gerados por testify/mock das interfaces em `contract/` e
  `pkg/port/`. Sem banco. `make test`
- **Integração**: `testcontainers-go` sobe `postgres:17-alpine`, roda as migrations e testa os
  repositories e os três fluxos ponta a ponta (onboarding; commit → reveal → validate → attest;
  close → aggregate → distribute) usando os adapters mock. `make test-integration`
- **Crypto**: vetores de teste do sealed box (interop com tweetnacl) e do hash canônico

---

## 13. Seeds

- `micro_regions`: as 35 microrregiões IBGE do RS (código IBGE de 5 dígitos + nome + UF).
  Passo Fundo é a praça do agricultor oficial da demo; a seed rica cobre outras microrregiões do planalto e da serra
- `cultures`: soja, milho, trigo
- `mock_reference_ranges` (CONAB): faixas de custo/ha e produtividade para as 35 microrregiões × 3 culturas
- `mock_sicar_cars` (SICAR): ~20 CARs fictícios em Passo Fundo (1–17 ativos, 18–20 inativos) + CARs dos agricultores extras (`…0100` em diante)
- `users`: 1 admin (`ADMIN_EMAIL`/`ADMIN_PASSWORD` do `.env`)
- `seed-demo`: 3 logins oficiais + ~33 agricultores extras (emails `seed.farmer.NN@agrobench.local`, senha dummy, wallets **off-chain** só para média). Produtor oficial `produtor@agrobench.local` com pubkey Devnet `FXsin7…jpzf3` em Passo Fundo × soja (3 ciclos agregados + janela `2026/27` aberta). Instituição: Cotrijal (plano regional, 5 microrregiões). Não cria `produtor01`–`14`.

---

## 14. Limitações assumidas

Limitações **conscientes** do MVP. Cada uma tem comentário no código apontando para cá.

**1. `validated_metrics` fica fora do enclave.** As métricas numéricas validadas são gravadas no
Postgres ligadas a `contribution_id`, porque a agregação é feita em SQL fora do enclave.
- *O que vaza*: métricas numéricas por contribuição — sem identidade civil, mas ligadas à wallet
  via `contribution_id`. É pseudonimização, não anonimização
- *Como evolui*: agregação dentro do enclave, ou aprendizado federado
- *Onde está documentado no código*: migration da tabela, struct `domain.ValidatedMetric` e a
  port `Enclave.Validate`

**2. O enclave mock não isola nada.** No adapter `pkg/adapter/enclave/mock`, a chave privada
x25519 vive no processo da API. O operador da infra **pode** ler o payload. A separação de
responsabilidades (quem decifra × quem agrega) está preservada na arquitetura, mas não na
garantia — é exatamente isso que o adapter `nitro` resolve.

**3. Chain na Devnet; o resto ainda é mock.** Solana está ativa (`adapters.chain=solana`):
Memo + USDC-SPL da treasury, conferido contra a Devnet. `LockStake`/`ReleaseStake` são Memo
(não travam USDC do produtor — a chave privada nunca chega ao backend). Stripe e Nitro
compilam, mas não estão ligados na demo. O produtor oficial (`produtor@agrobench.local`) tem
pubkey Devnet real. Agricultores extras da seed (`seed.farmer.NN`) têm pubkey off-chain só
para média: `GET /wallet` nessas contas falha/zera na Devnet — não são logins da banca.

**4. Endpoints públicos do SICAR e da CONAB não foram levantados.** Os adapters reais nascem com
client HTTP, structs de resposta e `// TODO(sicar)` / `// TODO(conab)` no lugar da URL.
