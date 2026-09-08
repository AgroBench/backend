// Package nitro é a implementação REAL de port.Enclave (NÃO ativa no pitch): o lado HOST
// de um AWS Nitro Enclave. O código que roda DENTRO do enclave é cmd/enclave.
//
// Fluxo:
//  1. Identity(): conecta via vsock, pede a identidade, verifica o attestation document
//     (COSE_Sign1 assinado pelo NSM, cadeia até a raiz da AWS, PCRs esperados) e confere
//     que as chaves públicas devolvidas estão no user_data do documento. Só então confia.
//  2. Validate(): manda o ciphertext pro enclave, recebe o veredito assinado e confere a
//     assinatura contra a chave verificada no passo 1.
//
// Status: semi-pronto. Segue o formato documentado do attestation document
// (https://docs.aws.amazon.com/enclaves/latest/user/verify-root.html). Não foi executado
// em uma instância Nitro dentro do escopo do MVP.
package nitro

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/mdlayher/vsock"

	"github.com/AgroBench/backend/internal/core/validate"
	"github.com/AgroBench/backend/pkg/port"
)

type Config struct {
	CID          uint32         // context id do enclave (nitro-cli describe-enclaves → EnclaveCID)
	Port         uint32         // default 5000
	RootCertPEM  []byte         // raiz da AWS Nitro Attestation PKI
	ExpectedPCRs map[int]string // PCR index → hex esperado (0, 1, 2 pelo menos)
	Timeout      time.Duration  // por request
	// Dial permite substituir o vsock por TCP em testes locais (host sem Nitro).
	Dial func(ctx context.Context) (net.Conn, error)
}

type Enclave struct {
	cfg Config

	mu       sync.RWMutex
	identity *port.EnclaveIdentity // cache após verificação bem-sucedida
}

func New(cfg Config) (*Enclave, error) {
	if cfg.Port == 0 {
		cfg.Port = DefaultPort
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	if len(cfg.RootCertPEM) == 0 {
		return nil, errors.New("enclave nitro: root_cert_pem é obrigatório para verificar a atestação")
	}
	if cfg.Dial == nil {
		cid, p := cfg.CID, cfg.Port
		cfg.Dial = func(context.Context) (net.Conn, error) { return vsock.Dial(cid, p, nil) }
	}
	return &Enclave{cfg: cfg}, nil
}

func (e *Enclave) Identity(ctx context.Context) (port.EnclaveIdentity, error) {
	e.mu.RLock()
	if e.identity != nil {
		id := *e.identity
		e.mu.RUnlock()
		return id, nil
	}
	e.mu.RUnlock()

	var res Response
	if err := e.roundTrip(ctx, Request{Op: OpIdentity}, &res); err != nil {
		return port.EnclaveIdentity{}, err
	}
	if res.Identity == nil {
		return port.EnclaveIdentity{}, errors.New("enclave nitro: resposta sem identidade")
	}
	id := *res.Identity

	doc, err := VerifyAttestation(id.Attestation, e.cfg.RootCertPEM, e.cfg.ExpectedPCRs, time.Now())
	if err != nil {
		return port.EnclaveIdentity{}, fmt.Errorf("enclave nitro: atestação rejeitada: %w", err)
	}
	// O enclave coloca "box_pub|sign_pub" (hex) no user_data. É o vínculo chave ↔ enclave medido.
	want := []byte(id.BoxPublicKey + "|" + id.SigningPublicKey)
	if !bytes.Equal(doc.UserData, want) {
		return port.EnclaveIdentity{}, errors.New("enclave nitro: chaves públicas não batem com o user_data atestado")
	}

	e.mu.Lock()
	e.identity = &id
	e.mu.Unlock()
	return id, nil
}

func (e *Enclave) Validate(ctx context.Context, req port.ValidateRequest) (port.Verdict, error) {
	id, err := e.Identity(ctx)
	if err != nil {
		return port.Verdict{}, err
	}
	var res Response
	if err := e.roundTrip(ctx, Request{Op: OpValidate, Validate: &req}, &res); err != nil {
		return port.Verdict{}, err
	}
	if res.Verdict == nil {
		return port.Verdict{}, errors.New("enclave nitro: resposta sem veredito")
	}
	v := *res.Verdict
	if v.SignerPubKey != id.SigningPublicKey {
		return port.Verdict{}, errors.New("enclave nitro: veredito assinado por chave diferente da atestada")
	}
	if err := validate.Verify(v, id.SigningPublicKey); err != nil {
		return port.Verdict{}, fmt.Errorf("enclave nitro: %w", err)
	}
	return v, nil
}

func (e *Enclave) roundTrip(ctx context.Context, req Request, res *Response) error {
	ctx, cancel := context.WithTimeout(ctx, e.cfg.Timeout)
	defer cancel()

	conn, err := e.cfg.Dial(ctx)
	if err != nil {
		return fmt.Errorf("enclave nitro: conectando (cid=%d port=%d): %w", e.cfg.CID, e.cfg.Port, err)
	}
	defer conn.Close()
	if dl, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(dl)
	}

	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return fmt.Errorf("enclave nitro: enviando request: %w", err)
	}
	if err := json.NewDecoder(conn).Decode(res); err != nil {
		return fmt.Errorf("enclave nitro: lendo response: %w", err)
	}
	if res.Error != "" {
		return fmt.Errorf("enclave nitro: enclave respondeu erro: %s", res.Error)
	}
	return nil
}

// pcrHex é helper para logs/config.
func pcrHex(b []byte) string { return hex.EncodeToString(b) }
