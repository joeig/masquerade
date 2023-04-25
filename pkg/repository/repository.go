package repository

type Repository interface {
	RepoRoot() string
	ProjectWebsite() string
}
