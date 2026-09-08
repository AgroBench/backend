// Package mock é a implementação de port.Enclave ATIVA NO PITCH.
//
// A chave privada x25519 vive no processo da API (config `enclave.mock.box_private_key`,
// ou gerada no boot se vazia). Não há isolamento de hardware: o operador da infra PODE
// ler o payload aqui. A separação de responsabilidades (quem decifra × quem agrega) está
// preservada na arquitetura, mas não na garantia — isso é exatamente o que o adapter
// nitro resolve. Ver README §5 (ports) e §14, limitação 2 ("Limitações assumidas").
package mock

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"log/slog"

	"github.com/AgroBench/backend/internal/core/crypto"
	"github.com/AgroBench/backend/internal/core/validate"
	"github.com/AgroBench/backend/pkg/port"
)

type Enclave struct {
	boxPub  *[crypto.KeySize]byte
	boxPriv *[crypto.KeySize]byte
	signKey ed25519.PrivateKey
}

// Config recebe as chaves em hex. Vazias → geradas no boot (e logadas em nível WARN, porque
// cada restart invalida os payloads cifrados com a chave anterior).
type Config struct {
	BoxPrivateKeyHex      string
	SigningPrivateSeedHex string
}

func New(cfg Config) (*Enclave, error) {
	e := &Enclave{}

	if cfg.BoxPrivateKeyHex != "" {
		priv, err := crypto.ParseKey32(cfg.BoxPrivateKeyHex)
		if err != nil {
			return nil, fmt.Errorf("enclave mock: box_private_key: %w", err)
		}
		e.boxPriv = priv
		e.boxPub = publicFromPrivate(priv)
	} else {
		pub, priv, err := crypto.GenerateBoxKeyPair()
		if err != nil {
			return nil, err
		}
		e.boxPub, e.boxPriv = pub, priv
		slog.Warn("enclave mock: chave x25519 gerada no boot; payloads cifrados antes do restart não serão decifráveis",
			"box_public_key", crypto.EncodeKey(pub))
	}

	if cfg.SigningPrivateSeedHex != "" {
		k, err := crypto.ParseSigningSeed(cfg.SigningPrivateSeedHex)
		if err != nil {
			return nil, fmt.Errorf("enclave mock: signing_private_seed: %w", err)
		}
		e.signKey = k
	} else {
		_, k, err := crypto.GenerateSigningKey()
		if err != nil {
			return nil, err
		}
		e.signKey = k
		slog.Warn("enclave mock: chave ed25519 gerada no boot; vereditos antigos não serão verificáveis com a nova chave")
	}

	return e, nil
}

func (e *Enclave) Identity(context.Context) (port.EnclaveIdentity, error) {
	return port.EnclaveIdentity{
		BoxPublicKey:     crypto.EncodeKey(e.boxPub),
		SigningPublicKey: fmt.Sprintf("%x", e.signKey.Public().(ed25519.PublicKey)),
		Attestation:      nil, // mock não tem attestation document
		Provider:         "mock",
	}, nil
}

func (e *Enclave) Validate(_ context.Context, req port.ValidateRequest) (port.Verdict, error) {
	v := port.Verdict{ContributionID: req.ContributionID, CommitHash: req.CommitHash}

	plain, err := crypto.Open(req.Ciphertext, e.boxPub, e.boxPriv)
	if err != nil {
		// Não é erro de infra: é um payload que não foi cifrado para este enclave. Vira rejeição.
		v.Accepted = false
		v.Checks = []port.Check{{Name: "decrypt", Passed: false, Note: "payload não decifrável por este enclave"}}
		return e.sign(v)
	}

	res := validate.Run(validate.Input{
		ContributionID: req.ContributionID,
		CycleID:        req.CycleID,
		Level:          req.Level,
		CommitHash:     req.CommitHash,
		Plaintext:      plain,
		References:     req.References,
	})
	v.Accepted, v.Checks, v.Metrics = res.Accepted, res.Checks, res.Metrics
	return e.sign(v)
}

func (e *Enclave) sign(v port.Verdict) (port.Verdict, error) {
	if err := validate.Sign(&v, e.signKey); err != nil {
		return port.Verdict{}, fmt.Errorf("enclave mock: assinando veredito: %w", err)
	}
	return v, nil
}

// BoxPublicKey expõe a chave pública x25519 (usado por testes e seeds para cifrar).
func (e *Enclave) BoxPublicKey() *[crypto.KeySize]byte { return e.boxPub }
