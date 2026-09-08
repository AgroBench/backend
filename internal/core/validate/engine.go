// Package validate é o motor de validação que roda DENTRO do ambiente de validação.
// É compartilhado por:
//   - pkg/adapter/enclave/mock (roda no processo da API — ATIVO NO PITCH)
//   - cmd/enclave (binário que roda dentro do AWS Nitro Enclave — REAL)
//
// Recebe o texto puro já decifrado e devolve checks + métricas. Não assina: a assinatura
// é responsabilidade de quem detém a chave ed25519 (o adapter/enclave).
package validate

import (
	"fmt"

	"github.com/google/uuid"

	"github.com/AgroBench/backend/internal/core/domain"
	"github.com/AgroBench/backend/internal/core/payload"
	"github.com/AgroBench/backend/pkg/port"
)

const (
	CheckHashMatchesCommit = "hash_matches_commit"
	CheckSchemaValid       = "schema_valid"
	CheckLevelMatches      = "level_matches"
	CheckCycleMatches      = "cycle_matches"
	CheckReferenceRange    = "reference_range" // um por métrica com faixa: reference_range:<metric>
)

type Input struct {
	ContributionID uuid.UUID
	CycleID        uuid.UUID
	Level          domain.ContributionLevel
	CommitHash     string
	Plaintext      []byte
	References     []port.ReferenceRange
}

type Result struct {
	Accepted bool
	Checks   []port.Check
	Metrics  []port.Metric // vazio se rejeitado
}

// Run executa todos os checks. A ordem importa: sem hash correto ou schema válido, o
// restante nem roda (evita processar payload que não é o commitado).
func Run(in Input) Result {
	var checks []port.Check
	fail := func(name, note string) Result {
		checks = append(checks, port.Check{Name: name, Passed: false, Note: note})
		return Result{Accepted: false, Checks: checks}
	}
	pass := func(name string) { checks = append(checks, port.Check{Name: name, Passed: true}) }

	if payload.Hash(in.Plaintext) != in.CommitHash {
		return fail(CheckHashMatchesCommit, "hash do payload difere do commit")
	}
	pass(CheckHashMatchesCommit)

	parsed, err := payload.Parse(in.Plaintext)
	if err != nil {
		return fail(CheckSchemaValid, "payload fora do schema do nível")
	}
	pass(CheckSchemaValid)

	if parsed.Envelope.Level != in.Level {
		return fail(CheckLevelMatches, "nível do envelope difere do commit")
	}
	pass(CheckLevelMatches)

	if parsed.Envelope.CycleID != in.CycleID {
		return fail(CheckCycleMatches, "ciclo do envelope difere do commit")
	}
	pass(CheckCycleMatches)

	metrics := parsed.Metrics()
	byName := make(map[string]float64, len(metrics))
	for _, m := range metrics {
		byName[m.Name] = m.Value
	}

	accepted := true
	for _, ref := range in.References {
		v, ok := byName[ref.Metric]
		if !ok {
			continue // nível não fornece essa métrica; não penaliza
		}
		name := fmt.Sprintf("%s:%s", CheckReferenceRange, ref.Metric)
		if v < ref.Min || v > ref.Max {
			accepted = false
			checks = append(checks, port.Check{Name: name, Passed: false, Note: "fora da faixa esperada para a região"})
			continue
		}
		pass(name)
	}
	if !accepted {
		return Result{Accepted: false, Checks: checks}
	}

	out := make([]port.Metric, len(metrics))
	for i, m := range metrics {
		out[i] = port.Metric{Name: m.Name, Value: m.Value}
	}
	return Result{Accepted: true, Checks: checks, Metrics: out}
}
