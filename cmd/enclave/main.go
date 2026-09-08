// Binário que roda DENTRO do AWS Nitro Enclave (adapter REAL — NÃO ativo no pitch).
//
// Responsabilidades:
//   - gerar as chaves x25519 (cifra do payload) e ed25519 (assinatura do veredito) no boot,
//     em memória, nunca persistidas — a chave privada só existe dentro do enclave;
//   - pedir ao NSM um attestation document com user_data = "box_pub|sign_pub";
//   - atender requests do host via vsock (protocolo em pkg/adapter/enclave/nitro/protocol.go);
//   - decifrar, validar (internal/core/validate — o MESMO motor do mock) e assinar.
//
// Build: CGO_ENABLED=0 GOOS=linux go build -o enclave ./cmd/enclave, empacotar num EIF
// com nitro-cli build-enclave. O enclave não tem rede nem disco: as referências (CONAB)
// chegam dentro do request.
//
// Flag -tcp permite rodar fora do Nitro (sem atestação) para teste do protocolo.
package main

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"

	"github.com/hf/nsm"
	"github.com/hf/nsm/request"
	"github.com/mdlayher/vsock"

	"github.com/AgroBench/backend/internal/core/crypto"
	"github.com/AgroBench/backend/internal/core/validate"
	"github.com/AgroBench/backend/pkg/adapter/enclave/nitro"
	"github.com/AgroBench/backend/pkg/port"
)

type enclave struct {
	boxPub  *[crypto.KeySize]byte
	boxPriv *[crypto.KeySize]byte
	signKey ed25519.PrivateKey
	attest  func(userData []byte) ([]byte, error)
}

func main() {
	tcpAddr := flag.String("tcp", "", "escuta TCP em vez de vsock (teste local, sem atestação)")
	port := flag.Uint("port", uint(nitro.DefaultPort), "porta vsock")
	flag.Parse()

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, nil)))

	boxPub, boxPriv, err := crypto.GenerateBoxKeyPair()
	if err != nil {
		fatal("gerando x25519", err)
	}
	_, signKey, err := crypto.GenerateSigningKey()
	if err != nil {
		fatal("gerando ed25519", err)
	}
	e := &enclave{boxPub: boxPub, boxPriv: boxPriv, signKey: signKey, attest: nsmAttest}

	var ln net.Listener
	if *tcpAddr != "" {
		e.attest = func([]byte) ([]byte, error) { return nil, nil } // sem NSM fora do Nitro
		ln, err = net.Listen("tcp", *tcpAddr)
	} else {
		ln, err = vsock.Listen(uint32(*port), nil)
	}
	if err != nil {
		fatal("listen", err)
	}
	slog.Info("enclave pronto", "addr", ln.Addr().String(), "box_public_key", crypto.EncodeKey(boxPub))

	for {
		conn, err := ln.Accept()
		if err != nil {
			slog.Error("accept", "err", err)
			continue
		}
		go e.handle(conn)
	}
}

func (e *enclave) handle(conn net.Conn) {
	defer conn.Close()
	var req nitro.Request
	if err := json.NewDecoder(conn).Decode(&req); err != nil {
		_ = json.NewEncoder(conn).Encode(nitro.Response{Error: "request inválido: " + err.Error()})
		return
	}
	res := e.dispatch(context.Background(), req)
	_ = json.NewEncoder(conn).Encode(res)
}

func (e *enclave) dispatch(_ context.Context, req nitro.Request) nitro.Response {
	switch req.Op {
	case nitro.OpIdentity:
		id := port.EnclaveIdentity{
			BoxPublicKey:     crypto.EncodeKey(e.boxPub),
			SigningPublicKey: fmt.Sprintf("%x", e.signKey.Public().(ed25519.PublicKey)),
			Provider:         "nitro",
		}
		doc, err := e.attest([]byte(id.BoxPublicKey + "|" + id.SigningPublicKey))
		if err != nil {
			return nitro.Response{Error: "atestação: " + err.Error()}
		}
		id.Attestation = doc
		return nitro.Response{Identity: &id}

	case nitro.OpValidate:
		if req.Validate == nil {
			return nitro.Response{Error: "validate ausente"}
		}
		v := e.validate(*req.Validate)
		return nitro.Response{Verdict: &v}
	}
	return nitro.Response{Error: "op desconhecida: " + req.Op}
}

func (e *enclave) validate(req port.ValidateRequest) port.Verdict {
	v := port.Verdict{ContributionID: req.ContributionID, CommitHash: req.CommitHash}
	plain, err := crypto.Open(req.Ciphertext, e.boxPub, e.boxPriv)
	if err != nil {
		v.Checks = []port.Check{{Name: "decrypt", Passed: false, Note: "payload não decifrável por este enclave"}}
	} else {
		res := validate.Run(validate.Input{
			ContributionID: req.ContributionID, CycleID: req.CycleID, Level: req.Level,
			CommitHash: req.CommitHash, Plaintext: plain, References: req.References,
		})
		v.Accepted, v.Checks, v.Metrics = res.Accepted, res.Checks, res.Metrics
	}
	if err := validate.Sign(&v, e.signKey); err != nil {
		slog.Error("assinando veredito", "err", err)
	}
	return v
}

// nsmAttest pede ao Nitro Security Module (/dev/nsm) um attestation document.
func nsmAttest(userData []byte) ([]byte, error) {
	sess, err := nsm.OpenDefaultSession()
	if err != nil {
		return nil, fmt.Errorf("abrindo /dev/nsm: %w", err)
	}
	defer sess.Close()

	res, err := sess.Send(&request.Attestation{UserData: userData})
	if err != nil {
		return nil, err
	}
	if res.Error != "" {
		return nil, errors.New(string(res.Error))
	}
	if res.Attestation == nil || len(res.Attestation.Document) == 0 {
		return nil, errors.New("NSM devolveu documento vazio")
	}
	return res.Attestation.Document, nil
}

func fatal(msg string, err error) {
	slog.Error(msg, "err", err)
	os.Exit(1)
}
