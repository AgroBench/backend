package validate

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/AgroBench/backend/pkg/port"
)

// signedBody é o que a assinatura do veredito cobre. Serializado com encoding/json
// (chaves em ordem de declaração do struct, sem espaços) — determinístico.
type signedBody struct {
	ContributionID string        `json:"contribution_id"`
	CommitHash     string        `json:"commit_hash"`
	Accepted       bool          `json:"accepted"`
	Checks         []port.Check  `json:"checks"`
	Metrics        []port.Metric `json:"metrics"`
}

func SignedPayload(v port.Verdict) ([]byte, error) {
	return json.Marshal(signedBody{
		ContributionID: v.ContributionID.String(),
		CommitHash:     v.CommitHash,
		Accepted:       v.Accepted,
		Checks:         v.Checks,
		Metrics:        v.Metrics,
	})
}

// Sign preenche SignerPubKey e Signature do veredito.
func Sign(v *port.Verdict, priv ed25519.PrivateKey) error {
	body, err := SignedPayload(*v)
	if err != nil {
		return err
	}
	v.SignerPubKey = hex.EncodeToString(priv.Public().(ed25519.PublicKey))
	v.Signature = ed25519.Sign(priv, body)
	return nil
}

var ErrBadSignature = errors.New("veredito: assinatura inválida")

// Verify confere a assinatura de um veredito contra a chave pública informada (hex).
func Verify(v port.Verdict, signerPubHex string) error {
	pub, err := hex.DecodeString(signerPubHex)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return ErrBadSignature
	}
	body, err := SignedPayload(v)
	if err != nil {
		return err
	}
	if !ed25519.Verify(ed25519.PublicKey(pub), body, v.Signature) {
		return ErrBadSignature
	}
	return nil
}
