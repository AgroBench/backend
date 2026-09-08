# E2E Wallet / Property(CAR) / Catálogo — 2026-09-08

**Status do módulo**: ✅ OK (pepper **CORRIGIDO** no reteste)

Contrato HTTP (cultures, micro-regions, wallet, export OTP, property) **passou**. Persistência: pubkey + blob na wallet; CAR só como HMAC (não em claro). `AUTH_PEPPER` no container da API: **presente e não vazio** (reteste abaixo). HMACs gravados **antes** da correção continuam sendo HMAC com pepper `""`.

**Ambiente**: `http://localhost:8080` (já no ar; healthz 200). Postgres `docker compose exec -T db psql -U agrobench -d agrobench`. Não subiu docker. SMS mock.

**Catálogo**: estava **vazio** (`GET /cultures` e `GET /micro-regions` HTTP 200 body `null`). Rodei **`make seed`** (base, aditivo: `ON CONFLICT DO NOTHING`, log `rows=38`). **Não** rodei `seed-demo`. Inseri 2 CARs mock exclusivos deste teste (não usei 1–20 do seed).

**Usuário de teste (producer próprio)**:
- email: `e2e-wallet-1788897898@agrobench.test`
- phone: `+55549888897898`
- cpf: `81788897898`
- `user_id`: `01a0829f-b38c-786b-9a9a-6fbb3eb04bb1`
- pubkey: `E2eWalletPub1788897898XXXXXXXXXX` (32 chars; **nenhuma chave privada enviada**)
- `encrypted_blob` (base64 std): `ZTJlLWVuY3J5cHRlZC1ibG9iLTE3ODg4OTc4OTgtbm8tcHJpdmtleQ==`
- CAR ativo (inserido em `mock_sicar_cars`): `RS-4314100-E2EOK1788897898` → norm `RS4314100E2EOK1788897898`
- CAR inativo: `RS-4314100-E2ERJ1788897898`
- microrregião: Passo Fundo `ibge_code=43010` id `01a0829e-2468-7495-88a6-052eaa12699a`

Fluxo de sessão: `POST /auth/register` 201 → `GET /admin/otp/{user_id}` 200 code `622510` → `POST /auth/mfa/verify` 200 (`expires_in`: 900).

---

## Catálogo

### GET /api/v1/cultures (público)

**Resultado**: OK

Antes do seed: HTTP **200**, body `null` (contrato §1.5 aceita `[]` ou `null`).

Depois do `make seed`: HTTP **200** `application/json`, 3 itens, `ORDER BY name`:

| id | code | name |
|---|---|---|
| `01a0829e-248e-700c-9ad8-94b5c4d827c7` | corn | Milho |
| `01a0829e-248b-7757-9d31-fbb181cfaf48` | soybean | Soja |
| `01a0829e-248f-77ac-8c9b-19aea73b2b7f` | wheat | Trigo |

Com Bearer producer: mesmo 200 (rota pública).

### GET /api/v1/micro-regions (público)

**Resultado**: OK

Depois do seed: HTTP **200**, **35** itens, `uf=RS`, `ORDER BY uf, name`. Passo Fundo presente (`ibge_code` **43010**, não confundir com município CAR `4314100`).

SQL: `SELECT count(*) FROM micro_regions` → 35; `cultures` → 3.

---

## Wallet

### GET /api/v1/wallet (antes de criar)

**Resultado**: OK — HTTP **404** `{ "code": "NOT_FOUND", "detail": "wallet não encontrada" }`

### POST /api/v1/wallet sem Bearer

**Resultado**: OK — HTTP **401** `detail: "token ausente"`

### POST /api/v1/wallet com `private_key` extra

**Resultado**: OK (rejeita chave privada / campo desconhecido)

HTTP **400** `INVALID_INPUT` `detail: "json: unknown field \"private_key\""` (`DisallowUnknownFields`).

### POST /api/v1/wallet blob não-base64

**Resultado**: OK — HTTP **400** `detail: "encrypted_blob deve ser base64"`

### POST /api/v1/wallet (happy path)

**Resultado**: OK — HTTP **201**

```json
{
  "id": "01a0829f-b9e1-78ad-a88c-af1cbd69facc",
  "pubkey": "E2eWalletPub1788897898XXXXXXXXXX",
  "blob_version": 1,
  "balance_usdc": 0,
  "reward_usdc": 0
}
```

Sem `encrypted_blob` no JSON de create/get (só no export). Sem chave privada. Produtor novo: saldo 0 (contrato §1.9).

### GET /api/v1/wallet

**Resultado**: OK — HTTP **200**, mesmo shape (ainda sem `exported_at`).

### POST /api/v1/wallet duplicado (mesmo user)

**Resultado**: OK — HTTP **409** `detail: "wallet já cadastrada"`

### GET /api/v1/wallet/export sem `code`

**Resultado**: OK — HTTP **403**

```json
{ "code": "FORBIDDEN", "message": "OTP enviado", "otp_required": true }
```

OTP mock: `GET /admin/otp/{user_id}` → `{ "code": "964435", "purpose": "login" }` (purpose login, como o contrato §3.3 / §8).

### GET /api/v1/wallet/export?code=000000

**Resultado**: OK — HTTP **401** `detail: "código inválido"`

### GET /api/v1/wallet/export?code=964435

**Resultado**: OK — HTTP **200**

```json
{
  "pubkey": "E2eWalletPub1788897898XXXXXXXXXX",
  "encrypted_blob": "ZTJlLWVuY3J5cHRlZC1ibG9iLTE3ODg4OTc4OTgtbm8tcHJpdmtleQ==",
  "blob_version": 1
}
```

Blob idêntico ao enviado no POST. GET /wallet em seguida inclui `exported_at`: `2026-09-08T20:05:02Z` (formato `2006-01-02T15:04:05Z`).

### SQL `wallets`

```sql
SELECT id, pubkey, encode(encrypted_blob,'base64') AS blob_b64,
       octet_length(encrypted_blob) AS blob_len, blob_version, exported_at
FROM wallets WHERE user_id = '01a0829f-b38c-786b-9a9a-6fbb3eb04bb1';
```

| Campo | Valor |
|---|---|
| pubkey | `E2eWalletPub1788897898XXXXXXXXXX` |
| blob_b64 | `ZTJlLWVuY3J5cHRlZC1ibG9iLTE3ODg4OTc4OTgtbm8tcHJpdmtleQ==` (40 bytes) |
| blob_version | 1 |
| exported_at | `2026-09-08 20:05:02.012267+00` |

Colunas da tabela: `id, user_id, pubkey, encrypted_blob, blob_version, exported_at, created_at, updated_at`. **Não há** coluna de chave privada / secret.

---

## Property / CAR

### GET /api/v1/property (antes)

**Resultado**: OK — HTTP **404** `detail: "propriedade não encontrada"`

### POST /api/v1/property região inexistente

**Resultado**: OK — HTTP **404** `NOT_FOUND` `detail: "microrregião não encontrada"` (código; o doc §9 cita 400 *ou* NOT_FOUND).

### POST /api/v1/property CAR ativo (SICAR mock)

**Resultado**: OK — HTTP **201**

```json
{
  "id": "01a0829f-bf9f-746f-b35a-4650e6f4c9ee",
  "micro_region_id": "01a0829e-2468-7495-88a6-052eaa12699a",
  "car_status": "approved",
  "verified_at": "2026-09-08T20:05:02Z"
}
```

CAR **não** volta no JSON. `car_status` lido no body (não só HTTP).

### GET /api/v1/property (após approved)

**Resultado**: OK — HTTP **200**, mesmo `id` / `approved`.

### POST /api/v1/property CAR duplicado

**Resultado**: OK — HTTP **409** `detail: "CAR já vinculado"`

### POST /api/v1/property CAR inativo (mock `active=false`)

**Resultado**: OK — HTTP **201** `car_status: "rejected"` **sem** `verified_at` (contrato §3.4 / §9).

### POST /api/v1/property CAR inexistente no SICAR mock

**Resultado**: OK — HTTP **201** `car_status: "rejected"` sem `verified_at`.

### GET /api/v1/property (mais recente)

**Resultado**: OK — HTTP **200** devolve a property **mais recente** (`rejected` do CAR inativo), não a approved. Conforme §9 (`ORDER BY created_at DESC LIMIT 1`).

### SQL `properties` + SICAR mock

```sql
SELECT id, car_hmac, car_status, verified_at
FROM properties WHERE user_id = '01a0829f-b38c-786b-9a9a-6fbb3eb04bb1'
ORDER BY created_at;
```

| id | car_status | verified_at | car_hmac (64 hex) |
|---|---|---|---|
| `…bf9f…` | approved | preenchido | `29d7d436788e865016beed21e66a2ad33223fef6e5f0ec071418a324b19084c3` |
| `…c183…` | rejected | NULL | `5ee38c9404b4b1b5d76a21de081367e5ec203ed1a16fb4e9b6d2703c8bc60f3a` |
| `…c2c4…` | rejected | NULL | `85a90998cdb3958dc1af5ffc871cc35810db4a2ac4e1fc40dba480ba0c3773e8` |

- Colunas CAR: só `car_hmac` e `car_status` — **não existe** coluna `car` em claro.
- `car_hmac ILIKE '%E2EOK%' OR … '%4314100%'` → **0** linhas (HMAC não contém o CAR).
- `length(btrim(car_hmac)) = 64` nas 3 rows.

```sql
SELECT car_number, active FROM mock_sicar_cars
WHERE car_number IN ('RS4314100E2EOK1788897898','RS4314100E2ERJ1788897898');
```

`E2EOK…` active=**t**; `E2ERJ…` active=**f**. Status SICAR mock bate com `approved` / `rejected`.

### Chain mock (side effect com wallet já existente)

3 eventos `car_verification` em `mock_chain_events` para a pubkey: `approved: true` depois `false`/`false`. Payload **sem** o número do CAR.

---

## Falha com evidência: AUTH_PEPPER vazio no processo da API

> Achado do E2E **original** (container sem `AUTH_PEPPER`). **Superseded** pelo reteste abaixo: **CORRIGIDO**.

`car_hmac` **não** é o CAR em texto, mas o HMAC usava **pepper vazio**.

1. `.env` no host tem `AUTH_PEPPER=change-me-32-bytes-minimum-please-really`.
2. Container `agrobench-api` (`printenv`): **não** lista `AUTH_PEPPER` nem `AUTH_JWT_SECRET`. Só `PORT`, `DATABASE_URL`, `CONFIG_FILE` (+ vars Go). Compose `environment:` sobrescreve esses três; `env_file: .env` **não** chegou neste container (já estava Up; não recriei).
3. `config.dev.json` tem `"auth": { "pepper": "" }`.
4. Reprodução:

```text
HMAC-SHA256(key="", msg="RS4314100E2EOK1788897898")
  = 29d7d436788e865016beed21e66a2ad33223fef6e5f0ec071418a324b19084c3
  = car_hmac da property approved
```

Com o pepper do `.env` o hex seria outro (`ea6f4944…`). README promete pepper via `AUTH_PEPPER`. CPF/CAR ficam dicionário-atacáveis (formato conhecido) enquanto o pepper for `""`. Código (`HMACIdentifier`) não recusa pepper vazio.

Mesma classe de risco para JWT se `auth.jwt_secret` também está `""` (não exercido neste módulo além do MFA que funcionou com o secret efetivo).

---

## Edge cases

| Caso | Resultado |
|---|---|
| Catálogo vazio pré-seed | 200 + `null` |
| Campo extra `private_key` | 400 unknown field |
| Blob não base64 | 400 |
| Wallet duplicada | 409 |
| Export sem OTP | 403 `otp_required` |
| Export OTP errado | 401 |
| Região UUID inexistente | 404 |
| CAR duplicado | 409 |
| CAR inativo / inexistente | 201 + `rejected` |
| GET property | só a mais recente (não lista) |
| Chave privada no POST | nunca aceita |

---

## Reteste AUTH_PEPPER — 2026-09-08 17:26 (UTC-3)

**Resultado**: ✅ CORRIGIDO

Pré-condição: `docker-compose.yaml` injeta `.env` (`env_file` + `AUTH_PEPPER`). API já recriada. HMACs antigos no banco usaram pepper `""` — este reteste usou **usuário e CAR novos**.

### 1. Pepper no processo da API

```text
docker compose exec -T api printenv AUTH_PEPPER
→ presente / não vazio (valor não copiado neste relatório)
```

`AUTH_JWT_SECRET` também presente/não vazio. `GET /healthz` → HTTP 200 `{"status":"ok"}`.

### 2. Producer novo + MFA

| Campo | Valor |
|---|---|
| email | `e2e-retest-pepper-1788899140@agrobench.test` |
| phone | `+5554988914001` |
| cpf | `81788991400` |
| `user_id` | `01a082b3-7f1f-7c59-983d-70bb9665e900` |
| pubkey | `E2eRetestPep1788899140XXXXXXXXXX` (32 chars; sem chave privada) |
| CAR (inserido em `mock_sicar_cars`, active=t) | `RS-4314100-E2ERT1788899140` → norm `RS4314100E2ERT1788899140` |
| microrregião | Passo Fundo `01a0829e-2468-7495-88a6-052eaa12699a` |

Fluxo: `POST /auth/register` **201** (`mfa_required`) → `GET /admin/otp/{user_id}` **200** code `234027` purpose `login` → `POST /auth/mfa/verify` **200** (`expires_in` 900, refresh 64 hex).

### 3. Wallet + property

- `POST /api/v1/wallet` **201** — pubkey acima, `balance_usdc` 0, `reward_usdc` 0. `id` `01a082b3-7f42-7246-adb6-27cbbc140874`
- `POST /api/v1/property` **201** — `car_status: "approved"`, `verified_at: "2026-09-08T20:26:36Z"`, JSON **sem** CAR. `id` `01a082b3-7f4e-703b-8732-90ccfc07297e`
- `GET /api/v1/wallet` **200** — mesmo `id` / pubkey
- `GET /api/v1/property` **200** — mesmo `id` / `approved`

### 4. SQL `car_hmac` ≠ HMAC com pepper vazio

```sql
SELECT id, car_hmac, car_status, length(btrim(car_hmac))
FROM properties WHERE id = '01a082b3-7f4e-703b-8732-90ccfc07297e';
```

| Campo | Valor |
|---|---|
| `car_hmac` | `9573a3e61be8f3fb33ae859bd96a7ce7d45bd8651001679f85c48c53a5372177` (64 hex) |
| `car_status` | `approved` |
| leak do CAR no hex (`ILIKE '%E2ERT%'` / `'%4314100%'`) | **falso** |

Contradição (pepper `""`):

```text
HMAC-SHA256(key="", msg="RS4314100E2ERT1788899140")
  = 8f81f5827692267034983f923e1d081fbb87407c5a54e34939d764a1d5bcf636
  ≠ car_hmac
```

Confirmação positiva (HMAC calculado **dentro** do container com `$AUTH_PEPPER`; pepper **não** impresso):

```text
printf %s "RS4314100E2ERT1788899140" | openssl dgst -sha256 -hmac "$AUTH_PEPPER"
  = 9573a3e61be8f3fb33ae859bd96a7ce7d45bd8651001679f85c48c53a5372177
  = car_hmac
```

CPF do mesmo user: `cpf_hmac` também ≠ HMAC com pepper vazio e **bate** com HMAC do pepper do processo.

HMACs **antigos** (property approved do E2E original, CAR `RS4314100E2EOK1788897898`) continuam iguais a `HMAC-SHA256("", CAR)` = `29d7d436788e865016beed21e66a2ad33223fef6e5f0ec071418a324b19084c3`. Não foram rehashados.

---

## Recomendações

1. ~~Recriar o container da API com `.env` injetado (`AUTH_PEPPER`, `AUTH_JWT_SECRET`).~~ **Feito** — reteste OK. Ainda vale falhar o boot se `auth.pepper` / `jwt_secret` estiverem vazios (código `HMACIdentifier` ainda aceita pepper `""`).
2. App: após um CAR `rejected`, `GET /property` deixa de mostrar a property `approved` anterior — guardar o `id` do POST para commit.
3. Catálogo `null` vs `[]`: tratar os dois como vazio.
4. Rows de `properties` / `users` gravadas com pepper vazio (antes da recriação do container) não migram sozinhas — lookup por HMAC antigo quebra se alguém re-hashear.
