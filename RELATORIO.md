# Relatório de testes E2E — AgroBench API

Ambiente: `make dev` · http://localhost:8080 · Postgres agrobench
Data: 2026-09-08

Contrato: `docs/frontend-api.md`

## Ambiente

- healthz: HTTP 200 `{"status":"ok"}`
- readyz: HTTP 200 `{"database":"up","status":"ok"}`
- postgres: `select 1` OK (containers `agrobench-api` e `agrobench-db` já estavam Up ~2h; não foi subido duplicado)

## Índice de endpoints

| Módulo | Endpoint | Resultado | Notas |
|---|---|---|---|
| Identity/Auth | `POST /api/v1/auth/register` | OK | 201 MFA challenge; cpf_hmac 64 hex ≠ CPF; argon2id. Partial: `docs/relatorio-partials/01-auth.md` |
| Identity/Auth | `POST /api/v1/auth/login` | OK | producer → MFA; institution/admin → tokens (`expires_in` 900) |
| Identity/Auth | `POST /api/v1/auth/mfa/verify` | OK | 200 tokens; OTP consumed; refresh SHA-256 no banco |
| Identity/Auth | `POST /api/v1/auth/refresh` | OK | rotaciona; reuse → 401 e revoga família |
| Identity/Auth | `POST /api/v1/auth/logout` | OK | 204; token desconhecido 204; access JWT segue válido |
| Identity/Auth | `POST /api/v1/auth/recovery/start` | OK | 204 sempre (par certo e errado); OTP `purpose=recovery` |
| Identity/Auth | `POST /api/v1/auth/recovery/confirm` | OK | 204; revoga todos os refresh; senha nova + MFA |
| Identity/Auth | `GET /api/v1/me` | OK | producer/institution/admin; sem CPF |
| Identity/Auth | `GET /api/v1/admin/otp/{user_id}` | OK | mock público; 400 id inválido; 404 user inexistente |
| Catálogo | `GET /api/v1/cultures` | OK | público; 200; 3 itens após seed (soybean/corn/wheat). Pré-seed: 200 + `null`. Partial: `docs/relatorio-partials/02-wallet-car.md` |
| Catálogo | `GET /api/v1/micro-regions` | OK | público; 200; 35 RS; Passo Fundo `ibge_code=43010` |
| Wallet | `POST /api/v1/wallet` | OK | 201 pubkey+saldo 0; recusa `private_key` (400); blob inválido 400; duplicata 409 |
| Wallet | `GET /api/v1/wallet` | OK | 404 antes; 200 depois; `exported_at` após export. SQL: pubkey + bytea blob; sem coluna de chave privada |
| Wallet | `GET /api/v1/wallet/export` | OK | sem `code` → 403 `otp_required`; OTP mock purpose `login`; `?code=` → 200 blob base64 |
| Property/CAR | `POST /api/v1/property` | OK | 201 `approved` (SICAR active) / `rejected` (inativo ou inexistente); 409 CAR duplicado; 404 região inexistente. Reteste pepper: 201 approved CAR novo; `car_hmac` ≠ HMAC("", CAR) |
| Property/CAR | `GET /api/v1/property` | OK | 404 antes; 200 a mais recente; JSON sem CAR. SQL: só `car_hmac` 64 hex (não o CAR em claro). Reteste GET 200 approved |
| Cycle | `GET /api/v1/cycles` | OK | público; filtros `culture`/`region` (UUID) e `status`; vazio `[]`. Partial: `docs/relatorio-partials/03-contribution.md` |
| Cycle | `POST /api/v1/admin/cycles` | OK | 201 `open`; admin via SQL `UPDATE users SET role='admin'` num 2º user (`e2e-contrib-adm-1788897980@…`) |
| Cycle | `POST /api/v1/admin/cycles/{id}/close` | OK | 200 `closed`; job aggregate → `aggregated` em <1 s |
| Enclave | `GET /api/v1/enclave/public-key` | OK | 200 `provider=mock`; box x25519 64 hex; `attestation` omitido |
| Contribution | `POST /api/v1/contributions/commit` | OK | Reteste 20:28Z: attempt 1 **201** `committed` + stake FK. 1ª passagem: 409 FK (CreateStake antes de Create) + 502 saldo 0 + attempt 2 via rejected SQL. Duplicata 409; ciclo fechado 409 |
| Contribution | `POST /api/v1/contributions/{id}/reveal` | OK | sealed box NaCl real → 200 `revealed`; não-base64 400; repetido 409 |
| Contribution | `GET /api/v1/contributions` | OK | lista da wallet; by id poll `revealed`→`accepted`; 404 id inexistente |
| Benchmark | `GET /api/v1/benchmark/me` | OK | Reteste: 403 < 3 ciclos; `cycles_validated` **0 e 1** alinhados ao `detail` (`0/3`, `1/3`). Cycle inválido 400. Partial: `docs/relatorio-partials/04-institution-admin.md` |
| Benchmark | `GET /api/v1/benchmark/report` | OK | 200 institution; reteste grava `report_access` com `institutions.id` ≠ `users.id` (1 linha/GET). Rejected/sem plano também 200 (contrato §12) |
| Institution | `POST /api/v1/institutions/register` | OK | 201 pending; sem `user_id`; 409 duplicata; CNPJ curto 400 |
| Institution | `GET /api/v1/institutions/me` | OK | pending→approved→rejected; producer/admin 403 |
| Institution | `POST /api/v1/institutions/subscribe` | OK | 201 checkout mock; pending/rejected 403; regional 0/6 regiões 400 |
| Payment | `POST /api/v1/webhooks/payment` | OK | path `/api/v1/webhooks/payment`; `paid` 204 ativa sub; `status!=paid` 204 ignora |
| Admin | `POST /api/v1/admin/institutions/{id}/approve` | OK | 204; GET me approved; 404/400 |
| Admin | `POST /api/v1/admin/institutions/{id}/reject` | OK | 204; subscribe 403 |
| Admin | `POST /api/v1/admin/payments/{id}/confirm` | OK | 204 (id via SQL; subscribe não devolve `payment_id`) |
| Pool | `GET /api/v1/pool/periods` | OK | vazio `null`; depois array |
| Pool | `GET /api/v1/pool/periods/{month}` | OK | 404 depois 200 |
| Pool | `POST /api/v1/admin/pool/distribute` | OK | Reteste: 202 → período `2026-09` `distributed` gross 3000 net 2546; 1 payout 1697.33. 1ª passagem: `carried` por `report_access` vazio |
| Admin | `POST /api/v1/admin/jobs/run/{kind}` | OK | só `distribute_pool` 202; outro kind 400 `job desconhecido` |
| Wallet | `GET /api/v1/wallet/payouts` | OK | 200 `null` no producer novo do reteste (payout foi à wallet do ciclo soja de outro testador) |

## Falhas

- **Identity/Auth**: nenhuma falha bloqueante. Ver `docs/relatorio-partials/01-auth.md`.
  - `GET /admin/otp` devolve `purpose=login` mesmo após recovery (contrato §4.1 / SMS mock).
  - Logout não invalida o access JWT vigente (só o refresh enviado).
- **Wallet/CAR — AUTH_PEPPER CORRIGIDO** (reteste 2026-09-08): `printenv AUTH_PEPPER` no container `agrobench-api` → presente e não vazio (valor não copiado). Producer novo `e2e-retest-pepper-1788899140@…` + CAR `RS4314100E2ERT1788899140`: `car_hmac` = `9573a3e6…372177` ≠ `HMAC-SHA256("", CAR)` (`8f81f582…bcf636`); bate com HMAC do pepper do processo (openssl no container). GET wallet/property 200. HMACs **antigos** (ex. `29d7d436…` do CAR `RS4314100E2EOK1788897898`) continuam sendo HMAC com pepper `""`. Ver `docs/relatorio-partials/02-wallet-car.md`.
- **Contribution/commit attempt 1 — 409 FK CORRIGIDO** (reteste 2026-09-08 20:28Z): producer novo `e2e-retest-commit-20260908202853@…` (pepper mudou). Sem INSERT `rejected`. `POST /contributions/commit` → **201** `attempt:1` `status:committed`. SQL: 1 contribution + 1 stake com o mesmo `contribution_id`; saldo 50→40 USDC. Retry mesmo ciclo → 409 `já existe contribuição ativa neste ciclo`. Reveal sealed box + poll → `accepted` (reward 5 USDC). Air: `dev/main` 20:20:42Z > `commit.go` 20:19:59Z; logs sem `stakes_contribution_id_fkey`. 1ª passagem (bloqueante): CreateStake antes do Create → 23503 / HTTP 409; work-around SQL `rejected` → attempt 2. Ver `docs/relatorio-partials/03-contribution.md`.
- **`report_access` / split do pool — CORRIGIDO** (reteste 2026-09-08 20:25Z): institution nova `e2e-retest-20260908202640@…`. GET `/benchmark/report` ×3 → 3 rows; `report_access.institution_id` = `institutions.id` (`01a082b4-7681-…`) ≠ `users.id` (`01a082b4-767f-…`). `POST /admin/pool/distribute?month=2026-09` 202; unique River pulou o 1º insert (job completed da 1ª passagem). Após `DELETE river_job` do unique: período **`distributed`** gross 3000 / net 2546 / 1 payout 1697.33 (soja 2/3 dos acessos; trigo 1 acesso sem `contribution_weights` → fatia skip, status não volta a `carried`). 1ª passagem: 0 rows no FK errado → `carried` `totalAcc==0`. Ver `docs/relatorio-partials/04-institution-admin.md`.
- **Painel `cycles_validated` — CORRIGIDO** (reteste): producer `e2e-retest-prod-20260908202640@…`. 403 `0` + `detail` `0/3`; após 1 contribution `accepted` (commit attempt 1 **201** + reveal, ciclo wheat open `E2E08201033` **não fechado**): 403 **`cycles_validated: 1`** / `1/3`. Soja permanece `0/3`. Não é hardcode 0.

## Observações

- Partials de backup em `docs/relatorio-partials/`
- Boot sem erro. Air com hot-reload; adapters mock (chain, enclave, sicar, reference_data, sms, payment). Enclave mock regenera chaves a cada restart do Air — payloads cifrados antes do restart não decifram.
- Catálogo estava vazio no início do teste wallet/CAR; rodei `make seed` (base, aditivo `ON CONFLICT DO NOTHING`, `rows=38`). Não rodei `seed-demo`. Inseri 2 CARs mock exclusivos (`RS4314100E2EOK…` active, `RS4314100E2ERJ…` inactive). Reteste pepper: CAR extra `RS4314100E2ERT1788899140` (active) + producer `e2e-retest-pepper-1788899140@agrobench.test`.
- Contribution E2E (1ª passagem): producer `e2e-contrib-1788897980@agrobench.local`; mint SQL `mock_chain_balances` +50 USDC na pubkey `E2EContribWallet1788897980XXXXXX` (sem mint HTTP). Admin próprio via `UPDATE users SET role='admin', mfa_enabled=false`. Ciphertext sealed box real; worker `accepted` na attempt 2 (work-around FK); reward 5 USDC; checks CONAB `area_ha` + `total_cost_ha` ok.
- Contribution reteste attempt 1: producer `e2e-retest-commit-20260908202853@agrobench.local`; admin `e2e-retest-adm-20260908202853@…`; ciclo próprio `RT08202853` (`01a082b5-9a80-…`, deixado **open**); contribuição `01a082b5-9d1c-…` accepted na attempt 1. **Não** rodei `seed-demo`.
- Institution/admin E2E (1ª passagem): producer `e2e-inst-20260908201033-prod@agrobench.test` (MFA); institution `e2e-inst-20260908201033@agrobench.test` (login = tokens, sem MFA). Cadastro CNPJ → pending → admin aprova → webhook `POST /api/v1/webhooks/payment` → sub `active`. Relatório 200 mas `report_access` 0. Pool `carried`. **Não** rodei `seed-demo`. Ciclos próprios `E2E08201033` (open) e `E2E8201033c` (aggregated); não fechei ciclos de outros.
- Institution/admin **reteste** 20:25Z: institution `e2e-retest-20260908202640@agrobench.test`; admin próprio `e2e-retest-adm-20260908202640@…` via SQL; producer `e2e-retest-prod-20260908202640@…`. `report_access` 3 rows com FK certo. Pool `2026-09` `distributed`. Painel n=0 e n=1. Commit+reveal no ciclo open `E2E08201033` (não fechei). **Não** rodei `seed-demo`.
