// Package port define as interfaces das dependências externas do AgroBench.
//
// Cada port tem DUAS implementações em pkg/adapter/<nome>/:
//
//	mock/  → ATIVA NO PITCH. Simula o serviço no Postgres/memória. Nunca sai do ambiente local.
//	<real>/ → integração real, semi-pronta, sem teste em ambiente real dentro do escopo do MVP.
//
// A seleção é feita por config (`adapters.<nome>` = mock | <real>) em pkg/adapter/registry.
// O comentário no topo de cada interface diz qual é qual.
package port

import "errors"

// ErrNotImplemented é devolvido por métodos dos adapters reais que ainda não foram
// concluídos. O chamador deve tratar como falha de dependência externa.
var ErrNotImplemented = errors.New("port: não implementado neste adapter")
