package github

import (
	"testing"
)

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
