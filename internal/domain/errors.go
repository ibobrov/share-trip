package domain

import "errors"

var (
	ErrDomain        = errors.New("domain error")
	ErrNotFound      = errors.New("not found entity")
	ErrForbidden     = errors.New("forbidden")
	ErrConflict      = errors.New("conflict")
	ErrSkipOperation = errors.New("skip operation")
)
