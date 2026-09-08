# E2E Cycles / Enclave / Contributions — 2026-09-08

**Status do módulo**: OK (reteste attempt 1)

- Reveal, validação (enclave mock), atestação e close de ciclo: **OK** (1ª passagem e reteste)
- Commit **attempt 1** (stake 10 USDC): **CORRIGIDO** no reteste — 201 `committed` `attempt:1`, 1 stake com o mesmo `contribution_id`. 1ª passagem: 409 FK (work-around SQL `rejected` → attempt 2).

**Ambiente**: API `http://localhost:8080` (já no ar; healthz 200 / readyz 200). Postgres `docker compose exec -T db psql -U agrobench -d agrobench`. Não subi docker. Não rodei `make seed-demo`. Catálogo (cultures/micro-regions) já existia (seed base de outro agente).

**Producer próprio**:
- email: `e2e-contrib-1788897980@agrobench.local`
- phone: `+555498897980`
- `user_id`: `01a082a0-f04e-73de-ad87-8c2eb4a1f154`
- senha: `e2e-senha-1`
- wallet pubkey: `E2EContribWallet1788897980XXXXXX`
- `wallet_id`: `01a082a0-f2f3-7587-9aeb-f2f9fe7515fa`
- property: `01a082a0-f2f9-7d7e-91dd-0104469af594` (`car_status=approved`)
- CAR exclusivo (inserido em `mock_sicar_cars`): `RS-4314100-E2E1788897980`

**Admin próprio** (não usei `admin@agrobench.local`):
- email: `e2e-contrib-adm-1788897980@agrobench.local`
- `user_id`: `01a082a0-f0f6-74bd-9642-93c955ea88b7`
- Como obteve: `POST /auth/register` (nasce producer+MFA) + SQL `UPDATE users SET role='admin', mfa_enabled=false WHERE id='…'` + `POST /auth/login` → tokens diretos (sem MFA).

**Ciclo testado (fluxo completo)**:
- `cycle_id`: `01a082a3-18b2-7489-8037-063a9972aec2`
- label: `e2e8898121` · soja × Passo Fundo (`43010`) · `opens_at` 2026-09-01 · `closes_at` 2027-06-01
- contribuição aceita: `01a082a3-1967-718d-a6e2-182cba04d0d2`

Sealed box: helper Go `/tmp/e2eseal` (`golang.org/x/crypto/nacl/box.SealAnonymous` = `crypto_box_seal`). Payload basic canônico (chaves ordenadas, sem espaços), hash SHA-256 dos bytes exatos.

---

## Reteste 2026-09-08 20:28 UTC — commit attempt 1 (409 FK)

**Resultado**: **CORRIGIDO**

Não alterei código da API neste reteste. Não rodei `seed-demo`. Não fechei ciclos de outros. Pepper mudou: usuário **novo**.

**Air / binário** (código novo estava no processo):

| Item | Timestamp UTC |
|---|---|
| `internal/contribution/usecase/commit.go` | 2026-09-08 20:19:59 |
| `/api/dev/main` (Air) | 2026-09-08 20:20:42 |
| Boot API (log) | 2026-09-08 17:20:42-03 = 20:20:42Z — `building...` / `running...` / `http server iniciado` |

Binário **depois** do `commit.go`. Container `agrobench-api` recriado (~20:20Z); `AUTH_PEPPER` injetado (len>0). Enclave mock regenerou chaves no boot (`box_public_key=0aef720adc3f3ecd…`). Logs **sem** `stakes_contribution_id_fkey` / `SQLSTATE 23503`.

Ordem no use case (lido no fonte que o binário compilou): `WithTx` → `Create` contribution → `LockStake` → `CreateStake`.

**Producer novo**:
- email: `e2e-retest-commit-20260908202853@agrobench.local`
- `user_id`: `01a082b5-9892-73a2-b715-5012cbf4c61b`
- senha: `e2e-senha-1`
- wallet pubkey: `E2ERetestCommit20260908202853XXXXXXXXXXXXXXX`
- `wallet_id`: `01a082b5-9a8d-7991-bc35-9abd3fdc3fa8`
- property: `01a082b5-9b32-7687-95e1-ed314cca57eb` (`car_status=approved`)
- CAR exclusivo (`mock_sicar_cars`): `RS-4314100-E2ERET20260908202853` → norm `RS4314100E2ERET20260908202853`

**Admin próprio** (não usei `admin@agrobench.local`):
- email: `e2e-retest-adm-20260908202853@agrobench.local`
- `user_id`: `01a082b5-9948-7dce-8de6-cc4aa99bcb51`
- SQL `UPDATE users SET role='admin', mfa_enabled=false` + `POST /auth/login` → tokens (`expires_in=900`, sem MFA)

**Ciclo próprio OPEN** (não fechei):
- `cycle_id`: `01a082b5-9a80-785b-baee-161b4378926d`
- label: `RT08202853` · soja × Passo Fundo · `opens_at` 2026-09-01 · `closes_at` 2027-06-01 · `status=open`

Mint SQL 50 USDC na pubkey (sem mint HTTP). `GET /wallet` → `balance_usdc: 50`.

**Antes do commit**: `SELECT count(*) FROM contributions WHERE wallet_id=…` = **0**. Nenhuma contribution `rejected` inserida.

### POST `/api/v1/contributions/commit` attempt 1

HTTP **201**

```json
{
  "id": "01a082b5-9d1c-76fe-8b7c-a96d9e58ad27",
  "attempt": 1,
  "status": "committed",
  "commit_hash": "642287bb18f6b99de61338be33de8d3ac48ca2f48ce9c11e7a996cc900ee59aa",
  "commit_tx": "mock_338410a97aa499c907f697cb7aae598f"
}
```

Payload basic canônico (chaves ordenadas, sem espaços): `area_ha=50`, `total_cost_brl=250000`, `culture_code=soybean`.

### SQL após attempt 1 (antes do reveal)

| Tabela | Resultado |
|---|---|
| `contributions` | **1** row: `attempt=1` `status=committed` id `01a082b5-9d1c-…` |
| `stakes` | **1** row: `contribution_id` **igual**, `amount_usdc=10.000000`, `status=locked`, `lock_tx=mock_8f7909cb…` |
| `mock_chain_balances` | 40_000_000 (40 USDC) — caiu 10 |
| `contributions` `rejected` | **0** |
| `GET /wallet` | `balance_usdc=40` |

### Retry mesmo ciclo

HTTP **409** `CONFLICT` `detail: "já existe contribuição ativa neste ciclo"` — esperado (não é o FK).

### Reveal + poll (fluxo restante não quebrou)

Sealed box NaCl real (`SealAnonymous`) para `box_public_key=0aef720adc3f3ecd…` (boot atual). Hash do helper = hash do commit.

| Passo | HTTP | Observação |
|---|---|---|
| `POST …/reveal` | **200** | `status: revealed` |
| poll `GET …/{id}` | **200** | `#1` já `accepted` (~350 ms; `validating` não visível) |

SQL pós-accepted:

| Tabela | Resultado |
|---|---|
| `contributions` | `attempt=1` `status=accepted` |
| `stakes` | ainda `locked` (sem `release_tx`) |
| `validation_verdicts` | `verdict=accepted`; pubkey prefixo `0bbf447bfdf51091` = `signing_public_key` |
| `validated_metrics` | `area_ha=50`, `total_cost_ha=5000` |
| `attestations` | `reward_usdc=5`, `tx=mock_196b5cd3…`, `paid_at=2026-09-08 20:28:57 UTC` |
| `mock_chain_events` | commit, stake_lock, attest, transfer (`mock_5e74661f…`, wallet vazio no FROM treasury) |
| saldo final | 45_000_000 (40 + reward 5) |

---

## SQL de saldo mock (problema conhecido)

Produtor novo começa com `balance_usdc=0`. Não há mint HTTP. 1º commit tenta `LockStake(10 USDC)`.

1. `POST /wallet` → 201 `balance_usdc: 0`
2. `POST /contributions/commit` com saldo 0 → **502** `EXTERNAL_DEPENDENCY_ERROR` `detail: "falha ao travar stake (saldo insuficiente?)"`
   - Log: `chain mock: saldo insuficiente: conta E2EContribWallet1788897980XXXXXX, valor 10.000000 USDC`
   - Side-effect: `mock_chain_events` kind=`commit` **já foi gravado** (Commit on-chain roda *antes* do LockStake).
3. Crédito no banco (50 USDC = 50_000_000 micro-USDC):

```sql
INSERT INTO mock_chain_balances (account, micro_usdc)
VALUES ('E2EContribWallet1788897980XXXXXX', 50000000)
ON CONFLICT (account) DO UPDATE
  SET micro_usdc = mock_chain_balances.micro_usdc + EXCLUDED.micro_usdc, updated_at = now();
```

4. `GET /wallet` → `balance_usdc: 50`

Também garanti treasury mock ≥ 100 USDC para o `TransferUSDC` da recompensa (nessa instância a treasury já tinha saldo alto do seed).

**Saldo final** após 3 LockStake órfãos (10 USDC cada, attempt 1 quebrada) + reward 5 USDC: `balance_usdc=25`, `reward_usdc=5`.

---

## Bug bloqueante (1ª passagem): commit attempt 1 → 409 FK

> Superado no reteste acima. Mantido como evidência da 1ª passagem.

`internal/contribution/usecase/commit.go` na attempt 1: `LockStake` (on-chain) → `CreateStake` (tabela `stakes`) → `Create` (tabela `contributions`).

`stakes.contribution_id` **REFERENCES** `contributions(id)`. CreateStake antes do Create → `SQLSTATE 23503`.

Log da API:

```
code=CONFLICT op=contribution.CreateStake
ERROR: insert or update on table "stakes" violates foreign key constraint "stakes_contribution_id_fkey" (SQLSTATE 23503)
```

HTTP **409** `{ "code": "CONFLICT", "message": "Conflito ao processar requisição" }` — **sem `detail`**.

Efeitos:
- Contribuição **não** é persistida.
- `LockStake` **já debitou** 10 USDC e deixou linha órfã em `mock_chain_stakes` (PK = contribution_id que nunca existiu em `contributions`).
- `GET /contributions` continua `[]`.
- Retry HTTP continua 409 (mesmo com saldo).

**Work-around só para este E2E** (não é contrato; não alterei código da API):

```sql
INSERT INTO contributions (id, cycle_id, wallet_id, property_id, level, attempt,
  commit_hash, commit_tx, commit_at, status, reject_reason)
VALUES ('76500bf4-0a7f-4d33-9897-606afc3a472e', '<cycle>', '<wallet>', '<property>',
  'basic', 1, '<64 a''s>', 'mock_e2e_fk_workaround', now(), 'rejected',
  'e2e: workaround CreateStake FK before Create contribution');
```

Isso faz `HasRejected=true` → próximo commit é **attempt 2**, que **não** chama `LockStake`/`CreateStake` → `Create` funciona.

Correção sugerida: criar a contribution **antes** do stake (ou numa transação com FK deferrable), e só então `CreateStake`. Idealmente reverter o LockStake on-chain se o insert falhar.

---

## GET /api/v1/cycles

**Resultado**: OK (público; filtros `culture`/`region` = UUID, `status` literal)

Antes de criar ciclo: 200 `[]` (não `null` neste run).

Depois:

| Query | HTTP | Observação |
|---|---|---|
| (sem filtro) | 200 | lista completa, `ORDER BY opens_at DESC` |
| `?status=open` | 200 | só o ciclo OPEN recém-criado |
| `?culture=<soybean uuid>` | 200 | 2 ciclos soja × Passo Fundo |
| `?region=<Passo Fundo uuid>&status=closed` | 200 | `[]` — o ciclo 1 já estava `aggregated`, não `closed` |
| `?culture=&region=&status=open` | 200 | inclui o ciclo criado (`e2e8898121`) |

---

## POST /api/v1/admin/cycles

**Resultado**: OK

Bearer admin próprio. Body: `culture_id` soja, `micro_region_id` Passo Fundo, `label` único `e2e8898121`, janela contendo now.

- HTTP **201** `status: "open"`
- `id`: `01a082a3-18b2-7489-8037-063a9972aec2`

Ciclo anterior `e2e8897980` (mesmo par cultura×região, outro label) também 201; depois close → job aggregate → `aggregated`.

---

## GET /api/v1/enclave/public-key

**Resultado**: OK (público)

- HTTP **200**
- `provider`: `"mock"`
- `box_public_key`: `a53569b6768d62ee2709f3a594f41c0bba78ce0c8c79373811e42f0f8e19cb6f` (64 hex x25519)
- `signing_public_key`: `c42937e72a7fda14c153552193095aebf86ea31a839e89266caffb857d10dedb` (64 hex ed25519)
- `attestation` omitido (mock)

---

## POST /api/v1/contributions/commit

**Resultado (1ª passagem)**: PARCIAL (attempt 1 quebrada; attempt 2 OK). **Reteste**: OK — ver seção no topo.

Payload basic canônico:

```json
{"cycle_id":"01a082a3-18b2-7489-8037-063a9972aec2","data":{"area_ha":50,"culture_code":"soybean","total_cost_brl":250000},"level":"basic","nonce":"<32 hex>","version":1}
```

`commit_hash` = SHA-256 hex = `3359a2fe37e8d5e26df34edc110fbf0d799897ec9beb7fda01cb2f6f2cb7a5e0`

| Caso | HTTP | Body |
|---|---|---|
| saldo 0, attempt 1 | **502** | `EXTERNAL_DEPENDENCY_ERROR` / `falha ao travar stake (saldo insuficiente?)` |
| saldo ≥10, attempt 1 | **409** | `CONFLICT` sem detail (FK `stakes_contribution_id_fkey`) |
| após dummy `rejected`, attempt 2 | **201** | `status: committed`, `attempt: 2`, `commit_tx: mock_cc39a308c8d35a6cd0e15d63f250ab3a` |
| 2ª ativa no mesmo ciclo | **409** | `detail: "já existe contribuição ativa neste ciclo"` |
| ciclo `closed`/`aggregated` | **409** | `detail: "ciclo não está aberto"` |

`id` aceito: `01a082a3-1967-718d-a6e2-182cba04d0d2`

---

## POST /api/v1/contributions/{id}/reveal

**Resultado**: OK (cifra real)

Ciphertext: NaCl sealed box para `box_public_key` do enclave, base64 std, 324 chars.

| Caso | HTTP | Body |
|---|---|---|
| ciphertext não-base64 (`@@@…`) | **400** | `INVALID_INPUT` / `ciphertext deve ser base64` |
| sealed box válido | **200** | `status: "revealed"` |
| reveal repetido | **409** | `detail: "contribuição não está em committed"` |

O motor de validação aceitou o ciphertext (veredito `accepted` — se a cifra estivesse errada, check `decrypt` falharia e a contribuição cairia em `rejected`).

---

## GET /api/v1/contributions e GET by id

**Resultado**: OK

Poll `GET /contributions/{id}`: `revealed` → `accepted` (worker rápido; `validating` não apareceu no poll de ~350 ms).

- Lista da wallet: 200, 2 itens (`accepted` attempt 2 + `rejected` dummy).
- GET by id: metadata sem ciphertext / checks.
- UUID inexistente: **404** `detail: "contribuição não encontrada"`.

GET HTTP **não** devolve veredito interno (contrato §3.6) — conferido no JSON.

---

## POST /api/v1/admin/cycles/{id}/close

**Resultado**: OK

- HTTP **200** `status: "closed"`
- Job `aggregate_cycle`: poll `GET /cycles?culture=…&region=…` → `aggregated` em <1 s.

Commit depois do close: **409** `ciclo não está aberto`.

---

## SQL — contributions, stakes, verdicts, attestations, chain

### contributions (wallet do e2e)

| id | attempt | status | has_ciphertext |
|---|---|---|---|
| `76500bf4-…` (dummy) | 1 | rejected | f |
| `01a082a3-1967-718d-a6e2-182cba04d0d2` | 2 | **accepted** | t |

Tabela domínio `stakes`: **0 rows** para essa wallet (attempt 2 não cria stake; attempt 1 HTTP nunca chegou a persistir).

### validation_verdicts

- `verdict`: **accepted**
- `enclave_signature`: 64 bytes
- `enclave_pubkey` prefixo `c42937e72a7fda14` (bate com `signing_public_key`)
- `checks`:

| check | passed |
|---|---|
| `hash_matches_commit` | true |
| `schema_valid` | true |
| `level_matches` | true |
| `cycle_matches` | true |
| `reference_range:area_ha` | true |
| `reference_range:total_cost_ha` | true |

### validated_metrics

| metric | value |
|---|---|
| `area_ha` | 50.0000 |
| `total_cost_ha` | 5000.0000 |

(`250000 / 50 = 5000`, faixa CONAB 2000–8000)

### attestations

- `tx`: `mock_d3a5fc85b3839481ac06e0d36e848943`
- `reward_usdc`: **5.000000** (basic 1× `rewards.base_usdc=5`)
- `paid_at`: 2026-09-08 20:08:42 UTC

### mock_chain_events (contribuição aceita)

| kind | tx_ref |
|---|---|
| commit | `mock_cc39a308c8d35a6cd0e15d63f250ab3a` |
| attest | `mock_d3a5fc85b3839481ac06e0d36e848943` |
| transfer | `mock_812d658441468d347d6a1bac9fb6155e` (reward treasury → wallet) |

Há ainda 3 `stake_lock` órfãos (10 USDC cada) das tentativas attempt 1, `released=f`, contribution_id **sem** row em `contributions`.

### mock_chain_balances (fim)

| account | micro_usdc |
|---|---|
| `E2EContribWallet1788897980XXXXXX` | 25_000_000 (25 USDC) |
| treasury mock | (já tinha saldo alto; −5 USDC de reward) |

`GET /wallet`: `balance_usdc=25`, `reward_usdc=5`.

---

## Edge cases

| Caso | Resultado |
|---|---|
| Saldo 0 no 1º commit | 502 stake — **passou** |
| Commit duplicado (ativa no ciclo) | 409 — **passou** |
| Ciclo fechado | 409 — **passou** |
| Reveal não-base64 | 400 — **passou** |
| Reveal repetido | 409 — **passou** |
| GET id inexistente | 404 — **passou** |
| Filtro cycles `status=closed` vs `aggregated` | lista vazia se já aggregou — **passou** |
| Sealed box real (NaCl) | worker accepted — **passou** |
| Commit attempt 1 com saldo (1ª passagem) | 409 FK — **falhou (bug)** |
| Commit attempt 1 com saldo (**reteste**) | 201 `attempt:1` `committed` + 1 stake — **passou (CORRIGIDO)** |
| Stake domínio persistido | 1ª passagem: não (attempt 2). Reteste: **1 row** `locked` no mesmo `contribution_id` |
| Status `validating` visível no poll | não observado (worker < 400 ms) |

---

## Recomendações

1. ~~**Corrigir ordem no `Commit`**~~ — **feito e retestado**: `Create` → `LockStake` → `CreateStake` na mesma tx. Attempt 1 = 201. Compensação `ReleaseStake` se o insert falhar (código presente; não exercitada neste reteste).
2. Mint HTTP ou crédito no `POST /wallet` mock (produtor limpo não staka).
3. 409 de FK deveria ter `detail` acionável; hoje o app só vê `CONFLICT`.
4. `LockStake` órfão não tem rollback — saldo some sem contribution.
