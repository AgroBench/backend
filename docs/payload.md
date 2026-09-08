# Payload do produtor — formato e regra do hash

Referência para o app. Fonte de verdade: `internal/core/payload`.

## Envelope

```json
{
  "version": 1,
  "level": "basic | intermediate | advanced",
  "cycle_id": "uuid do ciclo aberto",
  "nonce": "hex aleatório, mínimo 32 caracteres (16 bytes)",
  "data": { ...conforme o nível... }
}
```

O `nonce` é obrigatório: sem ele, um payload de nível básico tem pouca entropia e um terceiro poderia adivinhar o hash publicado no commit por força bruta.

## `data` por nível

**basic**
```json
{ "culture_code": "soybean", "area_ha": 50, "total_cost_brl": 250000 }
```

**intermediate** = basic +
```json
{
  "cost_by_input": { "fertilizer_brl": 60000, "pesticide_brl": 41000, "seed_brl": 25000, "fuel_brl": 15000, "labor_brl": 20000 },
  "yield_sacks_ha": 72,
  "planting_date": "2025-10-15",
  "harvest_date": "2026-03-01",
  "suppliers": { "fertilizer": "...", "pesticide": "...", "seed": "...", "fuel": "..." }
}
```

**advanced** = intermediate +
```json
{
  "soil_type": "latossolo",
  "rotation_history": ["corn", "wheat"],
  "irrigation": { "used": true, "system": "center_pivot | drip | sprinkler | furrow | other" },
  "pest_events": [{ "name": "ferrugem", "management": "fungicida x" }],
  "climate_losses": [{ "event": "drought | hail | frost | flood | heat_wave | other", "area_pct": 12.5 }],
  "mechanization": { "type": "own | outsourced | mixed", "machinery": ["colheitadeira"] }
}
```

Campos desconhecidos são rejeitados. Validações completas nas tags `validate` de `internal/core/payload/levels.go`.

## Commit e reveal

1. O app serializa o envelope em bytes (`plaintext`).
2. `commit_hash = sha256(plaintext)` em hex. **O hash é dos bytes exatos**, sem canonicalização no servidor. O app deve guardar os mesmos bytes até o reveal. Se precisar re-serializar, use ordenação de chaves e sem espaços (equivalente a `payload.Canonicalize`).
3. `POST /contributions/commit` com `cycle_id`, `level`, `hash`.
4. O app busca a chave do ambiente de validação em `GET /enclave/public-key` (`box_public_key`, x25519 em hex) e cifra com **sealed box** (`crypto_box_seal` do libsodium / `tweetnacl-sealedbox-js`): saída = `ephemeral_pub(32) || ciphertext`.
5. `POST /contributions/{id}/reveal` com o ciphertext em base64.

## O que sai do enclave

Só o veredito assinado (ed25519): `accepted`, lista de `checks` (nome + passou/não, sem valores) e, se aceito, as métricas numéricas por hectare listadas em `internal/core/payload/metrics.go`. Fornecedores, tipo de solo e histórico de rotação nunca saem individualmente.

Métricas do painel gratuito do produtor: apenas `area_ha` e `total_cost_ha`.
