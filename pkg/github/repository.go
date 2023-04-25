package github

import "github.com/google/go-github/v52/github"

type Repository struct {
	repository *github.Repository
}

func (r *Repository) RepoRoot() string {
	if r.repository == nil {
		return ""
	}
	return r.repository.GetHTMLURL()
}

func (r *Repository) ProjectWebsite() string {
	if r.repository == nil {
		return ""
	}
	return r.repository.GetHomepage()
}
