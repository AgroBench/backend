package nitro

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"

	"github.com/AgroBench/backend/internal/core/crypto"
	"github.com/AgroBench/backend/internal/core/validate"
	"github.com/AgroBench/backend/pkg/port"
)

// Server é o lado DENTRO do enclave: gera chaves em memória, atesta via NSM (ou
// função injetada em testes) e atende o protocolo JSON de protocol.go.
// cmd/enclave só escolhe o listener (vsock × TCP) e a fonte de atestação.
type Server struct {
	boxPub  *[crypto.KeySize]byte
	boxPriv *[crypto.KeySize]byte
	signKey ed25519.PrivateKey
	// Attest pede o attestation document com user_data = "box_pub|sign_pub".
	// No Nitro real aponta para /dev/nsm; em testes, um documento COSE assinado localmente.
	Attest func(userData []byte) ([]byte, error)
}

func NewServer() (*Server, error) {
	boxPub, boxPriv, err := crypto.GenerateBoxKeyPair()
	if err != nil {
		return nil, fmt.Errorf("enclave nitro: gerando x25519: %w", err)
	}
	_, signKey, err := crypto.GenerateSigningKey()
	if err != nil {
		return nil, fmt.Errorf("enclave nitro: gerando ed25519: %w", err)
	}
	return &Server{boxPub: boxPub, boxPriv: boxPriv, signKey: signKey}, nil
}

func (s *Server) BoxPublicKey() *[crypto.KeySize]byte { return s.boxPub }

func (s *Server) SigningPublicKeyHex() string {
	return fmt.Sprintf("%x", s.signKey.Public().(ed25519.PublicKey))
}

func (s *Server) Handle(conn net.Conn) {
	defer conn.Close()
	var req Request
	if err := json.NewDecoder(conn).Decode(&req); err != nil {
		_ = json.NewEncoder(conn).Encode(Response{Error: "request inválido: " + err.Error()})
		return
	}
	_ = json.NewEncoder(conn).Encode(s.Dispatch(context.Background(), req))
}

func (s *Server) Dispatch(_ context.Context, req Request) Response {
	switch req.Op {
	case OpIdentity:
		id := port.EnclaveIdentity{
			BoxPublicKey:     crypto.EncodeKey(s.boxPub),
			SigningPublicKey: s.SigningPublicKeyHex(),
			Provider:         "nitro",
		}
		if s.Attest == nil {
			return Response{Error: "atestação não configurada"}
		}
		doc, err := s.Attest([]byte(id.BoxPublicKey + "|" + id.SigningPublicKey))
		if err != nil {
			return Response{Error: "atestação: " + err.Error()}
		}
		id.Attestation = doc
		return Response{Identity: &id}

	case OpValidate:
		if req.Validate == nil {
			return Response{Error: "validate ausente"}
		}
		v := s.validate(*req.Validate)
		return Response{Verdict: &v}
	}
	return Response{Error: "op desconhecida: " + req.Op}
}

func (s *Server) validate(req port.ValidateRequest) port.Verdict {
	v := port.Verdict{ContributionID: req.ContributionID, CommitHash: req.CommitHash}
	plain, err := crypto.Open(req.Ciphertext, s.boxPub, s.boxPriv)
	if err != nil {
		v.Checks = []port.Check{{Name: "decrypt", Passed: false, Note: "payload não decifrável por este enclave"}}
	} else {
		res := validate.Run(validate.Input{
			ContributionID: req.ContributionID, CycleID: req.CycleID, Level: req.Level,
			CommitHash: req.CommitHash, Plaintext: plain, References: req.References,
		})
		v.Accepted, v.Checks, v.Metrics = res.Accepted, res.Checks, res.Metrics
	}
	if err := validate.Sign(&v, s.signKey); err != nil {
		slog.Error("assinando veredito", "err", err)
	}
	return v
}
