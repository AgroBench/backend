package port

import "context"

// ReferenceDataClient fornece as faixas esperadas de custo/produtividade usadas no
// cruzamento com fontes públicas (README raiz §3, "CONAB e sensoriamento remoto").
//
// Implementações:
//   - pkg/adapter/refdata/mock  → ATIVA NO PITCH. Lê a tabela seed mock_reference_ranges.
//   - pkg/adapter/refdata/conab → REAL. Client HTTP para as séries de custo de produção da CONAB.
//     TODO: endpoint público não confirmado; ver comentário no adapter.
//
// Seleção: config `adapters.reference_data` (mock | conab).
type ReferenceDataClient interface {
	// ExpectedRanges devolve as faixas por métrica para cultura × microrregião. Lista vazia
	// significa "sem referência" — o enclave então só valida schema e hash.
	ExpectedRanges(ctx context.Context, cultureCode, ibgeCode string) ([]ReferenceRange, error)
}
