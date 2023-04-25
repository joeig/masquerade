package github

import (
	"github.com/google/go-github/v52/github"
	"testing"
)

func TestRepository_RepoRoot(t *testing.T) {
	repo := &Repository{repository: &github.Repository{HTMLURL: github.String("https://example.com")}}

	if repo.RepoRoot() != "https://example.com" {
		t.Error("wrong repo root")
	}
}

func TestRepository_ProjectWebsite(t *testing.T) {
	repo := &Repository{repository: &github.Repository{Homepage: github.String("https://example.com")}}

	if repo.ProjectWebsite() != "https://example.com" {
		t.Error("wrong project website")
	}
}
