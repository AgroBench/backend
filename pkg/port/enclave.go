package port

import (
	"context"

	"github.com/google/uuid"

	"github.com/AgroBench/backend/internal/core/domain"
)

// EnclaveIdentity é o que o produtor precisa para cifrar: a chave x25519 do ambiente de
// validação e a prova de que ela pertence a um enclave legítimo (attestation).
type EnclaveIdentity struct {
	BoxPublicKey     string `json:"box_public_key"`     // x25519, hex
	SigningPublicKey string `json:"signing_public_key"` // ed25519, hex — verifica vereditos
	Attestation      []byte `json:"attestation"`        // Nitro: attestation document (COSE). Mock: nil
	Provider         string `json:"provider"`           // "mock" | "nitro"
}

// ReferenceRange é a faixa esperada de uma métrica para cultura × microrregião (mock CONAB).
type ReferenceRange struct {
	Metric string
	Min    float64
	Max    float64
}

type ValidateRequest struct {
	ContributionID uuid.UUID
	CycleID        uuid.UUID
	Level          domain.ContributionLevel
	CommitHash     string
	Ciphertext     []byte // sealed box
	References     []ReferenceRange
}

type Check struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Note   string `json:"note,omitempty"` // genérica; nunca contém valor do produtor
}

type Metric struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}

// Verdict é o resultado assinado da validação. Signature cobre SignedPayload().
//
// LIMITAÇÃO ASSUMIDA DO MVP (README §14, limitação 1): Metrics sai do enclave
// em texto puro, ligado a ContributionID. São métricas numéricas por hectare, sem nome, CPF,
// CAR ou coordenada — mas ainda vinculáveis à wallet pelo contribution_id. Isso é o que
// permite a agregação rodar em SQL fora do enclave. Evolução prevista: agregação dentro do
// enclave (devolvendo só médias/medianas por ciclo) ou aprendizado federado.
type Verdict struct {
	ContributionID uuid.UUID `json:"contribution_id"`
	CommitHash     string    `json:"commit_hash"`
	Accepted       bool      `json:"accepted"`
	Checks         []Check   `json:"checks"`
	Metrics        []Metric  `json:"metrics,omitempty"` // só quando Accepted
	SignerPubKey   string    `json:"signer_public_key"` // ed25519 hex
	Signature      []byte    `json:"signature"`
}

// Enclave é o ambiente isolado que decifra e valida o payload (README raiz §3).
//
// Implementações:
//   - pkg/adapter/enclave/mock  → ATIVA NO PITCH. Decifra NO MESMO PROCESSO da API com a chave
//     x25519 de config (ou gerada no boot). Não há isolamento de hardware: é só a arquitetura.
//   - pkg/adapter/enclave/nitro → REAL. Lado host de um AWS Nitro Enclave: fala com cmd/enclave
//     via vsock, verifica o attestation document (cadeia de certificados AWS + PCRs esperados)
//     antes de confiar na chave pública devolvida.
//
// Ambas usam internal/core/validate para as regras — a diferença é só ONDE a chave privada vive.
// Seleção: config `adapters.enclave` (mock | nitro).
type Enclave interface {
	Identity(ctx context.Context) (EnclaveIdentity, error)
	Validate(ctx context.Context, req ValidateRequest) (Verdict, error)
}
