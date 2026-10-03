package domain

import "errors"

var (
	ErrDomain   = errors.New("domain error")
	ErrNotFound = errors.New("not found entity")
)
