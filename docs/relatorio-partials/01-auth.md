# E2E Identity/Auth — 2026-09-08

**Status do módulo**: APROVADO (todos os endpoints do contrato §7 Identity: OK)

**Ambiente**: API `http://localhost:8080` (já no ar; healthz 200). Postgres `docker compose exec -T db psql -U agrobench -d agrobench`. SMS mock.

**Usuário de teste (producer)**:
- email: `e2e-auth-20260908200234@agrobench.test`
- phone: `+5554788897754`
- cpf: `28889775408` (11 dígitos, único)
- senha inicial: `senha-segura-1` → após recovery: `nova-senha-segura-2`
- `user_id`: `01a0829e-2bb9-710d-9d4a-db3c131fb758`

**Institution (só para provar login sem MFA)**:
- email: `e2e-inst-20260908200234@agrobench.test`
- cnpj: `38897754000169`
- `users.id`: `01a0829f-707b-75a5-8d41-28ba63ee8ee5`

**Admin**: seed `admin@agrobench.local` / `admin123`

Tokens JWT e refresh truncados abaixo (`…`).

---

## GET /healthz

**Resultado**: OK

```bash
curl -sS -D - http://localhost:8080/healthz
```

- HTTP **200** `{"status":"ok"}`

## GET /readyz

**Resultado**: OK (infra; necessário para Postgres)

```bash
curl -sS -D - http://localhost:8080/readyz
```

- HTTP **200** `{"database":"up","status":"ok"}`

---

## POST /api/v1/auth/register

**Resultado**: OK

```bash
curl -sS -D - -X POST http://localhost:8080/api/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"e2e-auth-20260908200234@agrobench.test","phone":"+5554788897754","cpf":"28889775408","password":"senha-segura-1"}'
```

- HTTP **201**
- Trecho: `{"mfa_required":true,"mfa_token":"eyJ…","user_id":"01a0829e-2bb9-710d-9d4a-db3c131fb758"}`
- Sem access/refresh (conforme contrato).

**SQL**

```sql
SELECT id, email, phone, role, mfa_enabled, status,
       length(cpf_hmac) AS hmac_len, cpf_hmac,
       (cpf_hmac = '28889775408') AS hmac_equals_plain_cpf,
       (password_hash LIKE '$argon2%') AS argon2
FROM users WHERE email = 'e2e-auth-20260908200234@agrobench.test';
```

- **ok**: 1 row, `role=producer`, `mfa_enabled=t`, `status=active`
- `cpf_hmac` = `6f266a9cd53c2d5b6c8c687df36abc4c30b764ce727be4966c08ea5c0b8412f6` (64 hex, **não** é o CPF em texto)
- `hmac_equals_plain_cpf = f`
- `password_hash` prefixo `$argon2id$v=`

```sql
SELECT purpose, length(code_hash), consumed_at IS NULL AS pending, attempts
FROM otp_codes WHERE user_id = '01a0829e-2bb9-710d-9d4a-db3c131fb758';
```

- **ok**: `purpose=login`, `code_hash` 64 hex, pending, attempts=0

### Negativos

| Caso | Status | Body |
|---|---|---|
| email/CPF duplicado | **409** | `CONFLICT` `email ou CPF já cadastrado` |
| campo extra `foo` | **400** | `INVALID_INPUT` `json: unknown field "foo"` |
| cpf `"123"` | **400** | `INVALID_INPUT` `cpf: min=11` |
| `{}` | **400** | `email: required; phone: required; cpf: required; password: required` |

---

## GET /api/v1/admin/otp/{user_id}

**Resultado**: OK

```bash
curl -sS -D - http://localhost:8080/api/v1/admin/otp/01a0829e-2bb9-710d-9d4a-db3c131fb758
```

- HTTP **200** `{"user_id":"01a0829e-2bb9-710d-9d4a-db3c131fb758","code":"705822","purpose":"login"}`
- Público (sem Bearer), SMS mock.

| Caso | Status | Detail |
|---|---|---|
| `not-a-uuid` | **400** | `user_id inválido` |
| UUID inexistente | **404** | `usuário não encontrado` |

**Obs. (esperado pelo contrato §4.1)**: depois de `recovery/start`, o GET ainda devolve `"purpose":"login"` porque o texto do SMS mock não contém `"recupera"`. O OTP no banco é `purpose=recovery`. O código em claro (`329743`) funcionou no confirm.

---

## POST /api/v1/auth/mfa/verify

**Resultado**: OK

Código errado:

```bash
curl -sS -D - -X POST http://localhost:8080/api/v1/auth/mfa/verify \
  -H 'Content-Type: application/json' \
  -d '{"mfa_token":"<jwt mfa>","code":"000000"}'
```

- HTTP **401** `UNAUTHORIZED` `código inválido`
- SQL: `attempts=1`, ainda pending

Código certo (`705822`):

- HTTP **200** `{"access_token":"eyJ…","refresh_token":"26831ce97bc5fc73…70f3","expires_in":900,"token_type":"Bearer"}`
- `refresh_token` 64 hex.

**SQL OTP** — `consumed=t` (consumed_at preenchido), attempts=1.

**SQL refresh**

```sql
SELECT length(token_hash), token_hash = '<refresh em claro>' AS hash_equals_plain,
       token_hash = sha256(refresh) AS hash_equals_sha256, revoked_at IS NULL
FROM refresh_tokens WHERE user_id = '…';
```

- `token_hash` 64 hex SHA-256 do refresh (**não** o token em claro): `hash_equals_plain_refresh=f`, `hash_equals_sha256_of_refresh=t`
- 1 token ativo, `expires_at > now()`

---

## GET /api/v1/me

**Resultado**: OK

```bash
curl -sS -D - http://localhost:8080/api/v1/me \
  -H 'Authorization: Bearer <access>'
```

- HTTP **200**
```json
{"id":"01a0829e-2bb9-710d-9d4a-db3c131fb758","email":"e2e-auth-20260908200234@agrobench.test","phone":"+5554788897754","role":"producer","mfa_enabled":true,"status":"active"}
```
- Sem CPF, sem wallet (contrato).

| Caso | Status | Detail |
|---|---|---|
| sem Authorization | **401** | `token ausente` |
| `Bearer ` vazio | **401** | `token ausente` |
| MFA JWT como access | **401** | `token inválido` |

Producer / institution / admin: os três `/me` autenticados devolveram a role correta (ver seções abaixo).

---

## POST /api/v1/auth/refresh

**Resultado**: OK

```bash
curl -sS -D - -X POST http://localhost:8080/api/v1/auth/refresh \
  -H 'Content-Type: application/json' \
  -d '{"refresh_token":"26831ce9…70f3","device":"web-chrome"}'
```

- HTTP **200** tokens **novos** (`refresh` diferente do anterior), `expires_in:900`

**SQL após rotação**

```sql
SELECT revoked_at IS NULL AS active, device FROM refresh_tokens WHERE user_id = '…' ORDER BY created_at;
```

| token | active | device | revoked |
|---|---|---|---|
| antigo | **f** | (vazio) | 20:03:55 |
| novo | **t** | `web-chrome` | — |

Reuso do refresh antigo:

- HTTP **401** `refresh token inválido`
- SQL: `still_active=0`, `revoked=2` (família inteira, contrato §1.4)
- Refresh “novo” também **401** depois do reuse.

---

## POST /api/v1/auth/login

**Resultado**: OK

### Producer (MFA obrigatório)

```bash
curl -sS -D - -X POST http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"e2e-auth-20260908200234@agrobench.test","password":"senha-segura-1"}'
```

- HTTP **200** `{"mfa_required":true,"mfa_token":"eyJ…","user_id":"01a0829e-…"}` — **sem** access/refresh
- SQL: novo OTP `login` pending (o anterior permanece consumed)

OTP mock após login: `{"code":"273185","purpose":"login"}` → mfa/verify **200** tokens.

Email/senha errados e email inexistente: ambos **401** `credenciais inválidas` (não vaza existência).

### Institution (`mfa_enabled=false`)

Criado via `POST /api/v1/institutions/register` (necessário para ter a role). Login:

- HTTP **200** `access_token` + `refresh_token` + `expires_in:900` — **sem** `mfa_required`
- `/me`: `role=institution`, `mfa_enabled=false`, `phone=+5500000000000`

### Admin (conta seed)

- HTTP **200** tokens diretos, sem MFA
- `/me`: `role=admin`, `mfa_enabled=false`, email `admin@agrobench.local`

---

## POST /api/v1/auth/logout

**Resultado**: OK

```bash
curl -sS -D - -X POST http://localhost:8080/api/v1/auth/logout \
  -H 'Content-Type: application/json' \
  -d '{"refresh_token":"2499ea06…4c23"}'
```

- HTTP **204** body vazio, `Content-Type: application/json`
- Token desconhecido (`"aa"*32`): também **204** (idempotente)

**SQL**: o refresh usado ficou `revoked_at=2026-09-08 20:03:56` UTC; refresh subsequente **401**.

**Obs. (esperado)**: `GET /me` com o **access** ainda válido após logout → **200**. Logout só revoga aquele refresh, não o JWT de 15 min.

---

## POST /api/v1/auth/recovery/start

**Resultado**: OK

CPF+phone errados (phone `+5554999000999`):

- HTTP **204** vazio (não vaza conta)

CPF+phone certos:

- HTTP **204** vazio

```sql
SELECT purpose, consumed_at IS NULL AS pending FROM otp_codes
WHERE user_id = '01a0829e-2bb9-710d-9d4a-db3c131fb758' ORDER BY created_at DESC LIMIT 5;
```

- **ok**: linha nova `purpose=recovery`, pending, attempts=0

GET OTP mock: `code=329743` (purpose JSON `"login"` — ver obs. no OTP).

---

## POST /api/v1/auth/recovery/confirm

**Resultado**: OK

Código errado `000000`: **401** `código inválido`

Código certo + `new_password=nova-senha-segura-2`:

- HTTP **204** vazio
- SQL refresh: `still_active=0`, `revoked=4` (revogou **todos**)
- SQL OTP recovery: `consumed=t`, attempts=1
- Login com senha antiga: **401** `credenciais inválidas`
- Login com senha nova → MFA → `/me` **200** mesmo perfil

---

## Resumo por endpoint

| Endpoint | Resultado |
|---|---|
| `POST /api/v1/auth/register` | OK |
| `POST /api/v1/auth/login` | OK (producer MFA; institution e admin com tokens) |
| `POST /api/v1/auth/mfa/verify` | OK |
| `POST /api/v1/auth/refresh` | OK (rotação + reuse revoga família) |
| `POST /api/v1/auth/logout` | OK (204 idempotente) |
| `POST /api/v1/auth/recovery/start` | OK (204 inclusive par errado) |
| `POST /api/v1/auth/recovery/confirm` | OK (troca senha + revoga refresh) |
| `GET /api/v1/me` | OK |
| `GET /api/v1/admin/otp/{user_id}` | OK |

## Falhas deste módulo

Nenhuma falha bloqueante. Comportamentos abaixo batem com o contrato, não são bugs:

1. `GET /admin/otp` reporta `purpose=login` mesmo no fluxo recovery (SMS mock / §4.1).
2. Logout não invalida o access JWT vigente.
3. Poll inicial de `/healthz` em sandbox Cursor falhou (connection refused); com permissão de rede a API já estava Up ~2h.

## Edge cases cobertos

- duplicata 409, unknown fields 400, CPF curto 400, body vazio 400
- MFA code errado incrementa `attempts`
- `/me` sem token / MFA-as-access
- refresh reuse → família revogada
- logout token inexistente 204
- recovery start com phone errado 204
- senha antiga após recovery 401
- institution/admin login sem `mfa_token`
