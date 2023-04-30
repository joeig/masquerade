package repository

import "errors"

type Repository interface {
	GetRepoRoot() string
	GetProjectWebsiteOrFallback(fallback string) string
}

var ErrNotFound = errors.New("not found")
