# AgroBench — Plano de implementação do backend

Roadmap de execução. **Arquitetura, decisões, contratos, modelo de dados, endpoints e fluxos
estão no [README.md](README.md)** — este documento não repete nada disso, só aponta.

Decisões consolidadas em 2026-09-08.

---

## Fases

| Fase | Entrega | Referência no README | Verificação |
|---|---|---|---|
| 0 ✅ | Esqueleto: go.mod, cmd, config, apperrors, httpx, database, rest.go, Docker, compose, Air, Makefile, migration inicial, `/healthz` | [§1](README.md#1-ambiente-de-desenvolvimento), [§4](README.md#4-estrutura-de-pastas), [§11](README.md#11-configuração) | `make dev` sobe e responde |
| 1 ✅ | `pkg/port` + todos os adapters **mock** + **reais** semi-prontos + `cmd/enclave` + `internal/core/{payload,validate,crypto}` + registry | [§5](README.md#5-ports-externas-e-convenção-mock--real) | `go build ./...`, unit (payload, enclave mock) e integração (chain mock via testcontainers) |
| 2 | identity + auth core (argon2, hmac, OTP, JWT, refresh, middleware) | [§10](README.md#10-autenticação), [§7](README.md#7-endpoints) | Bruno: register → login → mfa → me |
| 3 | wallet + car + micro_regions/cultures + seeds | [§8](README.md#8-fluxos-principais) "Onboarding", [§13](README.md#13-seeds) | Bruno: onboarding completo |
| 4 | cycle + contribution (commit/reveal/stake) + River + worker de validação + atestação | [§8](README.md#8-fluxos-principais) "Commit-reveal", [§9](README.md#9-jobs) | Fluxo ponta a ponta com os mocks |
| 5 | aggregation + benchmark (regra dos 3 ciclos e do nível básico) | [§8](README.md#8-fluxos-principais) "Fechamento de ciclo" | Painel do produtor bloqueado/liberado |
| 6 | institution + payment + pool + split mensal | [§8](README.md#8-fluxos-principais) "Pool mensal" | Relatório pago + distribuição |
| 7 | admin/seed-demo, testes de integração, Bruno completo | [§12](README.md#12-testes), [§13](README.md#13-seeds) | `make test-integration` verde, roteiro da demo do deck funciona |
| 8 | Preencher os adapters reais (Solana devnet, Stripe test mode, Nitro host + `cmd/enclave`) | [§5](README.md#5-ports-externas-e-convenção-mock--real), [§14](README.md#14-limitações-assumidas) | Opcional pro pitch |

---

## Ordem dentro de cada fase de módulo (2 a 6)

Cada módulo em `internal/<modulo>/` segue sempre a mesma sequência, conforme o padrão descrito
no [README §4](README.md#4-estrutura-de-pastas):

1. `domain/` e `types/{input,output}` — o vocabulário do módulo
2. `contract/` — interfaces de repository e usecase
3. `repository/` — SQL, um arquivo por operação
4. `usecase/` — regra de negócio, um arquivo por operação
5. `handler/` (ou `worker/`, se for só job) — HTTP
6. `di/` + `rest/router.go` — wiring manual e registro no `pkg/adapter/rest/rest.go`
7. Migration correspondente + coleção Bruno em `docs/bruno/`

---

## Pendências fora do plano

Todas as decisões de arquitetura e regras de negócio estão fechadas (README §2). Ficam apenas:

- **Endpoints públicos do SICAR e CONAB**: não pesquisados — ver
  [README §14, limitação 4](README.md#14-limitações-assumidas)
- **Git**: `AgroBench/` ainda não é repositório. Fica a cargo do Felipe decidir quando
  inicializar e o que fazer com o `.git` interno de `pitch-deck/`
- **Fase 8**: os adapters reais ficam semi-prontos e sem teste em ambiente real dentro do escopo
  do pitch — ver [README §14, limitação 3](README.md#14-limitações-assumidas)
