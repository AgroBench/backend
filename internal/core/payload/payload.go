// Package payload define o formato do dado que o produtor contribui, por nível de
// granularidade (README raiz §6.2), e a regra do hash do commit.
//
// Regra do hash: commit_hash = sha256 dos BYTES EXATOS do JSON em texto puro que o app
// vai cifrar no reveal. Não há canonicalização no servidor: o enclave decifra, faz sha256
// dos bytes e compara com o commit. O app deve guardar os bytes serializados entre o
// commit e o reveal (ou usar Canonicalize, que é determinístico). Ver docs/payload.md.
package payload

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/AgroBench/backend/internal/core/crypto"
	"github.com/AgroBench/backend/internal/core/domain"
	"github.com/AgroBench/backend/internal/core/httpx"
)

const Version = 1

// Envelope é o documento completo em texto puro. `nonce` é aleatório (>= 16 bytes hex),
// gerado pelo app, e existe para impedir que um terceiro adivinhe o hash de um payload
// pequeno por força bruta (commit-reveal com dado de baixa entropia).
type Envelope struct {
	Version int                      `json:"version" validate:"required,eq=1"`
	Level   domain.ContributionLevel `json:"level"   validate:"required,oneof=basic intermediate advanced"`
	CycleID uuid.UUID                `json:"cycle_id" validate:"required"`
	Nonce   string                   `json:"nonce"   validate:"required,hexadecimal,min=32"`
	Data    json.RawMessage          `json:"data"    validate:"required"`
}

// Parsed é o envelope já decodificado no struct do nível correspondente.
type Parsed struct {
	Envelope Envelope
	Basic    *Basic        // sempre preenchido
	Inter    *Intermediate // preenchido se level >= intermediate
	Adv      *Advanced     // preenchido se level == advanced
}

var ErrLevelMismatch = errors.New("payload: nível do envelope não bate com o esperado")

// Parse valida o envelope e o `data` conforme o nível declarado.
func Parse(plaintext []byte) (*Parsed, error) {
	var env Envelope
	if err := json.Unmarshal(plaintext, &env); err != nil {
		return nil, fmt.Errorf("payload: envelope inválido: %w", err)
	}
	if err := httpx.Validate(&env); err != nil {
		return nil, fmt.Errorf("payload: %w", err)
	}

	p := &Parsed{Envelope: env}
	switch env.Level {
	case domain.LevelBasic:
		var d Basic
		if err := decodeStrict(env.Data, &d); err != nil {
			return nil, err
		}
		p.Basic = &d
	case domain.LevelIntermediate:
		var d Intermediate
		if err := decodeStrict(env.Data, &d); err != nil {
			return nil, err
		}
		p.Inter, p.Basic = &d, &d.Basic
	case domain.LevelAdvanced:
		var d Advanced
		if err := decodeStrict(env.Data, &d); err != nil {
			return nil, err
		}
		p.Adv, p.Inter, p.Basic = &d, &d.Intermediate, &d.Basic
	}
	return p, nil
}

func decodeStrict(raw json.RawMessage, dst any) error {
	if err := json.Unmarshal(raw, dst); err != nil {
		return fmt.Errorf("payload: data inválido: %w", err)
	}
	if err := httpx.Validate(dst); err != nil {
		return fmt.Errorf("payload: %w", err)
	}
	return nil
}

// Hash calcula o commit_hash dos bytes exatos do payload.
func Hash(plaintext []byte) string { return crypto.SHA256Hex(plaintext) }

// Canonicalize re-serializa um JSON com chaves ordenadas e sem espaços, preservando os
// números como texto (json.Number). Útil para quem gera payload em Go (seeds, testes).
func Canonicalize(raw []byte) ([]byte, error) {
	dec := json.NewDecoder(bytesReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	return json.Marshal(v) // encoding/json ordena chaves de map e não emite espaços
}

// Build monta o envelope e devolve os bytes canônicos prontos para Hash e Seal.
func Build(level domain.ContributionLevel, cycleID uuid.UUID, nonce string, data any) ([]byte, error) {
	rawData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	env := Envelope{Version: Version, Level: level, CycleID: cycleID, Nonce: nonce, Data: rawData}
	raw, err := json.Marshal(env)
	if err != nil {
		return nil, err
	}
	return Canonicalize(raw)
}
