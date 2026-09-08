package nitro

import (
	"github.com/AgroBench/backend/pkg/port"
)

// Protocolo host ↔ enclave sobre vsock: um JSON por conexão, request → response.
// Compartilhado com cmd/enclave (que importa este pacote só pelos tipos).

const (
	OpIdentity = "identity"
	OpValidate = "validate"

	// DefaultPort é a porta vsock do enclave. CID é descoberto pelo host (nitro-cli describe-enclaves).
	DefaultPort uint32 = 5000
)

type Request struct {
	Op       string                `json:"op"`
	Validate *port.ValidateRequest `json:"validate,omitempty"` // Ciphertext vai em base64 pelo encoding/json
}

type Response struct {
	Error    string                `json:"error,omitempty"`
	Identity *port.EnclaveIdentity `json:"identity,omitempty"`
	Verdict  *port.Verdict         `json:"verdict,omitempty"`
}
