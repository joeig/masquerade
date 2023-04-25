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

type GitHub struct {
	client  *github.Client
	limiter *rate.Limiter
	owner   string
}

func New(client *github.Client, limiter *rate.Limiter, owner string) *GitHub {
	return &GitHub{
		client:  client,
		limiter: limiter,
		owner:   owner,
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

	data, _, err := g.client.Repositories.Get(ctx, g.owner, repo)
	if err != nil {
		return nil, err
	}

	return &Repository{repository: data}, nil
}
