// Binário que roda DENTRO do AWS Nitro Enclave (adapter REAL — NÃO ativo no pitch).
//
// Responsabilidades:
//   - gerar as chaves x25519 (cifra do payload) e ed25519 (assinatura do veredito) no boot,
//     em memória, nunca persistidas — a chave privada só existe dentro do enclave;
//   - pedir ao NSM um attestation document com user_data = "box_pub|sign_pub";
//   - atender requests do host via vsock (protocolo em pkg/adapter/enclave/nitro);
//   - decifrar, validar (internal/core/validate — o MESMO motor do mock) e assinar.
//
// Build: CGO_ENABLED=0 GOOS=linux go build -o enclave ./cmd/enclave, empacotar num EIF
// com nitro-cli build-enclave. O enclave não tem rede nem disco: as referências (CONAB)
// chegam dentro do request.
//
// Flag -tcp permite rodar fora do Nitro (sem atestação) para teste do protocolo.
package main

import (
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
	"github.com/AgroBench/backend/pkg/adapter/enclave/nitro"
)

func main() {
	tcpAddr := flag.String("tcp", "", "escuta TCP em vez de vsock (teste local, sem atestação)")
	port := flag.Uint("port", uint(nitro.DefaultPort), "porta vsock")
	flag.Parse()

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, nil)))

	srv, err := nitro.NewServer()
	if err != nil {
		fatal("iniciando enclave", err)
	}

	var ln net.Listener
	if *tcpAddr != "" {
		srv.Attest = func([]byte) ([]byte, error) { return nil, nil } // sem NSM fora do Nitro
		ln, err = net.Listen("tcp", *tcpAddr)
	} else {
		srv.Attest = nsmAttest
		ln, err = vsock.Listen(uint32(*port), nil)
	}
	if err != nil {
		fatal("listen", err)
	}
	slog.Info("enclave pronto", "addr", ln.Addr().String(), "box_public_key", crypto.EncodeKey(srv.BoxPublicKey()))

	for {
		conn, err := ln.Accept()
		if err != nil {
			slog.Error("accept", "err", err)
			continue
		}
		go srv.Handle(conn)
	}
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
