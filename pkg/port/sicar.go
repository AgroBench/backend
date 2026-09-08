package port

import "context"

type CARRecord struct {
	Exists   bool
	Active   bool
	UF       string
	IBGECode string // código IBGE do município ou microrregião, se a fonte informar
}

// SicarClient consulta o Cadastro Ambiental Rural (README raiz §6.3).
//
// Implementações:
//   - pkg/adapter/sicar/mock → ATIVA NO PITCH. Lê a tabela seed mock_sicar_cars.
//   - pkg/adapter/sicar/http → REAL. Client HTTP para a consulta pública do SICAR.
//     TODO: endpoint público não confirmado; ver comentário no adapter.
//
// Seleção: config `adapters.sicar` (mock | http).
type SicarClient interface {
	Lookup(ctx context.Context, car string) (CARRecord, error)
}
