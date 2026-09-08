# E2E Benchmark / Institution / Payment mock / Pool / Admin — 2026-09-08

**Status do módulo**: ✅ APROVADO nos bugs retestados (`report_access` FK e `cycles_validated`); contrato §12 (relatório sem checar assinatura) segue como observação.

**Reteste 20:25–20:32Z (usuários novos — pepper mudou)**: `report_access.institution_id` = `institutions.id` ≠ `users.id`; cada GET `/benchmark/report` +1 linha. Pool `2026-09` **não** ficou `carried` por tabela vazia — após o worker rodar de verdade: `status=distributed`, 1 payout. Painel 403 com `cycles_validated` alinhado ao `detail` (`0/3` e **`1/3`**). Primeira passagem (20:10Z) documentada abaixo.

Fluxo HTTP instituição (cadastro → login sem MFA → pending → admin aprova → checkout mock → webhook `paid` → relatório 200) **passou** nas duas passagens.

**Ambiente**: API `http://localhost:8080` já no ar (`GET /healthz` 200; `GET /readyz` 200). Postgres `docker compose exec -T db psql -U agrobench -d agrobench`. **Não** subi docker. **Não** rodei `make seed-demo`. Catálogo/admin já existiam (`make seed` de outro testador). 204 devolve `Content-Type: application/json` e body vazio.

**Usuários próprios (prefixo `e2e-inst-{timestamp}`)**

| Papel | Email | MFA | IDs |
|---|---|---|---|
| producer | `e2e-inst-20260908201033-prod@agrobench.test` | sim | `user_id` `01a082a4-cdf3-77f3-bf31-21c7f4a5bc3d` |
| institution (aprovada) | `e2e-inst-20260908201033@agrobench.test` | não | institution `01a082a4-d29a-7555-9e68-e55ed99d305b` · user `01a082a4-d294-7e55-aa4e-c5788d2f61bc` |
| institution (rejeitada) | `e2e-inst-20260908201033-rej@agrobench.test` | não | institution `01a082a6-3660-7d81-aeb1-761949481da6` |
| admin (seed) | `admin@agrobench.local` / `admin123` | não | tokens no login |

- producer: phone `+5554908201033`, cpf `70908201033`, senha `senha-segura-1`
- CNPJ aprovada: `20260908201033` · CNPJ rejeitada: `99887766000155`
- CAR próprio: `RS-4314100-AAA00000000000000016` → `approved`
- Ciclos **meus** (wheat × Passo Fundo): `E2E08201033` **open** `01a082a4-cef1-75c9-ac7d-3ede33656d6f` · `E2E8201033c` **aggregated** `01a082a4-cefb-78ca-9b8a-ebe56f74d661` (fechei só este; não fechei ciclos de outros)

Tokens JWT truncados (`…`). `expires_in`: **900**. Institution/admin: **sem** `mfa_required`.

**Usuários do reteste (prefixo `e2e-retest-20260908202640`)**

| Papel | Email | MFA | IDs |
|---|---|---|---|
| institution (aprovada) | `e2e-retest-20260908202640@agrobench.test` | não | institution `01a082b4-7681-784d-9e7d-7490b31e8f79` · user `01a082b4-767f-7a5a-95d7-d9e924c53b61` |
| admin (próprio via SQL) | `e2e-retest-adm-20260908202640@agrobench.test` | não | `01a082b4-7781-7c8c-ad6d-0cec0bbbb2c8` (`UPDATE users SET role='admin', mfa_enabled=false`) |
| producer | `e2e-retest-prod-20260908202640@agrobench.test` | sim | `user_id` `01a082b7-dcb9-705c-812f-c72a0bf26cad` · wallet `01a082b7-dcca-7a90-aded-364c6089324c` |

- senha `senha-segura-1` · CNPJ `20260908202640` · producer phone `+5554908202641` cpf `80908202640`
- CAR próprio: `RS-4314100-E2ERT20260908202640` → `approved` (inserido em `mock_sicar_cars`)
- Ciclos **já existentes** (não criei, **não fechei**): wheat open `E2E08201033` `01a082a4-cef1-75c9-ac7d-3ede33656d6f` · wheat aggregated `E2E8201033c` `01a082a4-cefb-78ca-9b8a-ebe56f74d661` · soja aggregated (outro testador, só leitura) `e2e8898121` `01a082a3-18b2-7489-8037-063a9972aec2`
- **Não** rodei `seed-demo`. Healthz/readyz 200. Postgres `docker compose exec -T db`.

---

## Reteste — `report_access` (FK institutions.id)

**Resultado**: ✅ PASSOU

Fluxo: register institution → login tokens sem MFA → admin próprio SQL → `POST /admin/institutions/{id}/approve` 204 → subscribe regional Passo Fundo 201 → `POST /api/v1/webhooks/payment` `status=paid` 204 (sub `active`).

Três GET `/api/v1/benchmark/report?cycle=…` (Bearer institution):

| # | cycle | HTTP | Body |
|---|---|---|---|
| 1 | `e2e8898121` (soja, tem aggregates) | **200** | `basic:area_ha` / `basic:total_cost_ha` n=1 |
| 2 | mesmo ciclo | **200** | idem |
| 3 | `E2E8201033c` (trigo aggregated vazio) | **200** | `metrics: null` |
| — | `cycle=not-uuid` | **400** | `cycle inválido` |
| — | producer no report | **403** | `perfil insuficiente` |

**SQL** — `institutions.id` ≠ `users.id`; `report_access.institution_id` é o da instituição, **não** o do user:

```
institution_id 01a082b4-7681-784d-9e7d-7490b31e8f79
user_id        01a082b4-767f-7a5a-95d7-d9e924c53b61
ids_diferentes t
subscription   01a082b5-b90d-7c6c-892c-cc62f8139d16  status=active  plan=regional
```

| report_access.id | institution_id = institutions.id | = users.id | cycle |
|---|---|---|---|
| `01a082b5-b921-…` | t | f | soja `e2e8898121` |
| `01a082b5-b928-…` | t | f | soja `e2e8898121` |
| `01a082b5-b92c-…` | t | f | trigo `E2E8201033c` |

`count(*)` = **3** (1 linha por GET). `subscription_id` da assinatura `active`. Primeira passagem deste arquivo: 0 rows (INSERT usava `users.id` no FK).

---

## Reteste — pool `POST /admin/pool/distribute?month=2026-09`

**Resultado**: ✅ PASSOU (não é mais `carried` por tabela vazia)

Antes do job (com os 3 acessos já gravados):

```
report_access no mês: ciclo soja n=2, ciclo trigo n=1  (totalAcc=3)
contribution_weights: 1 row no ciclo soja (weight=1); 0 no trigo
accepted em 2026-09: 2
gross payments SET: 3000.00
pool_periods 2026-09 ainda: gross=2500 status=carried payouts=0  (job antigo)
```

1º `POST /api/v1/admin/pool/distribute?month=2026-09` (admin próprio): **202** body vazio. Institution no mesmo POST: **403** `perfil insuficiente`.

O período **não mudou** — River `UniqueOpts{ByArgs:true}` **pulou** o insert (`river_job` só tinha o completed id=5 `{"month":"2026-09"}` da 1ª passagem). HTTP 202 mesmo assim.

Work-around **só deste reteste** (não é código da API): `DELETE FROM river_job WHERE id=5`. 2º POST **202**. Poll imediato:

```json
{
  "id": "01a082a6-3c7e-7d9a-8263-0c9273a6d1d7",
  "month": "2026-09",
  "gross": 3000,
  "infra_cost": 4,
  "maintainer_fee": 450,
  "net": 2546,
  "status": "distributed"
}
```

- `gross=3000` = 2500 anteriores + 500 da institution nova
- `infra_cost=4` = 2 accepted × 2 USDC
- `maintainer_fee=450` = 15% de 3000
- **`status=distributed`** — `totalAcc=3` > 0 e `net>0`. **Não** é `carried` por `report_access` vazio.

**Payouts**: 1 row `amount_usdc=1697.333333` wallet `01a082a0-f2f3-…` (contribuidor do ciclo soja) = `2546 * 2/3` (2 acessos da soja / 3). Ciclo trigo teve 1 acesso mas `contribution_weights` = 0 → worker faz `wsum==0` / `continue` (fatia 1/3 **não** vira payout; status permanece `distributed`). `GET /wallet/payouts` do producer **novo** deste reteste: **200** `null` (não é o wallet pago).

Causa SQL se ainda fosse `carried`: **não se aplica**. Causa real da 1ª passagem era `totalAcc==0`. Agora há `report_access`. A fatia sem weights **não** rebaixa o status para `carried`.

---

## Reteste — painel `cycles_validated` (não hardcoded 0)

**Resultado**: ✅ PASSOU (n real; n=1 prova que não é literal 0)

Producer novo → MFA OTP `259885` → wallet pubkey `E2ERetestWallet20260908202640XXXX` saldo HTTP 0.

| Caso | HTTP | `cycles_validated` | `detail` |
|---|---|---|---|
| sem wallet | **404** | — | `wallet não encontrada` |
| wallet, 0 accepted, ciclo trigo | **403** | **0** | `painel bloqueado: 0/3 ciclos consecutivos` |
| `cycle=not-uuid` | **400** | — | `cycle inválido` |
| institution no painel | **403** | — | `perfil insuficiente` (sem body extra) |

Mint SQL 50 USDC (sem mint HTTP). Property approved. Commit **attempt 1** no ciclo **open** `E2E08201033` (não fechei): **201** `attempt:1` `status:committed` + stake `locked` 10 USDC. Reveal sealed box NaCl → 200 `revealed` → poll **accepted**.

| Caso após 1 `accepted` trigo × Passo Fundo | HTTP | `cycles_validated` | `detail` |
|---|---|---|---|
| `?cycle=` trigo open `E2E08201033` | **403** | **1** | `painel bloqueado: 1/3 ciclos consecutivos` |
| `?cycle=` trigo aggregated `E2E8201033c` | **403** | **1** | idem (mesma cultura×região) |
| `?cycle=` soja `e2e8898121` | **403** | **0** | `0/3` (contador é por cultura×região) |

JSON do n=1:

```json
{
  "code": "FORBIDDEN",
  "message": "Painel bloqueado",
  "detail": "painel bloqueado: 1/3 ciclos consecutivos",
  "cycles_validated": 1,
  "cycles_required": 3
}
```

n=0 com `detail` `0/3` já era consistente; n=1 ≠ 0 **isola** que o handler lê o n real (não hardcode). Não forcei 3 ciclos.

---

## GET /healthz e /readyz

**Resultado**: OK — 200 `{"status":"ok"}` / `{"database":"up","status":"ok"}`

---

## Login admin / institution / producer (MFA)

### `POST /api/v1/auth/login` admin

```bash
curl -sS -D - -X POST http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@agrobench.local","password":"admin123"}'
```

- HTTP **200** `access_token` + `refresh_token` + `expires_in:900` + `token_type: Bearer` — **sem** `mfa_required`
- `GET /me` → `role=admin`, `mfa_enabled=false`
- Campo extra `foo`: **400** `unknown field "foo"`

### Producer (MFA)

```bash
curl -sS -X POST http://localhost:8080/api/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"e2e-inst-20260908201033-prod@agrobench.test","phone":"+5554908201033","cpf":"70908201033","password":"senha-segura-1"}'
```

- HTTP **201** `{mfa_required:true, mfa_token, user_id}` — sem access/refresh
- `GET /api/v1/admin/otp/01a082a4-cdf3-77f3-bf31-21c7f4a5bc3d` (público) → **200** `{"code":"264115","purpose":"login"}`
- OTP id inválido **400** `user_id inválido`; UUID inexistente **404** `usuário não encontrado`
- MFA JWT no `Authorization` → **401** `token inválido`
- `POST /auth/mfa/verify` → **200** tokens
- Relogin producer: **200** MFA challenge **sem** `access_token`

### Institution (sem MFA)

Após `POST /institutions/register` + login:

- HTTP **200** tokens diretos. Keys: `access_token`, `refresh_token`, `expires_in`, `token_type`. Sem `mfa_required`.
- `GET /me`: `role=institution`, `mfa_enabled=false`, `phone=+5500000000000`

---

## OTP mock (`GET /api/v1/admin/otp/{user_id}`)

**Resultado**: OK

- Público, SMS mock. Producer: **200** código 6 dígitos.
- Institution (não envia SMS): **404** `nenhum OTP pendente` (user existe).

---

## Ciclos admin (próprios)

`culture_id` trigo `01a0829e-248f-77ac-8c9b-19aea73b2b7f` · Passo Fundo `01a0829e-2468-7495-88a6-052eaa12699a`.

### `POST /api/v1/admin/cycles`

- **201** `status=open` (label `E2E08201033` e `E2E8201033c`)
- `closes_at` antes de `opens_at` → **400** `closes_at deve ser depois de opens_at`
- mesmo cultura×região×label → **409** `ciclo já existe para cultura × região × safra`
- como producer → **403** `perfil insuficiente`

### `GET /api/v1/cycles`

- **200** lista (inclui os meus + ciclos de outros testadores). Filtro `status=open&culture=<uuid>&region=<uuid>` **200**.

### `POST /api/v1/admin/cycles/{id}/close`

Fechei **só** `E2E8201033c`:

- **200** `status=closed` → poll `GET /cycles?status=aggregated` → **aggregated** na 1ª tentativa
- close de novo → **409** `ciclo não está aberto`
- id inválido → **400** `id inválido`

**SQL**

```
E2E08201033 | open
E2E8201033c | aggregated
aggregates no meu ciclo fechado: 0  (zero contribuições accepted)
```

---

## Painel do produtor `GET /api/v1/benchmark/me`

**Resultado (1ª passagem)**: OK (403 esperado; n=0 só). **Reteste**: ✅ `cycles_validated` real (`0` e `1`) — ver seção Reteste acima.

```bash
curl -sS -D - 'http://localhost:8080/api/v1/benchmark/me?cycle=01a082a4-cef1-75c9-ac7d-3ede33656d6f' \
  -H 'Authorization: Bearer <access producer>'
```

- sem `cycle` / `cycle=not-uuid` → **400** `cycle inválido`
- producer < 3 ciclos → **403**

```json
{
  "code": "FORBIDDEN",
  "message": "Painel bloqueado",
  "detail": "painel bloqueado: 0/3 ciclos consecutivos",
  "cycles_validated": 0,
  "cycles_required": 3
}
```

- **1ª passagem**: `cycles_validated=0` batia com `detail` `0/3` (produtor sem `accepted`). Hardcode **não** isolado aqui (commit 502 saldo 0). **Reteste**: n=1 isolado — ver seção Reteste.
- como admin → **403** `perfil insuficiente` (sem o body extra do painel)

`GET /wallet` após create: `balance_usdc=0`. `GET /wallet/payouts` antes e depois do split: **200** `null`.

---

## Instituição: cadastro → pending → approve → subscribe

### `POST /api/v1/institutions/register`

```bash
curl -sS -X POST http://localhost:8080/api/v1/institutions/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"e2e-inst-20260908201033@agrobench.test","password":"senha-segura-1","cnpj":"20260908201033","name":"Cooperativa E2E 20260908201033"}'
```

- HTTP **201** `{"id":"01a082a4-d29a-7555-9e68-e55ed99d305b","name":"…","status":"pending"}` — **sem** `user_id`
- duplicata email/CNPJ → **409** `email ou CNPJ já cadastrado`
- `cnpj:"123"` → **400** `cnpj: min=14`
- `cnpj:"12.345.678/0001"` (14 chars, <14 dígitos) → **400** `CNPJ inválido`
- extra `foo` → **400** `unknown field "foo"`

### `GET /api/v1/institutions/me`

- Bearer institution, pending → **200** `status=pending`
- após approve → **200** `status=approved`
- producer / admin → **403** `perfil insuficiente`

**SQL pending**

```
id=01a082a4-d29a-… status=pending approved_at=NULL
email=e2e-inst-20260908201033@agrobench.test role=institution mfa_enabled=f phone=+5500000000000
```

### Subscribe ainda pending

```bash
curl -sS -X POST http://localhost:8080/api/v1/institutions/subscribe \
  -H 'Authorization: Bearer <inst>' -H 'Content-Type: application/json' \
  -d '{"plan":"regional","regions":["01a0829e-2468-7495-88a6-052eaa12699a"]}'
```

- **403** `instituição ainda não aprovada`

### `POST /api/v1/admin/institutions/{id}/approve`

- **204** body vazio, `Content-Type: application/json`
- GET me → `approved`. SQL: `status=approved`, `has_approved_at=t`
- UUID inexistente → **404** `instituição não encontrada`
- id inválido → **400** `id inválido`

### `POST /api/v1/admin/institutions/{id}/reject`

Segunda instituição `e2e-inst-20260908201033-rej@…`:

- register **201** pending → reject **204** → GET me **200** `status=rejected`
- subscribe national → **403** `instituição ainda não aprovada` (trata rejected como não-approved)

**SQL**

```
Cooperativa E2E 20260908201033 | approved
Rejeitada E2E 20260908201033   | rejected
```

---

## Checkout mock + webhook + admin confirm

### `POST /api/v1/institutions/subscribe` (aprovada)

| Caso | HTTP | Detail |
|---|---|---|
| regional sem `regions` | **400** | `plano regional exige 1 a 5 microrregiões` |
| regional 6 regiões | **400** | idem |
| extra field | **400** | `unknown field "foo"` |
| como producer | **403** | `perfil insuficiente` |
| regional 1 região | **201** | `subscription_id`, `checkout_url`, `provider_ref` — **sem** `payment_id` |

**201**

```json
{
  "subscription_id": "01a082a6-3812-7678-8077-8702a1c43b88",
  "checkout_url": "http://localhost:8080/mock/checkout/mock_cs_01a082a6-3812-7678-8077-8702a1c43b88",
  "provider_ref": "mock_cs_01a082a6-3812-7678-8077-8702a1c43b88"
}
```

**SQL pós-checkout**: subscription `pending`, `plan=regional`, `future_end=t`, payment `amount=500.00` `provider=mock` `confirmed_at` NULL.

`GET` da `checkout_url` → **404** `Rota não encontrada` (contrato: não é rota da API).

### `POST /api/v1/webhooks/payment` (público)

Path real: **`/api/v1/webhooks/payment`** (não `/webhooks/stripe`).

`status=open` + header `Stripe-Signature: ignored` → **204**; SQL continua `pending` / `confirmed_at` NULL.

```bash
curl -sS -X POST http://localhost:8080/api/v1/webhooks/payment \
  -H 'Content-Type: application/json' \
  -d '{"ref":"mock_cs_01a082a6-3812-7678-8077-8702a1c43b88","subscription_id":"01a082a6-3812-7678-8077-8702a1c43b88","amount_usdc":500,"status":"paid"}'
```

- **204**. SQL: subscription **`active`**, `paid=t`, `pool_tx=mock_606f0c08…`
- treasury 1_000_000 − 500 = **999495** (outro testador já tinha mexido no saldo); pool **500** USDC
- `ref` inexistente + `status=paid` → **204** (evento ignorado; não 404)

### `POST /api/v1/admin/payments/{id}/confirm`

`payment_id` **não** vem no subscribe — lido no SQL: `01a082a6-3a74-7707-9c57-9e0282a1cb58` (2ª assinatura **national** 2000 USDC).

- **204**. SQL: subscription national **`active`**, `pool_tx=mock_05892023…`
- payment inexistente → **404** `pagamento não encontrado`

**SQL payments finais**

| id | amount | paid |
|---|---|---|
| `01a082a6-3815-…` | 500 | t |
| `01a082a6-3a74-…` | 2000 | t |

Subscriptions: regional `active` + national `active`.

---

## Relatório pago `GET /api/v1/benchmark/report`

**Resultado (1ª passagem)**: PARCIAL (HTTP 200; `report_access` não gravava). **Reteste**: ✅ grava `institutions.id` — ver seção Reteste.

- sem `cycle` → **400** `cycle inválido`
- como producer → **403** `perfil insuficiente`
- institution + meu ciclo aggregated vazio → **200** `{"cycle_id":"01a082a4-cefb-…","metrics":null}`
- query `region`/`culture` extra: ainda **200** (handler ignora)
- instituição **rejected** (sem assinatura) → **200** (contrato: sem `RequireSubscription`)

GET num ciclo **de outro testador** que já tinha aggregates (só leitura; não fechei o ciclo):

```bash
GET /api/v1/benchmark/report?cycle=01a082a3-18b2-7489-8037-063a9972aec2
```

HTTP **200**

```json
{
  "cycle_id": "01a082a3-18b2-7489-8037-063a9972aec2",
  "metrics": [
    {"metric":"basic:area_ha","mean":50,"median":50,"p25":50,"p75":50,"n":1},
    {"metric":"basic:total_cost_ha","mean":5000,"median":5000,"p25":5000,"p75":5000,"n":1}
  ]
}
```

**SQL `report_access` depois dos GETs da 1ª passagem: 0 rows.** Insert usava `auth.UserID` no FK `institutions.id`. **Reteste**: JOIN `i.user_id`; 3 rows com `institution_id` = `institutions.id`.

---

## Pool e jobs admin

### `GET /api/v1/pool/periods`

- antes: **200** `null` (slice nil)
- `GET /pool/periods/2026-07` antes do job → **404** `período não encontrado`

### `POST /api/v1/admin/pool/distribute?month=2026-09`

- como producer → **403**
- como admin → **202** body vazio
- `GET /pool/periods/2026-09` imediato **200**

```json
{
  "id": "01a082a6-3c7e-7d9a-8263-0c9273a6d1d7",
  "month": "2026-09",
  "gross": 2500,
  "infra_cost": 2,
  "maintainer_fee": 375,
  "net": 2123,
  "status": "carried"
}
```

`gross=2500` = 500+2000 pagos no mês. `maintainer_fee=375` = 15% de 2500. `infra_cost=2` = 1 contribuição `accepted` de **outro** testador no mês × 2 USDC. **1ª passagem**: `status=carried` porque `report_access` vazio (`totalAcc==0`), **mesmo com net>0**. `payouts` = **0**. **Reteste**: `distributed` / payout 1697.33 — ver seção Reteste.

Não há cron: só este POST (ou `jobs/run`).

### `POST /api/v1/admin/jobs/run/{kind}`

- `nao_existe` / `validate_contribution` → **400** `{code:INVALID_INPUT, message:"job desconhecido"}` (sem `detail`)
- `distribute_pool?month=2026-07` → **202** → GET período **200** `gross=0` `status=carried`

`POST /admin/seed/demo` **não chamado**.

---

## SQL conferido

```sql
SELECT email, role, mfa_enabled FROM users WHERE email LIKE 'e2e-inst-20260908201033%';
SELECT id, name, status FROM institutions;
SELECT id, plan, status FROM subscriptions;
SELECT id, amount, confirmed_at IS NOT NULL AS paid, pool_tx FROM payments;
SELECT count(*) FROM report_access;          -- 1ª passagem: 0; reteste: 3
SELECT month, status, gross, net FROM pool_periods;  -- reteste 2026-09: distributed 3000/2546
SELECT count(*) FROM payouts;                -- 1ª passagem: 0; reteste: 1
SELECT label, status FROM cycles WHERE label LIKE 'E2E%';
```

`mock_chain_balances`: treasury debitada, pool creditada nos dois confirms.

---

## Resumo por endpoint

| Endpoint | Resultado |
|---|---|
| `GET /api/v1/benchmark/me` | OK (403 < 3 ciclos; reteste `cycles_validated` 0 e **1**) |
| `GET /api/v1/benchmark/report` | OK (200; reteste grava `report_access` com `institutions.id`) |
| `POST /api/v1/institutions/register` | OK |
| `GET /api/v1/institutions/me` | OK |
| `POST /api/v1/institutions/subscribe` | OK |
| `POST /api/v1/webhooks/payment` | OK (path real `/api/v1/webhooks/payment`) |
| `POST /api/v1/admin/institutions/{id}/approve` | OK 204 |
| `POST /api/v1/admin/institutions/{id}/reject` | OK 204 |
| `POST /api/v1/admin/payments/{id}/confirm` | OK 204 (id via SQL) |
| `GET /api/v1/pool/periods` | OK (`null` vazio; lista depois) |
| `GET /api/v1/pool/periods/{month}` | OK |
| `POST /api/v1/admin/pool/distribute` | OK (reteste: 202 → `distributed`, 1 payout; unique River pulou o 1º POST) |
| `POST /api/v1/admin/jobs/run/{kind}` | OK (`distribute_pool` 202; outro kind 400) |
| `GET /api/v1/admin/otp/{user_id}` | OK |
| `POST /api/v1/admin/cycles` | OK |
| `POST /api/v1/admin/cycles/{id}/close` | OK (só ciclo meu) |
| `GET /api/v1/wallet/payouts` | OK `null` no producer novo (payout foi à wallet de outro ciclo) |

## Falhas deste módulo

1. **`report_access` FK — CORRIGIDO no reteste.** 1ª passagem: 0 rows (`users.id` no FK). Reteste: 3 rows, `institution_id` = `institutions.id` ≠ `users.id`. Split `2026-09` → `distributed` + 1 payout (não `carried` por tabela vazia). Unique River no mesmo `month` devolve 202 sem reprocessar até apagar o job completed.
2. **`cycles_validated` hardcoded 0 — CORRIGIDO no reteste.** 403 com n=0 (`0/3`) e n=1 (`1/3`); soja permanece `0/3` (contador por cultura×região).
3. Relatório **não** exige instituição approved nem subscription active (contrato §12; instituição rejected leu métricas na 1ª passagem). Não retestado de propósito.

## Edge cases cobertos

- extra JSON 400; CNPJ curto / dígitos insuficientes; duplicata 409
- subscribe pending/rejected 403; regional 0 e 6 regiões 400
- webhook `status!=paid` e ref inexistente → 204 sem ativar
- checkout_url mock 404
- roles cruzadas (producer em admin/institution/report; admin em institutions/me)
- MFA JWT como access 401
- jobs kind desconhecido sem `detail`
- lista vazia `null` vs array
- 204 + `Content-Type: application/json`
