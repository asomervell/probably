package models

import "errors"

var (
	ErrEntriesNotBalanced = errors.New("entries must sum to zero (debits = credits)")
	ErrUnauthorized       = errors.New("unauthorized")
)
