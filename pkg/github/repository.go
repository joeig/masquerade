package github

import "github.com/google/go-github/v52/github"

type Repository struct {
	repository *github.Repository
}

func (r *Repository) GetRepoRoot() string {
	if r.repository == nil {
		return ""
	}
	return r.repository.GetHTMLURL()
}

func (r *Repository) GetProjectWebsiteOrFallback(fallback string) string {
	if r.repository == nil {
		return fallback
	}

	projectWebsite := r.repository.GetHomepage()
	if projectWebsite == "" {
		return fallback
	}

	return projectWebsite
}
