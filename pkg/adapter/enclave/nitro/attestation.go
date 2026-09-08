package nitro

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/veraison/go-cose"
)

// AttestationDocument é o payload CBOR do COSE_Sign1 emitido pelo Nitro Security Module.
// Campos conforme a documentação da AWS ("Attestation document specification").
type AttestationDocument struct {
	ModuleID    string          `cbor:"module_id"`
	Digest      string          `cbor:"digest"` // "SHA384"
	Timestamp   uint64          `cbor:"timestamp"`
	PCRs        map[uint][]byte `cbor:"pcrs"`
	Certificate []byte          `cbor:"certificate"` // DER, folha
	CABundle    [][]byte        `cbor:"cabundle"`    // DER, raiz → intermediárias
	PublicKey   []byte          `cbor:"public_key,omitempty"`
	UserData    []byte          `cbor:"user_data,omitempty"`
	Nonce       []byte          `cbor:"nonce,omitempty"`
}

// VerifyAttestation faz a verificação completa do documento:
//  1. decodifica o COSE_Sign1 e o payload CBOR;
//  2. valida a cadeia de certificados (folha → cabundle → raiz AWS informada);
//  3. confere a assinatura COSE (ES384) com a chave da folha;
//  4. compara os PCRs esperados.
func VerifyAttestation(raw []byte, rootPEM []byte, expectedPCRs map[int]string, now time.Time) (*AttestationDocument, error) {
	if len(raw) == 0 {
		return nil, errors.New("documento vazio")
	}

	var msg cose.Sign1Message
	if err := msg.UnmarshalCBOR(raw); err != nil {
		// Alguns emissores omitem a tag COSE_Sign1 (18); tenta com a tag prefixada.
		tagged := append([]byte{0xd2}, raw...)
		if err2 := msg.UnmarshalCBOR(tagged); err2 != nil {
			return nil, fmt.Errorf("COSE_Sign1 inválido: %w", err)
		}
	}

	var doc AttestationDocument
	if err := cbor.Unmarshal(msg.Payload, &doc); err != nil {
		return nil, fmt.Errorf("payload CBOR inválido: %w", err)
	}

	leaf, err := x509.ParseCertificate(doc.Certificate)
	if err != nil {
		return nil, fmt.Errorf("certificado folha: %w", err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(rootPEM) {
		return nil, errors.New("root_cert_pem não contém certificado válido")
	}
	inters := x509.NewCertPool()
	for _, der := range doc.CABundle {
		c, err := x509.ParseCertificate(der)
		if err != nil {
			return nil, fmt.Errorf("cabundle: %w", err)
		}
		inters.AddCert(c)
	}
	if _, err := leaf.Verify(x509.VerifyOptions{
		Roots:         roots,
		Intermediates: inters,
		CurrentTime:   now,
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
	}); err != nil {
		return nil, fmt.Errorf("cadeia de certificados: %w", err)
	}

	pub, ok := leaf.PublicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, errors.New("chave da folha não é ECDSA")
	}
	verifier, err := cose.NewVerifier(cose.AlgorithmES384, pub)
	if err != nil {
		return nil, err
	}
	if err := msg.Verify(nil, verifier); err != nil {
		return nil, fmt.Errorf("assinatura COSE: %w", err)
	}

	for idx, wantHex := range expectedPCRs {
		got, ok := doc.PCRs[uint(idx)]
		if !ok {
			return nil, fmt.Errorf("PCR%d ausente no documento", idx)
		}
		if !strings.EqualFold(hex.EncodeToString(got), wantHex) {
			return nil, fmt.Errorf("PCR%d divergente (esperado %s…, obtido %s…)", idx, wantHex[:16], pcrHex(got)[:16])
		}
	}
	return &doc, nil
}

// ParseRootPEM é helper para carregar a raiz da AWS de um arquivo/config.
func ParseRootPEM(data []byte) error {
	block, _ := pem.Decode(data)
	if block == nil {
		return errors.New("PEM inválido")
	}
	_, err := x509.ParseCertificate(block.Bytes)
	return err
}
