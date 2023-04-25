package github

import (
	"context"
	"errors"
	"github.com/google/go-github/v52/github"
	"go.eigsys.de/masquerade/pkg/repository"
	"golang.org/x/time/rate"
	"regexp"
)

var repoRegexp = regexp.MustCompile(`^[a-zA-Z0-9-_.]{1,32}$`)

type RepositoriesService interface {
	Get(ctx context.Context, owner, repo string) (*github.Repository, *github.Response, error)
}

type GitHub struct {
	repositoriesService RepositoriesService
	limiter             *rate.Limiter
	owner               string
}

func New(repositoriesService RepositoriesService, limiter *rate.Limiter, owner string) *GitHub {
	return &GitHub{
		repositoriesService: repositoriesService,
		limiter:             limiter,
		owner:               owner,
	}
}

func (g *GitHub) Type() string {
	return "git"
}

func (g *GitHub) isValidRepo(repo string) bool {
	return repoRegexp.MatchString(repo)
}

func (g *GitHub) Fetch(ctx context.Context, repo string) (repository.Repository, error) {
	if !g.isValidRepo(repo) {
		return nil, errors.New("invalid repo")
	}

	if err := g.limiter.Wait(ctx); err != nil {
		return nil, err
	}

	data, _, err := g.repositoriesService.Get(ctx, g.owner, repo)
	if err != nil {
		return nil, err
	}

	return &Repository{repository: data}, nil
}
