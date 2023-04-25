package main

import (
	"bytes"
	"context"
	"errors"
	"go.eigsys.de/masquerade/pkg/goget"
	"go.eigsys.de/masquerade/pkg/repository"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type mockRepository struct {
	RepoRootResult       string
	ProjectWebsiteResult string
}

func (m *mockRepository) RepoRoot() string {
	return m.RepoRootResult
}

func (m *mockRepository) ProjectWebsite() string {
	return m.ProjectWebsiteResult
}

type mockVCSHandler struct {
	typeResult  string
	fetchResult repository.Repository
	fetchErr    error
}

func (m *mockVCSHandler) Type() string {
	return m.typeResult
}

func (m *mockVCSHandler) Fetch(_ context.Context, _ string) (repository.Repository, error) {
	return m.fetchResult, m.fetchErr
}

type mockResponseBuilder struct {
	buildBytes []byte
	buildErr   error
}

func (m *mockResponseBuilder) Build(writer io.Writer, _ *goget.TemplateData) error {
	_, _ = writer.Write(m.buildBytes)
	return m.buildErr
}

type mockMemoizer struct {
	memoizeResult any
	memoizeErr    error
	memoizeCached bool
}

func (m *mockMemoizer) Memoize(_ string, _ func() (any, error)) (any, error, bool) {
	return m.memoizeResult, m.memoizeErr, m.memoizeCached
}

func Test_handleXCacheHeader(t *testing.T) {
	type args struct {
		response http.ResponseWriter
		cached   bool
		want     string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "miss",
			args: args{
				response: httptest.NewRecorder(),
				cached:   false,
			},
			want: "Miss",
		},
		{
			name: "hit",
			args: args{
				response: httptest.NewRecorder(),
				cached:   true,
			},
			want: "Hit",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handleXCacheHeader(tt.args.response, tt.args.cached)

			if tt.args.response.Header().Get("X-Cache") != tt.want {
				t.Error("wrong header value")
			}
		})
	}
}

func Test_handleCacheControlHeader(t *testing.T) {
	response := httptest.NewRecorder()

	handleCacheControlHeader(response, 2*time.Hour)

	if response.Header().Get("Cache-Control") != "public, max-age=7200" {
		t.Error("wrong header value")
	}
}

func Test_appContext_buildResponse(t *testing.T) {
	type fields struct {
		VCSHandler         VCSHandler
		ResponseBuilder    ResponseBuilder
		Cache              Memoizer
		PackageHost        string
		ListenAndServeAddr string
		MaxAge             time.Duration
	}
	type args struct {
		response http.ResponseWriter
		request  *http.Request
	}
	tests := []struct {
		name         string
		fields       fields
		args         args
		wantErr      bool
		wantResponse []byte
		wantHeaders  http.Header
	}{
		{
			name: "user",
			fields: fields{
				VCSHandler:      &mockVCSHandler{},
				ResponseBuilder: &mockResponseBuilder{buildBytes: []byte("the-response")},
				Cache:           &mockMemoizer{memoizeResult: &mockRepository{}, memoizeCached: true},
			},
			args: args{
				response: httptest.NewRecorder(),
				request:  httptest.NewRequest(http.MethodGet, "/foo", nil),
			},
			wantErr:      false,
			wantResponse: []byte("the-response"),
			wantHeaders:  map[string][]string{"X-Cache": {"Hit"}},
		},
		{
			name: "go-get",
			fields: fields{
				VCSHandler:      &mockVCSHandler{},
				ResponseBuilder: &mockResponseBuilder{buildBytes: []byte("the-response")},
				Cache:           &mockMemoizer{memoizeResult: &mockRepository{}, memoizeCached: true},
			},
			args: args{
				response: httptest.NewRecorder(),
				request:  httptest.NewRequest(http.MethodGet, "/foo?go-get=1", nil),
			},
			wantErr:      false,
			wantResponse: []byte("the-response"),
			wantHeaders:  map[string][]string{"X-Cache": {"Hit"}},
		},
		{
			name: "memoizer-error",
			fields: fields{
				VCSHandler: &mockVCSHandler{},
				Cache:      &mockMemoizer{memoizeErr: errors.New("error")},
			},
			args: args{
				response: httptest.NewRecorder(),
				request:  httptest.NewRequest(http.MethodGet, "/foo", nil),
			},
			wantErr: true,
		},
		{
			name: "response-builder-error",
			fields: fields{
				VCSHandler:      &mockVCSHandler{},
				ResponseBuilder: &mockResponseBuilder{buildErr: errors.New("error")},
				Cache:           &mockMemoizer{memoizeResult: &mockRepository{}},
			},
			args: args{
				response: httptest.NewRecorder(),
				request:  httptest.NewRequest(http.MethodGet, "/foo", nil),
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &appContext{
				VCSHandler:         tt.fields.VCSHandler,
				ResponseBuilder:    tt.fields.ResponseBuilder,
				Cache:              tt.fields.Cache,
				PackageHost:        tt.fields.PackageHost,
				ListenAndServeAddr: tt.fields.ListenAndServeAddr,
				MaxAge:             tt.fields.MaxAge,
			}
			if err := a.buildResponse(tt.args.response, tt.args.request); (err != nil) != tt.wantErr {
				t.Errorf("buildResponse() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !bytes.Equal(tt.args.response.(*httptest.ResponseRecorder).Body.Bytes(), tt.wantResponse) {
				t.Error("wrong response body")
			}
			for name, value := range tt.wantHeaders {
				if tt.args.response.(*httptest.ResponseRecorder).Header().Get(name) != value[0] {
					t.Errorf("wrong header = %v", name)
				}
			}
		})
	}
}
