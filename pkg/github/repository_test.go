package github

import (
	"github.com/google/go-github/v52/github"
	"testing"
)

func TestRepository_RepoRoot(t *testing.T) {
	repo := &Repository{repository: &github.Repository{HTMLURL: github.String("https://example.com")}}

	if repo.GetRepoRoot() != "https://example.com" {
		t.Error("wrong repo root")
	}
}

func TestRepository_GetProjectWebsiteOrFallback(t *testing.T) {
	type fields struct {
		repository *github.Repository
	}
	type args struct {
		fallback string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   string
	}{
		{
			name:   "everything-given",
			fields: fields{repository: &github.Repository{Homepage: github.String("the-homepage")}},
			args:   args{fallback: "the-fallback"},
			want:   "the-homepage",
		},
		{
			name: "repository-nil",
			args: args{fallback: "the-fallback"},
			want: "the-fallback",
		},
		{
			name:   "homepage-empty",
			fields: fields{repository: &github.Repository{Homepage: github.String("")}},
			args:   args{fallback: "the-fallback"},
			want:   "the-fallback",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Repository{
				repository: tt.fields.repository,
			}
			if got := r.GetProjectWebsiteOrFallback(tt.args.fallback); got != tt.want {
				t.Errorf("GetProjectWebsiteOrFallback() = %v, want %v", got, tt.want)
			}
		})
	}
}
