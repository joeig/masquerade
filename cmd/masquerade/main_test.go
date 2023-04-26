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
	"reflect"
	"testing"
	"time"
)

type mockRepository struct {
	RepoRootResult       string
	ProjectWebsiteResult string
}

func (m *mockRepository) GetRepoRoot() string {
	return m.RepoRootResult
}

func (m *mockRepository) GetProjectWebsiteOrFallback(_ string) string {
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
		name        string
		fields      fields
		args        args
		wantErr     bool
		wantHeaders http.Header
		wantBody    []byte
	}{
		{
			name: "user",
			fields: fields{
				VCSHandler:      &mockVCSHandler{},
				ResponseBuilder: &mockResponseBuilder{buildBytes: []byte("<head>")},
				Cache:           &mockMemoizer{memoizeResult: &mockRepository{}, memoizeCached: true},
			},
			args: args{
				response: httptest.NewRecorder(),
				request:  httptest.NewRequest(http.MethodGet, "/foo", nil),
			},
			wantErr:     false,
			wantBody:    []byte("<head>"),
			wantHeaders: map[string][]string{"X-Cache": {"Hit"}, "Content-Type": {"text/html; charset=utf-8"}},
		},
		{
			name: "go-get",
			fields: fields{
				VCSHandler:      &mockVCSHandler{},
				ResponseBuilder: &mockResponseBuilder{buildBytes: []byte("<head>")},
				Cache:           &mockMemoizer{memoizeResult: &mockRepository{}, memoizeCached: true},
			},
			args: args{
				response: httptest.NewRecorder(),
				request:  httptest.NewRequest(http.MethodGet, "/foo?go-get=1", nil),
			},
			wantErr:     false,
			wantBody:    []byte("<head>"),
			wantHeaders: map[string][]string{"X-Cache": {"Hit"}, "Content-Type": {"text/html; charset=utf-8"}},
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
			wantErr:     true,
			wantHeaders: map[string][]string{},
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
			wantErr:     true,
			wantHeaders: map[string][]string{"X-Cache": {"Miss"}, "Content-Type": {"text/plain; charset=utf-8"}}},
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
			response := tt.args.response.(*httptest.ResponseRecorder)
			if response.Code != 200 {
				t.Error("invalid code")
			}
			if !reflect.DeepEqual(tt.args.response.Header(), tt.wantHeaders) {
				t.Error("invalid header")
			}
			if !bytes.Equal(response.Body.Bytes(), tt.wantBody) {
				t.Error("invalid body")
			}
		})
	}
}

func Test_appContext_handleRequest(t *testing.T) {
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
		name        string
		fields      fields
		args        args
		wantCode    int
		wantHeaders http.Header
		wantBody    []byte
	}{
		{
			name: "ok",
			fields: fields{
				VCSHandler:      &mockVCSHandler{},
				ResponseBuilder: &mockResponseBuilder{buildBytes: []byte("<head>")},
				Cache:           &mockMemoizer{memoizeResult: &mockRepository{}, memoizeCached: true},
				MaxAge:          30 * time.Second,
			},
			args: args{
				response: httptest.NewRecorder(),
				request:  httptest.NewRequest(http.MethodGet, "/foo", nil),
			},
			wantCode: 200,
			wantHeaders: map[string][]string{
				"Cache-Control": {"public, max-age=30"},
				"X-Cache":       {"Hit"},
				"Content-Type":  {"text/html; charset=utf-8"},
			},
			wantBody: []byte("<head>"),
		},
		{
			name: "build-response-error",
			fields: fields{
				VCSHandler:      &mockVCSHandler{},
				ResponseBuilder: &mockResponseBuilder{},
				Cache:           &mockMemoizer{memoizeErr: errors.New("error"), memoizeCached: true},
				MaxAge:          30 * time.Second,
			},
			args: args{
				response: httptest.NewRecorder(),
				request:  httptest.NewRequest(http.MethodGet, "/foo", nil),
			},
			wantCode: 404,
			wantHeaders: map[string][]string{
				"Cache-Control":          {"public, max-age=30"},
				"Content-Type":           {"text/plain; charset=utf-8"},
				"X-Content-Type-Options": {"nosniff"},
			},
			wantBody: []byte("404 page not found\n"),
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
			a.handleRequest(tt.args.response, tt.args.request)
			response := tt.args.response.(*httptest.ResponseRecorder)
			if response.Code != tt.wantCode {
				t.Error("invalid code")
			}
			if !reflect.DeepEqual(tt.args.response.Header(), tt.wantHeaders) {
				t.Error("invalid header")
			}
			if !bytes.Equal(response.Body.Bytes(), tt.wantBody) {
				t.Error("invalid body")
			}
		})
	}
}
