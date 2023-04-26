package main

import (
	"context"
	"flag"
	"fmt"
	githubClient "github.com/google/go-github/v52/github"
	"github.com/kofalt/go-memoize"
	"go.eigsys.de/masquerade/pkg/github"
	"go.eigsys.de/masquerade/pkg/goget"
	"go.eigsys.de/masquerade/pkg/repository"
	"golang.org/x/time/rate"
	"io"
	"log"
	"net/http"
	"path"
	"strings"
	"time"
)

func handleXCacheHeader(response http.ResponseWriter, cached bool) {
	value := "Miss"
	if cached {
		value = "Hit"
	}

	response.Header().Add("X-Cache", value)
}

func handleCacheControlHeader(response http.ResponseWriter, maxAge time.Duration) {
	response.Header().Add("Cache-Control", fmt.Sprintf("public, max-age=%.f", maxAge.Seconds()))
}

type VCSHandler interface {
	Type() string
	Fetch(ctx context.Context, repo string) (repository.Repository, error)
}

type ResponseBuilder interface {
	Build(writer io.Writer, data *goget.TemplateData) error
}

type Memoizer interface {
	Memoize(key string, fn func() (any, error)) (any, error, bool)
}

type appContext struct {
	VCSHandler      VCSHandler
	ResponseBuilder ResponseBuilder
	Cache           Memoizer
	PackageHost     string
	ServerAddr      string
	MaxAge          time.Duration
}

func (a *appContext) ListenAndServe() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", a.handleRequest)

	return http.ListenAndServe(a.ServerAddr, mux)
}

func (a *appContext) buildResponse(response http.ResponseWriter, request *http.Request) error {
	repo := strings.Split(request.URL.Path, "/")[1]

	vcsData, err, cached := a.Cache.Memoize(repo, func() (any, error) {
		return a.VCSHandler.Fetch(request.Context(), repo)
	})
	if err != nil {
		return err
	}

	handleXCacheHeader(response, cached)

	vcsRepository := vcsData.(repository.Repository)

	data := &goget.TemplateData{
		ImportPrefix:   path.Join(a.PackageHost, repo),
		VCS:            a.VCSHandler.Type(),
		RepoRoot:       vcsRepository.GetRepoRoot(),
		ProjectWebsite: vcsRepository.GetProjectWebsiteOrFallback(vcsRepository.GetRepoRoot()),
	}

	return a.ResponseBuilder.Build(response, data)
}

func (a *appContext) handleRequest(response http.ResponseWriter, request *http.Request) {
	handleCacheControlHeader(response, a.MaxAge)

	if err := a.buildResponse(response, request); err != nil {
		log.Print(err)
		http.NotFound(response, request)
	}
}

func main() {
	serverAddr := flag.String("serverAddr", ":8493", "HTTP listener address")
	packageHost := flag.String("packageHost", "", "Package host")
	ttl := flag.Duration("ttl", 1*time.Hour, "Cache TTL")
	githubOwner := flag.String("githubOwner", "", "GitHub owner")
	githubRequestRate := flag.Float64("githubRequestRate", 25, "Max. request rate to GitHub")
	githubBucketSize := flag.Int("githubBucketSize", 100, "Max. request bucket size for GitHub")
	flag.Parse()

	if *serverAddr == "" || *packageHost == "" || *githubOwner == "" {
		flag.Usage()
		log.Fatal("invalid flag")
	}

	appCtx := &appContext{
		VCSHandler: github.New(
			githubClient.NewClient(nil).Repositories,
			rate.NewLimiter(rate.Limit(*githubRequestRate), *githubBucketSize),
			*githubOwner,
		),
		ResponseBuilder: goget.New(),
		Cache:           memoize.NewMemoizer(*ttl, *ttl),
		PackageHost:     *packageHost,
		ServerAddr:      *serverAddr,
		MaxAge:          *ttl,
	}

	log.Fatal(appCtx.ListenAndServe())
}
