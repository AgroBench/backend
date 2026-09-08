package apperrors

import "net/http"

const (
	ErrInvalidInput       = "INVALID_INPUT"
	ErrNotFound           = "NOT_FOUND"
	ErrConflict           = "CONFLICT"
	ErrUnauthorized       = "UNAUTHORIZED"
	ErrForbidden          = "FORBIDDEN"
	ErrExternalDependency = "EXTERNAL_DEPENDENCY_ERROR"
	ErrInternal           = "INTERNAL_ERROR"
	ErrConfigMissing      = "CONFIG_MISSING"
)

type ErrorDefinition struct {
	Code       string
	Message    string
	HTTPStatus int
}

var Catalog = map[string]ErrorDefinition{
	ErrInvalidInput:       {ErrInvalidInput, "Requisição inválida", http.StatusBadRequest},
	ErrNotFound:           {ErrNotFound, "Recurso não encontrado", http.StatusNotFound},
	ErrConflict:           {ErrConflict, "Conflito ao processar requisição", http.StatusConflict},
	ErrUnauthorized:       {ErrUnauthorized, "Não autorizado", http.StatusUnauthorized},
	ErrForbidden:          {ErrForbidden, "Acesso negado", http.StatusForbidden},
	ErrExternalDependency: {ErrExternalDependency, "Erro em dependência externa", http.StatusBadGateway},
	ErrInternal:           {ErrInternal, "Erro interno do servidor", http.StatusInternalServerError},
	ErrConfigMissing:      {ErrConfigMissing, "Configuração ausente", http.StatusInternalServerError},
}
