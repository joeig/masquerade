package github

import (
	"context"
	"errors"
	"github.com/google/go-github/v52/github"
	"go.eigsys.de/masquerade/pkg/repository"
	"golang.org/x/time/rate"
	"reflect"
	"testing"
)

type mockRepositoriesService struct {
	getRepository *github.Repository
	getResponse   *github.Response
	getError      error
}

func (m *mockRepositoriesService) Get(_ context.Context, _, _ string) (*github.Repository, *github.Response, error) {
	return m.getRepository, m.getResponse, m.getError
}

func TestGitHub_Type(t *testing.T) {
	g := &GitHub{}

	if g.Type() != "git" {
		t.Errorf("wrong type")
	}
}

func TestGitHub_isValidRepo(t *testing.T) {
	type args struct {
		repo string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "valid-1-character",
			args: args{repo: "a"},
			want: true,
		},
		{
			name: "valid-32-characters",
			args: args{repo: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
			want: true,
		},
		{
			name: "valid-special-characters",
			args: args{repo: "-_."},
			want: true,
		},
		{
			name: "invalid-0-characters",
			args: args{repo: ""},
			want: false,
		},
		{
			name: "invalid-33-characters",
			args: args{repo: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
			want: false,
		},
		{
			name: "invalid-character",
			args: args{repo: "/"},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &GitHub{}
			if got := g.isValidRepo(tt.args.repo); got != tt.want {
				t.Errorf("isValidRepo() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGitHub_Fetch(t *testing.T) {
	type fields struct {
		repositoriesService RepositoriesService
		limiter             *rate.Limiter
		owner               string
	}
	type args struct {
		ctx  context.Context
		repo string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    repository.Repository
		wantErr bool
	}{
		{
			name: "ok",
			fields: fields{
				repositoriesService: &mockRepositoriesService{getRepository: &github.Repository{Name: github.String("name")}},
				limiter:             rate.NewLimiter(rate.Inf, 0),
			},
			args: args{
				ctx:  context.Background(),
				repo: "the-repo",
			},
			want: &Repository{repository: &github.Repository{Name: github.String("name")}},
		},
		{
			name: "invalid-repo",
			args: args{
				repo: "the/repo",
			},
			wantErr: true,
		},
		{
			name: "limiter-error",
			fields: fields{
				limiter: rate.NewLimiter(0, 0),
			},
			args: args{
				ctx:  context.Background(),
				repo: "the-repo",
			},
			wantErr: true,
		},
		{
			name: "get-error",
			fields: fields{
				repositoriesService: &mockRepositoriesService{getError: errors.New("error")},
				limiter:             rate.NewLimiter(rate.Inf, 0),
			},
			args: args{
				ctx:  context.Background(),
				repo: "the-repo",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &GitHub{
				repositoriesService: tt.fields.repositoriesService,
				limiter:             tt.fields.limiter,
				owner:               tt.fields.owner,
			}
			got, err := g.Fetch(tt.args.ctx, tt.args.repo)
			if (err != nil) != tt.wantErr {
				t.Errorf("Fetch() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Fetch() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNew(t *testing.T) {
	repositoriesService := &mockRepositoriesService{}
	limiter := rate.NewLimiter(0, 0)
	owner := "the-owner"
	g := New(repositoriesService, limiter, owner)
	want := &GitHub{
		repositoriesService: repositoriesService,
		limiter:             limiter,
		owner:               owner,
	}

	if !reflect.DeepEqual(g, want) {
		t.Errorf("unexpected result")
	}
}
