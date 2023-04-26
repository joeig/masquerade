package repository

type Repository interface {
	GetRepoRoot() string
	GetProjectWebsiteOrFallback(fallback string) string
}
