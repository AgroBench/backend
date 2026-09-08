// Package domain reúne tipos de valor compartilhados por todos os módulos.
package domain

import "github.com/google/uuid"

// NewID gera um UUID v7 (ordenável por tempo). Toda chave primária do sistema usa isto.
func NewID() uuid.UUID {
	id, err := uuid.NewV7()
	if err != nil {
		// uuid.NewV7 só falha se o gerador de entropia do SO falhar; não há recuperação razoável.
		panic("uuid v7: " + err.Error())
	}
	return id
}
