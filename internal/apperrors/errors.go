// Package apperrors define o erro tipado da aplicação e o mapeamento para HTTP.
// Toda camada (repository, usecase, handler, gateway) devolve *AppError; o handler
// só chama WriteHTTPError.
package apperrors

import (
	"errors"
	"fmt"
)

type AppError struct {
	Code           string
	Op             string
	Layer          string
	Detail         string
	ExternalStatus int
	Err            error
}

func (e *AppError) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("[%s] %s (%s): %s — %v", e.Layer, e.Op, e.Code, e.Detail, e.Err)
	}
	return fmt.Sprintf("[%s] %s (%s): %v", e.Layer, e.Op, e.Code, e.Err)
}

func (e *AppError) Unwrap() error { return e.Err }

func New(code, op, layer string, err error) *AppError {
	return &AppError{Code: code, Op: op, Layer: layer, Err: err}
}

func (e *AppError) WithLayer(layer string) *AppError {
	e.Layer = layer
	return e
}

// WithDetail define a mensagem que vai pro cliente no campo "detail". Nunca coloque
// aqui conteúdo de erro interno (SQL, stack); use só texto pensado para o usuário.
func (e *AppError) WithDetail(detail string) *AppError {
	e.Detail = detail
	return e
}

func Is(err error, code string) bool {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Code == code
	}
	return false
}
