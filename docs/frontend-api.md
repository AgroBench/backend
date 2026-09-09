# Contrato de consumo HTTP — AgroBench API

Este documento é o **contrato de consumo para o app/frontend**. Foi derivado do código das rotas, types, handlers, use cases, `internal/apperrors`, `internal/core/httpx`, `internal/core/auth`, migrations e da coleção Bruno em `docs/bruno/`.

- Fonte de verdade de **arquitetura** do backend: [`README.md`](../README.md).
- Se este doc, o README e o código divergirem, **prevalecem para o cliente HTTP**: (1) o código das rotas/handlers/types; (2) este documento (derivado do código).
- Quem lê é outro agente Cursor: implemente o cliente **só com o que está aqui**. Não invente campos, query params nem rotas.

Coleção Bruno (exemplos executáveis): `docs/bruno/`. Payload agronômico (hash + cifra): [`docs/payload.md`](payload.md) e `internal/core/payload`.

---

## 1. Convenções globais

### 1.1 Base URL, prefixo, JSON

| Item | Valor real no código |
|---|---|
| Base (dev) | `http://localhost:8080` |
| Prefixo da API | `/api/v1` |
| Health | **fora** do prefixo: `GET /healthz`, `GET /readyz` |
| `Content-Type` de toda resposta | `application/json` (middleware `httpx.ContentTypeJSON` em **todas** as rotas, inclusive 204) |
| Body de request | JSON UTF-8. Campos JSON e identificadores em **inglês**. Mensagens de erro em **português**. |
| Campos desconhecidos | **Rejeitados** (`json.Decoder.DisallowUnknownFields`). Extra key → `400` `INVALID_INPUT`. |
| Mais de um valor JSON no body | `400` `INVALID_INPUT` (`body deve conter um único objeto JSON`) |
| Body vazio em POST que espera objeto | `400` `INVALID_INPUT` (`body vazio`) |
| Tamanho máximo do body | 2 MiB (`httpx.Decode` + `middleware.RequestSize`) |
| Timeout por request | 30 s (`middleware.Timeout`) |
| Throttle concorrente | 200 (`middleware.Throttle`) |
| Rate limit | **120 req/min por IP** (`httprate.LimitByIP`). Estouro → **429**. Esse 429 **não** passa pelo catálogo `apperrors` (o limiter responde antes do handler). |

Sobe com `make dev`. Porta: `server.http.port` = `8080`.

### 1.2 Headers

**Request**

| Header | Quando | Notas |
|---|---|---|
| `Content-Type: application/json` | Todo POST/PATCH com JSON | Sem body (close cycle, seed, approve, confirm, distribute) o header é opcional. |
| `Authorization: Bearer <access_token>` | Rotas autenticadas | Prefixo literal `Bearer ` (espaço). Sem o prefixo ou token vazio → `401` `UNAUTHORIZED` `detail: "token ausente"`. |
| `X-Request-Id` | Opcional | Chi `middleware.RequestID`. Se o cliente enviar, é reutilizado; senão a API gera. CORS permite o header. |
| `Stripe-Signature` | Só `POST /webhooks/payment` | No mock é **ignorada**. No Stripe real é obrigatória. |
| `Accept` | Opcional | Permitido no CORS. |

**Response**

| Header | Valor |
|---|---|
| `Content-Type` | `application/json` (sempre) |
| `X-Request-Id` | Exposto no CORS (`ExposedHeaders`) |

Não há cookies de sessão. `CORS AllowCredentials: false`. Não envie cookies; use Bearer.

### 1.3 CORS (browser)

Em `pkg/adapter/rest/rest.go`:

- Origins: `https://*`, `http://*` (localhost de qualquer porta ok)
- Methods: `GET POST PUT PATCH DELETE OPTIONS`
- Headers: `Accept`, `Authorization`, `Content-Type`, `X-Request-Id`
- Credentials: **false** (não dá para `fetch` com `credentials: 'include'` esperando cookie)

Preflight OPTIONS é atendido pelo chi-cors.

### 1.4 Auth em uma frase

- Access: JWT HS256, claim `purpose=access`, TTL **15 min** (`auth.access_ttl_minutes`).
- Refresh: opaco, 32 bytes em **hex** (64 chars), hash SHA-256 no banco, TTL **30 dias**, **rotação a cada uso**. Reuso de refresh já revogado → revoga **todos** os refresh do user.
- MFA: JWT separado `purpose=mfa`, TTL = OTP (**5 min**). **Não** vale como `Authorization` das rotas. `auth.Require` chama `ParseAccess`; MFA token → `401` `token inválido`.
- OTP: 6 dígitos, 5 tentativas, 5 min. Hash SHA-256 em `otp_codes`. SMS mock **não envia SMS de verdade**.
- Roles literais: `producer` \| `institution` \| `admin`.
- MFA **obrigatório** para `producer` (`mfa_enabled=true` no cadastro). `institution` e `admin` logam **só com senha** (login devolve tokens direto, sem `mfa_token`).
- `GET /me` aceita **qualquer** role autenticada (`auth.Require()` sem filtro).
- **Não existe** `auth.RequireSubscription()` no código das rotas (o README §10 cita; o router de benchmark **não** chama). Ver §15.

Claims do access JWT (não decodificar no cliente além de `exp`/`sub`/`role` se quiser UX; a API não exige que o app leia o JWT):

```text
sub        = user UUID
role       = producer | institution | admin
wallet     = pubkey da wallet se existir no momento da emissão; omitido/vazio se ainda não cadastrou
purpose    = "access"
jti        = UUID v7
iss        = "agrobench"
iat, exp
```

Depois de `POST /wallet`, o access **já emitido** continua sem `wallet` até o próximo `POST /auth/refresh` ou login. As rotas **não** leem a claim `wallet`; buscam a wallet no banco pelo `sub`. Não precisa re-logar só por causa da claim.

### 1.5 IDs, datas, números, paginação

| Tema | Contrato |
|---|---|
| IDs | UUID **v7**, string canônica `xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx`. Path params inválidos → `400` `INVALID_INPUT` (`id inválido` / `user_id inválido` / `cycle inválido`). |
| Datas de **response** | RFC3339 UTC, ex. `2025-09-01T00:00:00Z` (`time.RFC3339`). `exported_at` da wallet usa `2006-01-02T15:04:05Z` (também UTC com `Z`). |
| Datas de **request** | `encoding/json` em `time.Time` → RFC3339. Ex.: `"opens_at": "2025-09-01T00:00:00Z"`. |
| Datas do payload agronômico | `YYYY-MM-DD` (`planting_date`, `harvest_date`). **Não** RFC3339. |
| Dinheiro na API | `float64` em USDC (`balance_usdc`, `reward_usdc`, `amount_usdc`, preços). Interno é micro-USDC; o JSON já vem convertido. |
| Hash SHA-256 | hex minúsculo, **64 caracteres** (`commit.hash`, `commit_hash`). |
| Chaves cripto | hex. `box_public_key` x25519 = 32 bytes = 64 hex. `signing_public_key` ed25519 = 32 bytes = 64 hex. |
| Blob / ciphertext | **base64 std** (`encrypted_blob`, `ciphertext`). Não é base64url. |
| Paginação | **`go-paginate` não é usado em nenhum handler.** Não envie `page`, `per_page`, `limit`, `offset`. Listas devolvem o array JSON inteiro. Array vazio pode ser `[]` **ou** `null` (slice Go nil) — trate os dois como vazio. |

### 1.6 Idioma e validação de input

- Tags `validate` do go-playground. Falha → `400` `INVALID_INPUT`, `detail` no formato `campo: tag` ou `campo: tag=param`, campos em **lowercase do nome Go**, não do JSON. Ex.: `email: required`; `code: len=6`; `password: min=8`.
- Telefone: tag `e164` (ex. `+5554999990001`).
- Email: `email`, max 255 no register; login normaliza `ToLower(TrimSpace)`.
- Senha: min 8, max 72 (limite do argon2id).
- CPF no register: `min=11,max=14` **no JSON**; depois a API exige 11 dígitos após `NormalizeIdentifier` (remove pontuação). Aceita `529.982.247-25` ou `52998224725`.
- CNPJ: `min=14,max=18` no JSON; depois exige 14 dígitos normalizados.
- CAR: `min=8,max=60`. Normalizado (sem hífens) para lookup SICAR e HMAC.

`NormalizeIdentifier`: uppercase, remove tudo que não é `[A-Z0-9]`. `"123.456.789-01"` → `"12345678901"`; `"RS-4314100-AAA…"` → `"RS4314100AAA…"`.

### 1.7 Formato de erro HTTP

Toda `*AppError` vira:

```json
{
  "code": "UNAUTHORIZED",
  "message": "Não autorizado",
  "detail": "credenciais inválidas"
}
```

`detail` é omitido se vazio (`omitempty`). `message` vem do catálogo (português, genérica). `detail` é a mensagem acionável para o app.

404/405 de rota (não de recurso) **não** usam `detail`:

```json
{ "code": "NOT_FOUND", "message": "Rota não encontrada" }
{ "code": "METHOD_NOT_ALLOWED", "message": "Método não permitido" }
```

Catálogo (`internal/apperrors/catalog.go`):

| `code` | HTTP | `message` |
|---|---|---|
| `INVALID_INPUT` | 400 | Requisição inválida |
| `UNAUTHORIZED` | 401 | Não autorizado |
| `FORBIDDEN` | 403 | Acesso negado |
| `NOT_FOUND` | 404 | Recurso não encontrado |
| `CONFLICT` | 409 | Conflito ao processar requisição |
| `EXTERNAL_DEPENDENCY_ERROR` | 502 | Erro em dependência externa |
| `INTERNAL_ERROR` | 500 | Erro interno do servidor |
| `CONFIG_MISSING` | 500 | Configuração ausente |

Exceções que **não** seguem só o trio `code/message/detail`:

- `GET /benchmark/me` 403: body extra `cycles_validated`, `cycles_required` (ver §9 e §15).
- `GET /wallet/export` sem `code`: 403 `{ "code": "FORBIDDEN", "message": "OTP enviado", "otp_required": true }`.
- `POST /admin/jobs/run/{kind}` job desconhecido: 400 `{ "code": "INVALID_INPUT", "message": "job desconhecido" }` (sem `detail`).
- Fila down nesse endpoint: 503 `{ "code": "INTERNAL_ERROR", "message": "fila indisponível" }`.

Token expirado: `401` `UNAUTHORIZED` `detail: "token expirado"`. Token lixo / MFA usado como access: `detail: "token inválido"`. Role errada: `403` `FORBIDDEN` `detail: "perfil insuficiente"`.

### 1.8 O que o frontend NUNCA envia

- Chave **privada** da wallet em qualquer campo. Só `pubkey` + `encrypted_blob` (blob já cifrado **pelo app**, com segredo que só o usuário tem).
- Payload agronômico em claro no reveal. Reveal = **ciphertext** base64 (sealed box). O plaintext nunca vai na API.
- CPF / CAR / CNPJ **depois** do cadastro, salvo recovery (`cpf`+`phone`) e o `car` no `POST /property`. A API persiste só HMAC. Não reenvie CPF em `/me`, commit, etc.
- `password` em GET query.
- Segredos de infra (`JWT_SECRET`, `AUTH_PEPPER`, chave x25519 do enclave).

### 1.9 Contas e dados de seed (dev/demo)

`make seed` (base) ou `POST /admin/seed/demo` / `make seed-demo`. Sem seed: `GET /micro-regions` e `GET /cultures` vêm vazios; não há admin.

| Conta | Email | Senha | Role | MFA |
|---|---|---|---|---|
| Admin (`.env` `ADMIN_EMAIL` / `ADMIN_PASSWORD`) | default `admin@agrobench.local` | default `admin123` | `admin` | não |
| Demo produtor | `produtor@agrobench.local` | `demo12345` | `producer` | **sim** — pubkey Devnet `FXsin7UZTGrix1cEe1QpMDFz3a8cDHzVK7h2oisjpzf3` |
| Demo instituição | `instituicao@agrobench.local` | `demo12345` | `institution` | não |

- 35 microrregiões IBGE do RS. Demo oficial = Passo Fundo `ibge_code: "43010"`; seed rica também em Carazinho, Não-Me-Toque, Ijuí, Cruz Alta, Erechim, Santa Rosa, Vacaria, Santo Ângelo.
- Culturas: `soybean` (Soja), `corn` (Milho), `wheat` (Trigo).
- CARs mock (após `NormalizeIdentifier`): `RS-4314100-AAA` + 17 dígitos, `i=1..20`. `i<=17` ativos (aprovam); `i>=18` inativos (property nasce `rejected`). Ex. ativo: `RS-4314100-AAA00000000000000002`. O seed-demo vincula o CAR `…0001` ao produtor oficial. Agricultores extras usam `…0100` em diante (outros municípios) — **não** ocupam 2–17.
- Seed-demo: 1 produtor oficial, pubkey Solana real `FXsin7UZTGrix1cEe1QpMDFz3a8cDHzVK7h2oisjpzf3`. `GET /wallet` lê saldo na Devnet. Agricultores extras (`seed.farmer.NN@agrobench.local`) têm pubkey off-chain só para média — não airdrop na Devnet. Não existem mais `produtor01`–`14`.
- Seed-demo: ciclos históricos **já `aggregated`** (várias regiões × culturas) + janela **`2026/27` aberta** em Passo Fundo × soja para commit ao vivo. Instituição: Cotrijal, plano regional (5 microrregiões).
- `LockStake` na Devnet, com programa, **debita** 10 USDC da ATA do produtor (vault PDA). Sem `CHAIN_SOLANA_PROGRAM_ID`, lock no commit ainda é Memo.

---

## 2. Índice de rotas

Paths abaixo já incluem `/api/v1`, exceto health. Auth: `público` = sem Bearer; `Bearer` = access JWT; role entre parênteses se filtrada.

| Método | Path | Auth | Handler |
|---|---|---|---|
| `GET` | `/healthz` | público | liveness |
| `GET` | `/readyz` | público | DB ping |
| `POST` | `/api/v1/auth/register` | público | producer + OTP login |
| `POST` | `/api/v1/auth/login` | público | MFA challenge **ou** tokens |
| `POST` | `/api/v1/auth/mfa/verify` | público (`mfa_token` no body) | tokens |
| `POST` | `/api/v1/auth/refresh` | público (refresh no body) | tokens novos |
| `POST` | `/api/v1/auth/logout` | público (refresh no body) | 204 |
| `POST` | `/api/v1/auth/recovery/start` | público | 204 + OTP recovery |
| `POST` | `/api/v1/auth/recovery/confirm` | público | 204 |
| `GET` | `/api/v1/me` | Bearer (qualquer role) | perfil |
| `GET` | `/api/v1/admin/otp/{user_id}` | **público**, só se `adapters.sms=mock` | OTP em claro |
| `POST` | `/api/v1/wallet` | Bearer `producer` | cria wallet |
| `GET` | `/api/v1/wallet` | Bearer `producer` | saldo |
| `GET` | `/api/v1/wallet/export` | Bearer `producer` + OTP query | blob |
| `GET` | `/api/v1/wallet/payouts` | Bearer `producer` | payouts do pool |
| `GET` | `/api/v1/micro-regions` | público | catálogo |
| `GET` | `/api/v1/cultures` | público | catálogo |
| `POST` | `/api/v1/property` | Bearer `producer` | CAR |
| `GET` | `/api/v1/property` | Bearer `producer` | 1 property (a mais recente) |
| `GET` | `/api/v1/cycles` | público | lista |
| `POST` | `/api/v1/admin/cycles` | Bearer `admin` | abre ciclo |
| `POST` | `/api/v1/admin/cycles/{id}/close` | Bearer `admin` | fecha + job aggregate |
| `GET` | `/api/v1/enclave/public-key` | público | x25519 hex |
| `POST` | `/api/v1/contributions/commit` | Bearer `producer` | commit + stake |
| `POST` | `/api/v1/chain/stake/lock-tx` | Bearer `producer` | monta lock_stake (front assina) |
| `POST` | `/api/v1/chain/stake/lock-submit` | Bearer `producer` | co-assina treasury e envia |
| `POST` | `/api/v1/contributions/{id}/reveal` | Bearer `producer` | ciphertext + job validate |
| `POST` | `/api/v1/contributions/{id}/release-stake/tx` | Bearer `producer` | monta release_stake (front assina) |
| `POST` | `/api/v1/contributions/{id}/release-stake/submit` | Bearer `producer` | co-assina treasury e envia |
| `GET` | `/api/v1/contributions` | Bearer `producer` | lista da wallet |
| `GET` | `/api/v1/contributions/{id}` | Bearer `producer` | **não** checa dono |
| `GET` | `/api/v1/benchmark/me` | Bearer `producer` | painel grátis |
| `GET` | `/api/v1/benchmark/report` | Bearer `institution` | relatório (ver limitações) |
| `POST` | `/api/v1/institutions/register` | público | user institution + pending |
| `GET` | `/api/v1/institutions/me` | Bearer `institution` | status |
| `POST` | `/api/v1/institutions/subscribe` | Bearer `institution` | checkout |
| `POST` | `/api/v1/webhooks/payment` | público (assinatura Stripe ou JSON mock) | confirma |
| `POST` | `/api/v1/admin/institutions/{id}/approve` | Bearer `admin` | 204 |
| `POST` | `/api/v1/admin/institutions/{id}/reject` | Bearer `admin` | 204 |
| `POST` | `/api/v1/admin/payments/{id}/confirm` | Bearer `admin` | 204 mock webhook |
| `GET` | `/api/v1/pool/periods` | público | lista |
| `GET` | `/api/v1/pool/periods/{month}` | público | `YYYY-MM` |
| `POST` | `/api/v1/admin/pool/distribute` | Bearer `admin` | query `month` → 202 |
| `POST` | `/api/v1/admin/seed/demo` | Bearer `admin` | seed banca |
| `POST` | `/api/v1/admin/chain/initialize` | Bearer `admin` | `initialize` do programa (uma vez) |
| `POST` | `/api/v1/admin/jobs/run/{kind}` | Bearer `admin` | só `distribute_pool` |

**Não existem** (README ou types que o agente pode achar): `POST /wallet/export` (é GET), mint de USDC, listagem de payments, listagem de instituições, `RequireSubscription`, cron HTTP do pool, `GET /benchmark/report` com filtros `region`/`culture` (ignorados).

---

## 3. Máquinas de estado (literais JSON)

### 3.1 User `status`

`active` | `disabled`

Login / MFA recusam não-`active` com o mesmo `401` de senha errada (`credenciais inválidas` / `conta desativada` só no MFA se o token MFA é válido e a conta foi desabilitada).

### 3.2 User `role`

`producer` | `institution` | `admin`

### 3.3 OTP `purpose`

`login` | `recovery`

Export de wallet também usa purpose **`login`** (não há purpose `export`).

### 3.4 Property `car_status`

`pending` | `approved` | `rejected`

No mock a verificação SICAR é **síncrona** no `POST /property`: nasce `approved` (CAR existe e `active`) ou `rejected` (inexistente ou inativo). `pending` existe no CHECK do banco; o mock não deixa a property em `pending`.

Commit exige `approved`.

### 3.5 Cycle `status`

`open` | `closed` | `aggregated`

`IsOpen(now)` no commit: `status == open` **e** `opens_at <= now < closes_at`. Ciclo `open` mas fora da janela → `409` `ciclo não está aberto`.

Close: só de `open` → `closed`, senão `409` `ciclo não está aberto`. Enfileira `aggregate_cycle`. Worker grava `aggregates` e marca `aggregated`.

### 3.6 Contribution `status`

```text
committed → revealed → validating → accepted
                                   ↘ rejected
```

| status | Quem seta | O app faz |
|---|---|---|
| `committed` | `POST .../commit` | Guardar `id`, mostrar “envie o reveal” |
| `revealed` | `POST .../reveal` imediato | Poll `GET .../{id}` |
| `validating` | worker `validate_contribution` | Continuar poll |
| `accepted` | worker (veredito ok + atestação + reward) | Mostrar sucesso; reward em `GET /wallet` |
| `rejected` | worker | `reject_reason` genérico (ex. `validação rejeitada`). **1 nova tentativa** de commit no mesmo ciclo (attempt=2). Stake **não** é liberado. |

Única contribuição **ativa** por `(cycle_id, wallet_id)`: índice unique `WHERE status <> 'rejected'`. Segunda ativa → `409` `já existe contribuição ativa neste ciclo`.

`attempt`: `1` ou `2`. `2` se já existe alguma `rejected` no par ciclo×wallet. Stake (`LockStake` 10 USDC) **só na attempt 1**.

O código **não** impede um 3º commit se os dois anteriores estão `rejected` (o unique só barra não-rejected). O README promete 1 retry; o CHECK SQL aceita só `attempt IN (1,2)` mas outro insert `attempt=2` rejected+nova é possível. Para o app: trate 1 retry como regra de produto; se a API aceitar o 3º, `409` ou `23514` mapeado para `CONFLICT`.

GET contribuição **não** devolve veredito interno, `checks`, ciphertext, nem métricas. Só o struct de output (§8).

### 3.7 Stake `status`

`locked` | `released`

Release quando `ConsecutiveAccepted >= stake.release_after_cycles` (default **2**), mesma cultura × região, `opens_at DESC`, parando no primeiro não-`accepted`. Rejeição zera a contagem.

Com `CHAIN_SOLANA_PROGRAM_ID`, o worker **não** libera (log `ErrNeedsCoSign`); o app chama `POST /contributions/{id}/release-stake/tx` + `.../submit`. Sem program id, `ReleaseStake` ainda é Memo da treasury. Lock na attempt 1: `POST /chain/stake/lock-tx` + `lock-submit` (ver §11), depois `stake_tx` no commit.

### 3.8 Institution `status`

`pending` | `approved` | `rejected`

Subscribe exige `approved`. Register cria `pending`.

### 3.9 Subscription `status`

`pending` | `active` | `expired` | `cancelled`

`POST /institutions/subscribe` insere `pending`. Confirmação de pagamento (webhook ou admin confirm) → `active`. `period_end` = agora + 1 mês no insert. Plano: `regional` | `national`.

### 3.10 Pool period `status`

`open` | `distributed` | `carried`

`month` char(7) `YYYY-MM`. Sem acessos a relatório no mês → `carried` (net acumula conceitualmente; o worker atual grava o período com esse status).

### 3.11 Contribution `level`

`basic` | `intermediate` | `advanced`

Multiplicadores de recompensa (config): `1` / `2` / `3.5` × `rewards.base_usdc` (5) → 5 / 10 / 17.5 USDC.

---

## 4. Como pegar o OTP no mock (obrigatório na demo)

Não há Twilio. Três jeitos, o **oficial para o app** é o 1.

### 4.1 `GET /api/v1/admin/otp/{user_id}` (recomendado)

- **Público** (sem Bearer). Só é **registrado** se `adapters.sms=mock` (config default). Se SMS real, a rota **não existe** → 404 `Rota não encontrada`.
- Lê `SmsSender` mock `LastTo(user.Phone)` (outbox **in-memory**, últimas 200 msgs). Extrai `\b(\d{6})\b` do body.
- **Não lê a tabela `otp_codes`** (lá só tem hash). Reiniciar a API zera o outbox → `404` `nenhum OTP pendente` mesmo com OTP válido no banco.
- `purpose` no JSON: `"login"` ou `"recovery"` se o texto contiver `"recupera"`. A mensagem real é `AgroBench: seu código é %s. Válido por 5 minutos.` → **sempre `"login"`**, inclusive no recovery. Ignore `purpose` para branching; use o contexto do fluxo.

```http
GET /api/v1/admin/otp/0193a0c2-7e1b-7c11-8f00-aaaaaaaaaaaa
```

```json
{
  "user_id": "0193a0c2-7e1b-7c11-8f00-aaaaaaaaaaaa",
  "code": "482193",
  "purpose": "login"
}
```

`user_id` vem no body de register/login (`MFAChallenge`). Sem user_id não tem como chamar este GET.

Erros: `400` user_id inválido; `404` usuário inexistente ou sem SMS no outbox.

### 4.2 Log da API

```text
SMS (mock)  to=+5554999990001  body="AgroBench: seu código é 482193. Válido por 5 minutos."
```

`slog.Info` em `pkg/adapter/sms/mock`. Útil para humano; o agente de frontend deve usar 4.1.

### 4.3 `LastTo` (só testes Go)

Não é HTTP. Não chame.

Fluxo Bruno: register → `user_id` + `mfa_token` → GET OTP → mfa/verify.

---

## 5. Payload agronômico (commit-reveal)

Fonte: [`docs/payload.md`](payload.md), `internal/core/payload`. O servidor **não canonicaliza** no commit: `commit_hash = sha256(bytes_exatos_do_plaintext)`. O enclave, no reveal, decifra e faz o mesmo SHA-256.

### 5.1 O que o app faz (ordem obrigatória)

1. Montar o envelope JSON (abaixo). `nonce` = hex aleatório, **mínimo 32 chars** (16 bytes). Sem nonce o hash de um basic é brute-forceável.
2. Serializar **uma vez** para bytes. Se re-serializar, usar o equivalente a `payload.Canonicalize`: chaves de objeto **ordenadas**, **sem espaços**. `JSON.stringify` do JS **não** ordena chaves — implemente canonicalização (sort recursivo + `JSON.stringify` sem espaço) **ou** guarde o `Uint8Array` original.
3. `hash = SHA-256(bytes)` em hex 64 chars. **Guardar os mesmos bytes** até o reveal.
4. `POST /contributions/commit` com `cycle_id`, `property_id`, `level`, `hash`.
5. `GET /enclave/public-key` → `box_public_key` hex x25519.
6. Cifrar os **mesmos bytes** com **NaCl sealed box** (`crypto_box_seal` / `tweetnacl-sealedbox-js` / `nacl.sealedbox`): output = `ephemeral_pub(32) || ciphertext`.
7. `POST /contributions/{id}/reveal` com `{ "ciphertext": "<base64 std>" }`.

Nunca mande o JSON em claro. Nunca hasheie um stringify diferente do que foi selado.

Interop: `golang.org/x/crypto/nacl/box` `SealAnonymous` = libsodium `crypto_box_seal`.

### 5.2 Envelope

```json
{
  "version": 1,
  "level": "basic",
  "cycle_id": "0193b000-0000-7000-8000-000000000001",
  "nonce": "0123456789abcdef0123456789abcdef",
  "data": {}
}
```

`version` deve ser `1`. `level` ∈ `basic|intermediate|advanced` e **igual** ao `level` do commit. `cycle_id` **igual** ao do commit. Campos extra em `data` → schema inválido (rejeição no enclave, não 400 do HTTP de reveal).

### 5.3 `data` por nível

**basic** (recompensa 1×)

```json
{ "culture_code": "soybean", "area_ha": 50, "total_cost_brl": 250000 }
```

| Campo | Tipo | Validate |
|---|---|---|
| `culture_code` | string | required, lowercase, 2–30. Use o `code` de `GET /cultures` (`soybean`, não `"Soja"`). |
| `area_ha` | number | required, `>0`, `<=100000` |
| `total_cost_brl` | number | required, `>0` (total da safra, não por ha) |

Faixa mock CONAB Passo Fundo × soja: `area_ha` 1–5000, `total_cost_ha` (= total/área) 2000–8000. `250000/50=5000` passa. Fora da faixa → contribuição `rejected` (HTTP do reveal ainda é 200).

**intermediate** = basic + 

```json
{
  "culture_code": "soybean",
  "area_ha": 50,
  "total_cost_brl": 250000,
  "cost_by_input": {
    "fertilizer_brl": 60000,
    "pesticide_brl": 41000,
    "seed_brl": 25000,
    "fuel_brl": 15000,
    "labor_brl": 20000
  },
  "yield_sacks_ha": 72,
  "planting_date": "2025-10-15",
  "harvest_date": "2026-03-01",
  "suppliers": {
    "fertilizer": "Fornecedor A",
    "pesticide": "Fornecedor B",
    "seed": "Fornecedor C",
    "fuel": "Fornecedor D"
  }
}
```

`planting_date` / `harvest_date`: `datetime=2006-01-02`. `yield_sacks_ha`: `>0`, `<=500`. Soja Passo Fundo: 20–90.

**advanced** = intermediate +

```json
{
  "soil_type": "latossolo",
  "rotation_history": ["corn", "wheat"],
  "irrigation": { "used": true, "system": "center_pivot" },
  "pest_events": [{ "name": "ferrugem", "management": "fungicida x" }],
  "climate_losses": [{ "event": "drought", "area_pct": 12.5 }],
  "mechanization": { "type": "own", "machinery": ["colheitadeira"] }
}
```

| Campo | Enums / limites |
|---|---|
| `irrigation.system` | `center_pivot` \| `drip` \| `sprinkler` \| `furrow` \| `other` |
| `climate_losses[].event` | `drought` \| `hail` \| `frost` \| `flood` \| `heat_wave` \| `other` |
| `mechanization.type` | `own` \| `outsourced` \| `mixed` |
| `rotation_history` | 1–10 itens, lowercase |

Checks no enclave (nomes; o GET HTTP **não** os devolve): `decrypt`, `hash_matches_commit`, `schema_valid`, `level_matches`, `cycle_matches`, `reference_range:<metric>`.

Métricas que saem do enclave (não no GET contribuição; vão a `aggregates` / painel): `area_ha`, `total_cost_ha`, e nos níveis maiores `fertilizer_cost_ha`, `pesticide_cost_ha`, `seed_cost_ha`, `fuel_cost_ha`, `labor_cost_ha`, `yield_sacks_ha`, `cycle_days`, `irrigation` (0/1), `climate_loss_pct`, `pest_events`, `mechanization_own` (0/1). Painel grátis do produtor: **só** `area_ha` e `total_cost_ha`.

---

## 6. Infra

### `GET /healthz`

- Auth: público. Sem `/api/v1`.
- 200 `{ "status": "ok" }`

### `GET /readyz`

- 200 `{ "status": "ok", "database": "up" }`
- 503 `{ "status": "degraded", "database": "down" }`

---

## 7. Identity

### `POST /api/v1/auth/register`

Auth: público. Cria `role=producer`, `mfa_enabled=true`, `status=active`. Dispara OTP purpose `login`. **Não** devolve access/refresh.

**Body**

```json
{
  "email": "produtor@agrobench.local",
  "phone": "+5554999990001",
  "cpf": "529.982.247-25",
  "password": "senha-segura-1"
}
```

| Campo | Required | Regras |
|---|---|---|
| `email` | sim | email, max 255; persistido lowercase |
| `phone` | sim | e164 |
| `cpf` | sim | 11–14 chars; 11 dígitos após normalizar |
| `password` | sim | 8–72 |

**201**

```json
{
  "mfa_required": true,
  "mfa_token": "<jwt purpose=mfa>",
  "user_id": "0193a0c2-7e1b-7c11-8f00-aaaaaaaaaaaa"
}
```

Guarde `user_id` (OTP mock) e `mfa_token` (5 min).

**Erros:** `400` CPF inválido / validate; `409` `email ou CPF já cadastrado`; `502` `falha ao enviar SMS`.

Side effect: SMS mock + linha em `otp_codes`.

### `POST /api/v1/auth/login`

**Body**

```json
{
  "email": "produtor@agrobench.local",
  "password": "senha-segura-1"
}
```

**200 — producer com MFA** (mesmo shape do register):

```json
{
  "mfa_required": true,
  "mfa_token": "<jwt>",
  "user_id": "..."
}
```

**200 — institution / admin** (`mfa_enabled=false`):

```json
{
  "access_token": "<jwt purpose=access>",
  "refresh_token": "<64 hex>",
  "expires_in": 900,
  "token_type": "Bearer"
}
```

`expires_in` em **segundos** (15×60=900). O cliente **deve** ramificar: se `mfa_required === true` (ou se existe `mfa_token` e não `access_token`), ir para MFA; senão guardar tokens.

Email/senha errados, user inexistente, `disabled`: **sempre** `401` `credenciais inválidas` (não vaza existência).

Side effect (producer): novo OTP; OTP login anterior do user é invalidado.

### `POST /api/v1/auth/mfa/verify`

**Body**

```json
{
  "mfa_token": "<do register/login>",
  "code": "482193"
}
```

`code`: exatamente 6 dígitos (`len=6,numeric`).

**200** — `Tokens` (igual login sem MFA).

**Erros:** `401` `mfa_token inválido` / `mfa_token expirado` / `conta desativada` / `código inválido` / `código expirado`. 5 mismatches → OTP consumido, ainda `código inválido`.

Side effect: OTP consumido; refresh criado; access inclui `wallet` se já existir wallet.

### `POST /api/v1/auth/refresh`

**Sem** Bearer. Body:

```json
{
  "refresh_token": "<hex>",
  "device": "web-chrome"
}
```

`device` opcional, max 120. Se omitido, reusa o `device` gravado.

**200** `Tokens` (access **e** refresh **novos**). Descarte o refresh antigo.

**401** `refresh token inválido` (inexistente ou **reuso** — neste caso todos os refresh do user caem) / `refresh token expirado`.

### `POST /api/v1/auth/logout`

**Sem** Bearer.

```json
{ "refresh_token": "<hex>" }
```

**204** vazio. Token desconhecido → 204 mesmo (idempotente). Só revoga **esse** refresh, não a família.

### `POST /api/v1/auth/recovery/start`

```json
{
  "cpf": "529.982.247-25",
  "phone": "+5554999990001"
}
```

**204 sempre** se o JSON é válido — inclusive CPF/phone errados (não vaza conta). Se bater user ativo + phone exato, envia OTP `recovery`.

Para o mock: o app **já precisa do `user_id`** (do cadastro). `GET /admin/otp/{user_id}` depois disto.

### `POST /api/v1/auth/recovery/confirm`

```json
{
  "cpf": "529.982.247-25",
  "phone": "+5554999990001",
  "code": "482193",
  "new_password": "nova-senha-segura"
}
```

**204**. Revoga **todos** os refresh. **Não** emite sessão — o user faz login + MFA.

**401** `código inválido` (também se CPF/phone não batem).

### `GET /api/v1/me`

Bearer qualquer role.

**200**

```json
{
  "id": "0193a0c2-7e1b-7c11-8f00-aaaaaaaaaaaa",
  "email": "produtor@agrobench.local",
  "phone": "+5554999990001",
  "role": "producer",
  "mfa_enabled": true,
  "status": "active"
}
```

Sem CPF, sem wallet.

### `GET /api/v1/admin/otp/{user_id}`

Ver §4.1.

---

## 8. Wallet (só `producer`)

Outra role → `403` `perfil insuficiente`. Sem Bearer → `401`.

### `POST /api/v1/wallet`

**Body**

```json
{
  "pubkey": "ProducerWallet1111111111111111111111111111",
  "encrypted_blob": "dGVzdC1ibG9iLWNpZnJhZG8=",
  "blob_version": 1
}
```

| Campo | Required | Regras |
|---|---|---|
| `pubkey` | sim | string 32–64, **base58 Solana** (ed25519). Unique global. Pubkeys fake do seed-demo não servem na Devnet. |
| `encrypted_blob` | sim | base64 std (decode falha → `400` `encrypted_blob deve ser base64`) |
| `blob_version` | não | se `<1` ou omitido → **1** |

App gera keypair **no device**. Backend nunca vê a privada. 1 wallet por user (`users.id` unique).

**201** (mesmo shape do GET):

```json
{
  "id": "0193a0c3-....",
  "pubkey": "ProducerWallet1111111111111111111111111111",
  "blob_version": 1,
  "balance_usdc": 0,
  "reward_usdc": 0
}
```

`exported_at` omitido se nunca exportou. `balance_usdc` = `ChainClient.Balance` na Devnet (0 se a ATA USDC ainda não existe). `reward_usdc` = soma de `attestations.reward_usdc`.

**409** `wallet já cadastrada` (user ou pubkey duplicados).

### `GET /api/v1/wallet`

**200** mesmo JSON. **404** `wallet não encontrada`.

### `GET /api/v1/wallet/export`

**Não é POST. Não é body JSON.** Query `code`.

1. `GET /api/v1/wallet/export` **sem** `code` → envia OTP purpose `login` → **403**

```json
{
  "code": "FORBIDDEN",
  "message": "OTP enviado",
  "otp_required": true
}
```

2. OTP via §4. `GET /api/v1/wallet/export?code=482193` → **200**

```json
{
  "pubkey": "ProducerWallet1111111111111111111111111111",
  "encrypted_blob": "dGVzdC1ibG9iLWNpZnJhZG8=",
  "blob_version": 1
}
```

Marca `exported_at`. OTP inválido → `401` como MFA. Existe `types/input.ExportWallet` com JSON `code` — **o handler ignora o body**.

Atenção: OTP de export usa purpose `login` e **invalida** OTP de login pendente.

### `GET /api/v1/wallet/payouts`

Bearer producer. Sem wallet → 404.

**200** array:

```json
[
  {
    "id": "...",
    "cycle_id": "...",
    "amount_usdc": 12.5,
    "tx": "mock_..."
  }
]
```

Pode ser `null` se vazio.

---

## 9. CAR, regiões, culturas

### `GET /api/v1/micro-regions`

Público. **200** array:

```json
[
  { "id": "...", "ibge_code": "43010", "name": "Passo Fundo", "uf": "RS" }
]
```

35 itens após seed, `ORDER BY uf, name`. Use `id` (UUID) em `micro_region_id` / query `region`. `ibge_code` é o código IBGE da **microrregião** (5 dígitos). Passo Fundo município CAR usa `4314100` — **não** confundir com `43010`.

### `GET /api/v1/cultures`

Público. **Não está no README §7.** **200**:

```json
[
  { "id": "...", "code": "soybean", "name": "Soja" },
  { "id": "...", "code": "corn", "name": "Milho" },
  { "id": "...", "code": "wheat", "name": "Trigo" }
]
```

`id` → `culture_id` de ciclos. `code` → `culture_code` do payload.

### `POST /api/v1/property`

Bearer producer.

```json
{
  "car": "RS-4314100-AAA00000000000000015",
  "micro_region_id": "<uuid de GET /micro-regions, Passo Fundo>"
}
```

SICAR mock síncrono. CAR unique por HMAC (global). User **pode** ter várias properties (não há unique em `user_id`).

**201**

```json
{
  "id": "...",
  "micro_region_id": "...",
  "car_status": "approved",
  "verified_at": "2026-09-08T19:00:00Z"
}
```

`car_status: "rejected"` e sem `verified_at` se CAR inexistente/inativo. HTTP ainda é **201** — o app deve ler `car_status`, não só o status HTTP.

CAR não volta no JSON.

**400** região inexistente (`NOT_FOUND` se UUID de região não existe). **409** `CAR já vinculado`. **502** `falha ao consultar SICAR`.

Side effect: `ChainClient.RecordCARVerification` se o user já tem wallet (evento sem o CAR). Sem wallet, property é criada mesmo assim.

### `GET /api/v1/property`

**200** a property **mais recente** (`ORDER BY created_at DESC LIMIT 1`). **404** `propriedade não encontrada`. Não é lista.

Guarde o `id` do POST para o commit (`property_id`).

---

## 10. Cycle

### `GET /api/v1/cycles`

Público.

Query (todos opcionais; valores vazios = sem filtro):

| Query | Significado | Valor |
|---|---|---|
| `culture` | UUID `culture_id` | **não** é o code `soybean` |
| `region` | UUID `micro_region_id` | **não** é ibge_code |
| `status` | `open` \| `closed` \| `aggregated` | literal |

**200**

```json
[
  {
    "id": "...",
    "culture_id": "...",
    "micro_region_id": "...",
    "label": "2025/26",
    "opens_at": "2025-09-01T00:00:00Z",
    "closes_at": "2026-03-01T00:00:00Z",
    "status": "open"
  }
]
```

`ORDER BY opens_at DESC`.

### `POST /api/v1/admin/cycles`

Bearer admin.

```json
{
  "culture_id": "<uuid soybean>",
  "micro_region_id": "<uuid Passo Fundo>",
  "label": "2026/27",
  "opens_at": "2026-09-01T00:00:00Z",
  "closes_at": "2027-03-01T00:00:00Z"
}
```

`label` 4–20 chars. `closes_at` deve ser **depois** de `opens_at`. Status inicial `open`.

**201** mesmo shape do item da lista.

**400** `closes_at deve ser depois de opens_at`. **409** `ciclo já existe para cultura × região × safra` (unique culture+region+label).

Para commit na demo **hoje**, use `opens_at` no passado recente e `closes_at` no futuro. Unique impede relabel igual ao seed (`2025/26` Passo Fundo soja já existe se rodou seed-demo).

### `POST /api/v1/admin/cycles/{id}/close`

Bearer admin. Sem body.

**200** cycle com `status: "closed"`. Side effect: job `aggregate_cycle` (unique por `cycle_id`). Worker → `aggregates` + `contribution_weights` + `status=aggregated`.

**400** id inválido. **409** já fechado / não open.

O app **não** espera o aggregate no mesmo request. Poll `GET /cycles?status=aggregated` ou o `id`.

---

## 11. Contribution + enclave

### `GET /api/v1/enclave/public-key`

Público. Chamar **antes** de cifrar, de preferência perto do reveal (chave mock pode ser gerada no boot se `.env` vazio — payloads antigos param de decifrar).

**200**

```json
{
  "box_public_key": "<64 hex x25519>",
  "signing_public_key": "<64 hex ed25519>",
  "provider": "mock"
}
```

`attestation` omitido no mock (`[]byte` nil + `omitempty`). No nitro viria COSE em JSON como **base64** (`[]byte`).

`provider`: `mock` \| `nitro`. **502** se Identity falhar.

### Stake on-chain (co-sign do produtor)

Com `CHAIN_SOLANA_PROGRAM_ID` o lock **move USDC** (ATA do produtor → vault PDA). O backend **não tem** a chave do produtor. Fluxo na **attempt 1**:

1. `POST /chain/stake/lock-tx` → tx parcial (fee payer = treasury, já assinada pela treasury)
2. App assina com a keypair do produtor (`partialSign` / `signTransaction`)
3. `POST /chain/stake/lock-submit` → backend co-assina se faltar e envia
4. `POST /contributions/commit` com `stake_tx` = a `signature` do passo 3 (ou `signed_tx` = a tx base64 já assinada, pulando o submit)

Sem program id (fallback): `commit` ainda chama `LockStake` Memo só com a treasury. O two-step também funciona (Memo com producer signer).

#### `POST /api/v1/chain/stake/lock-tx`

Bearer producer. Amount em **micro-USDC** (10 USDC = `10000000`). `0` ou omitido → config `stake.amount_usdc`.

```json
{ "amount": 10000000 }
```

**200** `{ "tx": "<base64 da tx parcial>" }`

A tx já traz a assinatura da treasury. O app **não** troca o fee payer. Assina o produtor e devolve o byteserializado em base64 std.

#### `POST /api/v1/chain/stake/lock-submit`

```json
{ "tx": "<base64 com sig do produtor>" }
```

**200** `{ "signature": "<base58 Solana>" }`

**400** se faltar a assinatura do produtor. **502** se a RPC rejeitar (ATA sem USDC, programa não inicializado, blockhash expirado ~60s).

Guarde `signature` e mande no commit como `stake_tx`.

#### `POST /api/v1/contributions/{id}/release-stake/tx` e `.../submit`

Mesmo formato `{ "tx": "..." }` / `{ "signature": "..." }`. `{id}` = contribution com stake `locked` do caller.

O worker de validação **não** libera on-chain quando o programa exige o produtor (log + stake continua `locked`). O app deve chamar o release depois de 2 ciclos aceitos.

`POST /admin/chain/initialize` (admin) chama `initialize` uma vez. Equivalente CLI: `agrobench chain-init`.

### `POST /api/v1/contributions/commit`

Bearer producer.

```json
{
  "cycle_id": "0193b000-0000-7000-8000-000000000001",
  "property_id": "0193a0c4-....",
  "level": "basic",
  "hash": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
  "stake_tx": "<signature do lock-submit, attempt 1>"
}
```

`stake_tx` e `signed_tx` são opcionais. Com programa Anchor na attempt 1, **um dos dois é obrigatório** (senão 400 `assine o lock de stake antes do commit`). `signed_tx` = mesma base64 do lock-submit; o backend envia na hora.

README §7 omite `property_id`. **O código exige** UUID `required`.

`hash`: `len=64` (não valida hex). `level`: `oneof=basic intermediate advanced`.

Regras: ciclo `IsOpen`; property do user e `car_status=approved`; wallet existe; sem contribuição ativa no ciclo; attempt 1 ou 2; lock on-chain (two-step ou `LockStake` no fallback); `ChainClient.Commit` (Memo).

**201**

```json
{
  "id": "...",
  "cycle_id": "...",
  "wallet_id": "...",
  "level": "basic",
  "attempt": 1,
  "commit_hash": "aaaa...aaaa",
  "commit_tx": "<signature Solana, base58>",
  "status": "committed"
}
```

`reject_reason` omitido. Sem ciphertext.

**409** ciclo fechado / já tem ativa. **403** property de outro user / `CAR não aprovado`. **404** ciclo/property/wallet. **502** commit on-chain / stake.

**Não** envie o plaintext. O hash errado só aparece no worker (rejected), não neste POST.

### `POST /api/v1/contributions/{id}/reveal`

```json
{ "ciphertext": "<base64 do sealed box>" }
```

**O use case recebe `userID` e não verifica dono.** Qualquer producer autenticado que acerte o UUID de uma contribuição `committed` pode revelar. O app só deve revelar as próprias.

Decode base64 falha → `400` `ciphertext deve ser base64`. Status ≠ `committed` → `409` `contribuição não está em committed`.

**200** mesmo struct, `status: "revealed"`. Side effect: job `validate_contribution` (unique por `contribution_id`). Worker: `validating` → enclave → `accepted` (attest + TransferUSDC reward + talvez ReleaseStake) ou `rejected` (`reject_reason` típico `validação rejeitada`).

Hash/schema/faixa errados **não** dão 4xx no reveal. Poll `GET` até `accepted`|`rejected`. Em `serve` o worker é o mesmo processo; latência ~ms–s.

### `GET /api/v1/contributions`

Lista da **wallet do caller**. Sem wallet → 404. **200** array do struct acima (com `reject_reason` se houver). `ORDER BY created_at DESC`.

### `GET /api/v1/contributions/{id}`

**Não filtra por wallet.** Qualquer producer com UUID válido lê o metadata. Ciphertext não vai no JSON. **404** se não existe.

Poll: `GET /contributions/{id}` até sair de `revealed`/`validating`.

---

## 12. Benchmark

### `GET /api/v1/benchmark/me?cycle=<uuid>`

Bearer producer. Query `cycle` **obrigatória** (UUID). Sem ela ou inválida → `400` `cycle inválido`.

Trava: `ConsecutiveAccepted(wallet, culture do ciclo, região do ciclo) >= benchmark.free_after_cycles` (default **3**). Conta contribuições `accepted`/`rejected` daquela cultura×região, `ORDER BY cycles.opens_at DESC`, incrementa enquanto for `accepted`, para no primeiro não-accepted.

**200** (nível **sempre** `"basic"`, só métricas basic no SQL):

```json
{
  "cycle_id": "...",
  "level": "basic",
  "metrics": [
    { "metric": "area_ha", "mean": 50, "median": 50, "p25": 40, "p75": 60, "n": 14 },
    { "metric": "total_cost_ha", "mean": 4200, "median": 4200, "p25": 3800, "p75": 4600, "n": 14 }
  ]
}
```

Se passou a trava mas o ciclo ainda não tem `aggregates`, `metrics` pode ser `null`/ausente.

**403** quando `n < need`:

```json
{
  "code": "FORBIDDEN",
  "message": "Painel bloqueado",
  "detail": "painel bloqueado: 1/3 ciclos consecutivos",
  "cycles_validated": 0,
  "cycles_required": 3
}
```

**`cycles_validated` é hardcoded `0` no handler**, mesmo quando o use case calculou 1 ou 2. O número real está em `detail` (`N/M`). Não use `cycles_validated` para UX de progresso. `cycles_required` vem da config (3).

Seed-demo: o produtor único já tem 3 ciclos aceitos → painel **200** para esses `cycle_id` aggregated. Produtor novo: 403 até 3 aceitos consecutivos na mesma cultura×região.

### `GET /api/v1/benchmark/report?cycle=<uuid>`

Bearer `institution` apenas. README cita `region` e `culture`: **o handler ignora**; só `cycle`.

**Não** checa instituição `approved`, assinatura `active`, plano regional nem escopo de regiões. `RequireSubscription` **não está ligado**. Qualquer JWT institution lê todos os níveis daquele ciclo.

**200**

```json
{
  "cycle_id": "...",
  "metrics": [
    { "metric": "basic:area_ha", "mean": 50, "median": 50, "p25": 40, "p75": 60, "n": 14 }
  ]
}
```

`metric` no relatório = `level + ":" + metric` (ex. `basic:total_cost_ha`, `intermediate:yield_sacks_ha`).

Side effect pretendido: `INSERT report_access` (base do split). O insert usa `auth.UserID` como `institution_id`, mas `institutions.id` ≠ `users.id`. O erro é **engolido**. O split mensal pode **não** ver esses acessos. Não dependa disso para a demo de pool sem conferir o banco.

---

## 13. Institution e pagamento

### `POST /api/v1/institutions/register`

Público. Cria `users.role=institution`, `mfa_enabled=false`, phone dummy `+5500000000000`, `cpf_hmac` = HMAC(`CNPJ:`+dígitos) para não colidir com CPF. Institution `status=pending`. **Não** envia OTP. **Não** devolve tokens — em seguida `POST /auth/login`.

```json
{
  "email": "coop@agrobench.local",
  "password": "senha-segura-1",
  "cnpj": "12.345.678/0001-95",
  "name": "Cooperativa Demo"
}
```

CNPJ do Bruno `12.345.678/0001-99` é o **mesmo HMAC** da instituição seed (`12345678000199`) → **409** se seed-demo já rodou. Use outro CNPJ de 14 dígitos.

**201**

```json
{
  "id": "<institution uuid — use no approve>",
  "name": "Cooperativa Demo",
  "status": "pending"
}
```

`user_id` tem `json:"-"` — **não vem**. Login usa o email.

**400** `CNPJ inválido`. **409** `email ou CNPJ já cadastrado`.

### `GET /api/v1/institutions/me`

Bearer institution. **200** mesmo shape (`id`, `name`, `status`). **404** `instituição não encontrada`.

### `POST /api/v1/admin/institutions/{id}/approve` e `.../reject`

Bearer admin. `{id}` = **institution id**, não user id. Sem body. Decide pelo sufixo do path (`reject` vs resto).

**204**. **404** `instituição não encontrada`.

### `POST /api/v1/institutions/subscribe`

Bearer institution. Institution tem de estar `approved`.

```json
{
  "plan": "regional",
  "regions": ["<micro_region uuid>"]
}
```

| Campo | Regras |
|---|---|
| `plan` | `regional` \| `national` |
| `regions` | regional: **1 a 5** UUIDs (`plans.regional.max_regions`). national: ignorado na validação (pode omitir/vazio). |

Preços config: regional **500** USDC, national **2000**.

Insert subscription `pending`, payment `provider=mock`, `provider_ref` = `mock_cs_<subscription_id>`.

**201**

```json
{
  "subscription_id": "...",
  "checkout_url": "http://localhost:8080/mock/checkout/mock_cs_<subscription_id>",
  "provider_ref": "mock_cs_<subscription_id>"
}
```

**Não devolve `payment_id`.** A URL de checkout **não é uma rota da API** (`/mock/checkout/...` → 404 `Rota não encontrada`). No mock **não abra** essa URL esperando UI. Confirme via webhook (§ abaixo) ou admin confirm se você tiver o UUID do payment (não exposto).

**403** `instituição ainda não aprovada`. **400** `plano regional exige 1 a 5 microrregiões`.

### Confirmar pagamento no mock (demo)

**Caminho que o frontend consegue sem UUID de payment:**

`POST /api/v1/webhooks/payment`  
Auth: público. Body = JSON do mock (não é payload Stripe). Header `Stripe-Signature` ignorado.

```json
{
  "ref": "mock_cs_<subscription_id>",
  "subscription_id": "<uuid>",
  "amount_usdc": 500,
  "status": "paid"
}
```

`status` tem de ser exatamente `"paid"` senão o evento é ignorado (204 sem ativar). Lookup payment por `provider_ref`. Depois transfere USDC treasury→pool e `subscriptions.status=active`.

**204**.

**Caminho admin:** `POST /api/v1/admin/payments/{id}/confirm` — `{id}` é `payments.id`, **não** `subscription_id`. Sem GET de payments no código: só útil se o agente ler o banco. Monta o mesmo webhook internamente (`paymentmock.BuildWebhook`).

Treasury mock precisa de saldo (seed base minta 1_000_000 USDC na treasury). Sem seed, confirm pode 502 na transfer.

Stripe real: `ParseWebhook` usa assinatura; o body mock acima **não** serve.

---

## 14. Pool e admin

### `GET /api/v1/pool/periods`

Público. **200** array:

```json
[
  {
    "id": "...",
    "month": "2025-08",
    "gross": 5000,
    "infra_cost": 280,
    "maintainer_fee": 750,
    "net": 3970,
    "status": "distributed"
  }
]
```

`month` é `CHAR(7)` `YYYY-MM`. Seed-demo insere `2025-08` distributed.

### `GET /api/v1/pool/periods/{month}`

Público. Path = `2025-08`. **404** `período não encontrado`. **Não** lista `payouts` aninhados (README sugere payouts no GET; o JSON é só o período). Payouts do produtor: `GET /wallet/payouts`.

### `POST /api/v1/admin/pool/distribute?month=2026-08`

Bearer admin. Query `month` (`YYYY-MM`). Vazio → o worker usa **mês calendário anterior**.

**202** sem body. Enfileira `distribute_pool` (unique por args/`month`). **Não há cron River** no `queue.Start` — README §8/§9 falam em cron dia 1; **o código não registra periodic job**. A demo **tem** de chamar este POST (ou jobs/run).

### `POST /api/v1/admin/seed/demo`

Bearer admin. Sem body. **200** `{ "status": "ok" }`. Idempotente o bastante (ON CONFLICT / skip email oficial existente; recria `seed.farmer.*`). Cria o produtor oficial, agricultores extras em várias microrregiões do RS, ciclos aggregated + `2026/27` aberto, Cotrijal aprovada, pool `2025-08` / `2026-08` / `2026-09`.

### `POST /api/v1/admin/jobs/run/{kind}`

Bearer admin. Único `kind` implementado: `distribute_pool`. Query `month` igual ao distribute. Outro kind → 400 `job desconhecido`. README diz “qualquer job”; **validate/aggregate não têm disparo HTTP** (só reveal/close).

**202** vazio.

---

## 15. Sequências ponta a ponta

Substitua UUIDs. OTP = §4. Access em `Authorization: Bearer`. Refresh **não** vai no header.

### 15.1 Cadastro producer → OTP mock → MFA → `/me`

```text
POST /api/v1/auth/register
  {email, phone, cpf, password}
  → 201 { mfa_required, mfa_token, user_id }

GET  /api/v1/admin/otp/{user_id}
  → 200 { code }

POST /api/v1/auth/mfa/verify
  { mfa_token, code }
  → 200 { access_token, refresh_token, expires_in: 900, token_type: "Bearer" }

GET  /api/v1/me
  Authorization: Bearer <access>
  → 200 { id, email, phone, role: "producer", mfa_enabled: true, status: "active" }
```

Persistir: `access_token`, `refresh_token`, `user_id`. Renovar a cada ~15 min com refresh.

### 15.2 Login → MFA → refresh → logout

```text
POST /api/v1/auth/login { email, password }
  producer → 200 MFAChallenge (novo OTP)
  institution/admin → 200 Tokens (PARE; não chame MFA)

GET  /api/v1/admin/otp/{user_id}     # só producer
POST /api/v1/auth/mfa/verify { mfa_token, code } → Tokens

POST /api/v1/auth/refresh { refresh_token } → Tokens novos; apague o refresh antigo

POST /api/v1/auth/logout { refresh_token } → 204
```

401 access expirado → refresh. 401 refresh inválido por reuso → login completo (família revogada).

### 15.3 Recovery

```text
POST /api/v1/auth/recovery/start { cpf, phone } → 204
GET  /api/v1/admin/otp/{user_id}                 # precisa do user_id já conhecido
POST /api/v1/auth/recovery/confirm { cpf, phone, code, new_password } → 204
POST /api/v1/auth/login … MFA com a senha nova
```

### 15.4 Onboarding producer: wallet → CAR → ciclo aberto → commit → reveal → status

Pré: autenticado producer. Catálogos públicos.

```text
GET  /api/v1/micro-regions
GET  /api/v1/cultures
GET  /api/v1/cycles?culture=<culture_uuid>&region=<region_uuid>&status=open
     # se vazio: login admin e POST /admin/cycles (janela contendo now)

# App gera keypair. Nunca POST da privada.
POST /api/v1/wallet
  { pubkey, encrypted_blob: base64, blob_version: 1 }
  → 201 { id, pubkey, balance_usdc, reward_usdc }

POST /api/v1/property
  { car: "RS-4314100-AAA00000000000000015", micro_region_id }
  → 201 { id, car_status }
  # só continue se car_status === "approved"

# Montar envelope, canonicalizar, hash = sha256(bytes), guardar bytes.
GET  /api/v1/enclave/public-key → box_public_key
# Seal(bytes, box_pub) → ciphertext b64  (pode ser depois do commit; hash já fechado)

# Attempt 1 com programa: lock on-chain ANTES do commit.
POST /api/v1/chain/stake/lock-tx { "amount": 10000000 } → { tx }
# App: deserialize, partialSign com a keypair do produtor, serialize base64.
POST /api/v1/chain/stake/lock-submit { tx } → { signature }

POST /api/v1/contributions/commit
  { cycle_id, property_id, level: "basic", hash, stake_tx }
  → 201 { id, status: "committed", attempt, commit_tx }
  # stake_tx = signature do lock-submit. Sem program id, commit ainda faz LockStake Memo sozinho.

POST /api/v1/contributions/{id}/reveal
  { ciphertext }
  → 200 { status: "revealed" }

loop:
  GET /api/v1/contributions/{id}
  until status in { accepted, rejected }
```

`rejected` + attempt 1 → pode commit de novo (attempt 2, sem novo stake). `accepted` → `GET /wallet` para ver `reward_usdc` / `balance_usdc`.

Com `CHAIN_SOLANA_PROGRAM_ID`, lock/release/credit/distribute passam pelo programa `agrobench` (USDC na vault PDA). `commit_tx` / atestação continuam Memo. Explorer: `https://explorer.solana.com/tx/<sig>?cluster=devnet`. Sem program id, lock no commit ainda é Memo.

### 15.5 Painel do produtor

```text
GET /api/v1/benchmark/me?cycle=<uuid de um ciclo aggregated da mesma cultura×região>
```

- 3 aceitos consecutivos (seed-demo) → 200, só basic.
- Senão → 403; ler `detail` e `cycles_required`; **ignorar** `cycles_validated` (sempre 0).
- Ciclo ainda `open`/`closed` sem aggregates → 200 com metrics vazias se a trava passou.

### 15.6 Instituição: cadastro → espera → checkout → confirma mock → relatório

```text
POST /api/v1/institutions/register { email, password, cnpj, name }
  → 201 { id: institution_id, status: "pending" }

POST /api/v1/auth/login { email, password }
  → 200 Tokens  (sem MFA)

GET  /api/v1/institutions/me → pending
# UI: "aguardando aprovação"

# Admin:
POST /api/v1/auth/login { ADMIN_EMAIL, ADMIN_PASSWORD } → Tokens admin
POST /api/v1/admin/institutions/{institution_id}/approve → 204

# De volta institution:
GET  /api/v1/institutions/me → approved
POST /api/v1/institutions/subscribe
  { plan: "regional", regions: [micro_region_id] }
  → 201 { subscription_id, checkout_url, provider_ref }

POST /api/v1/webhooks/payment
  { ref: provider_ref, subscription_id, amount_usdc: 500, status: "paid" }
  → 204

GET  /api/v1/benchmark/report?cycle=<uuid aggregated>
  → 200 { cycle_id, metrics: [ { metric: "basic:area_ha", ... } ] }
```

Não navegue `checkout_url` no mock.

### 15.7 Admin: ciclos, instituição, seed, pool

```text
POST /api/v1/auth/login { admin email/senha } → Tokens

POST /api/v1/admin/seed/demo → { status: "ok" }

POST /api/v1/admin/cycles { culture_id, micro_region_id, label, opens_at, closes_at }
POST /api/v1/admin/cycles/{id}/close → job aggregate

POST /api/v1/admin/institutions/{id}/approve | reject

POST /api/v1/admin/pool/distribute?month=2026-08 → 202
# equivalente: POST /api/v1/admin/jobs/run/distribute_pool?month=2026-08

GET  /api/v1/pool/periods
GET  /api/v1/pool/periods/2026-08
```

Confirmar payment por UUID só se o id for conhecido; senão webhook §13.

---

## 16. Economia (defaults `config.dev.json`) — só leitura

| Chave | Default | Efeito no app |
|---|---|---|
| `rewards.base_usdc` | 5 | reward se accepted |
| `rewards.level_multipliers` | 1 / 2 / 3.5 | × nível |
| `stake.amount_usdc` | 10 | 10 USDC (`10_000_000` micro). Com `PROGRAM_ID`, `lock_stake` debita a ATA do produtor; sem id, Memo no commit |
| `stake.release_after_cycles` | 2 | Com programa, o app co-assina release. Sem programa, Memo automático no worker |
| `benchmark.free_after_cycles` | 3 | trava do painel |
| `pool.maintainer_fee_pct` | 15 | split |
| `pool.infra_cost_per_contribution_usdc` | 2 | split |
| `plans.regional.price_usdc` | 500 | subscribe |
| `plans.regional.max_regions` | 5 | subscribe |
| `plans.national.price_usdc` | 2000 | subscribe |
| `auth.access_ttl_minutes` | 15 | `expires_in` |
| `auth.otp_ttl_minutes` | 5 | MFA token + OTP |
| `auth.otp_max_attempts` | 5 | |

O frontend não configura isso.

---

## 17. Armadilhas para o agente

1. **OTP mock some no restart** (outbox memória). `GET /admin/otp/{user_id}` é público e só existe com SMS mock. Recovery não devolve `user_id`.
2. **`cycles_validated` no 403 do painel é sempre 0.** Use `detail`. A trava dos 3 ciclos **existe** no use case.
3. **Seed-demo:** 1 produtor oficial (`produtor@agrobench.local`) com pubkey Devnet `FXsin7UZTGrix1cEe1QpMDFz3a8cDHzVK7h2oisjpzf3` + extras `seed.farmer.NN` off-chain. Blob dummy — **não assina** lock no browser; use conta criada neste aparelho (`ensureWallet`). Com `PROGRAM_ID`, lock SPL na vault PDA. Recompensa ainda sai da treasury.
4. **Pool: não há cron.** Só `POST /admin/pool/distribute?month=YYYY-MM` (ou `jobs/run/distribute_pool`). README mente no cron.
5. **MFA obrigatório só para producer.** Institution/admin: login = tokens. Não chame `mfa/verify` com access token. MFA JWT no `Authorization` → 401.
6. **Commit exige `property_id`** (README omite). **`GET /cultures` e `GET /micro-regions` existem** (README omite). Query de ciclos `culture`/`region` são **UUIDs**.
7. **Export wallet = GET `?code=`**, 403 com `otp_required` se faltar code. Types/input JSON `code` é morto.
8. **Subscribe não devolve `payment_id`.** Checkout URL mock 404. Confirme com `POST /webhooks/payment` JSON mock. Relatório institution **não** valida assinatura/plano; `report_access` provavelmente não grava (FK user vs institution).
9. **`go-paginate` não é usado.** Listas completas; vazio pode ser `null`.
10. **Campos JSON extra → 400.** `DisallowUnknownFields`.
11. **Reveal/GET-by-id não autorizam por dono.** Não exponha IDs de outros.
12. **Hash = bytes exatos**, não “JSON parecido”. Canonicalize ou cache o buffer. `culture_code` lowercase do catálogo. Reveal 200 ≠ validação ok.
13. **CAR `…0001`** ocupado pelo seed-demo. Cadastro novo use 2–17. Instituição Bruno CNPJ `...0001-99` conflita com seed.
14. **1 contribuição ativa / ciclo**; retry se `rejected`. Stake só na attempt 1. Poll após reveal (`revealed` → `validating` → terminal).
15. **Jobs HTTP:** só `distribute_pool`. Close/reveal que enfileiram aggregate/validate. `GET /admin/otp` **não** é role admin.
16. **Access 15 min, refresh rotaciona.** Reuso de refresh derruba a sessão em todos os devices.
17. Health **sem** `/api/v1`. Rate limit 120/min/IP → 429 fora do catálogo.

---

## 18. Mapa README §7 × código

| README | Código |
|---|---|
| identity, wallet, cycles, commit/reveal, me, institutions, pool, seed, otp, jobs | Existem; detalhes acima |
| `GET /wallet/export` exige OTP | GET + query `code`; 403 `otp_required` |
| commit: cycle, level, hash | + **`property_id`** |
| `GET /benchmark/report?cycle=&region=&culture=` | só `cycle`; sem checagem de plano |
| `GET /pool/periods/{month}` inclui payouts | só campos do período |
| `POST /admin/jobs/run/{kind}` qualquer job | só `distribute_pool` |
| Cron split dia 1 | **ausente** |
| `RequireSubscription` | **ausente** nas rotas |
| `GET /micro-regions`, `GET /cultures` | **existem**, omitidos no README |
| Paginação go-paginate | **não usada** |

Bruno não cobre: export wallet, GET contribution by id, cultures, institution me, approve/reject, webhook, report, payouts, pool by month, jobs/run, distribute, health. Os exemplos JSON deste doc e os `.bru` existentes são os payloads válidos.
)
