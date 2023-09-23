package main

import (
	"context"
	"errors"
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
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"
	"time"
)

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

type AppContext struct {
	VCSHandler      VCSHandler
	ResponseBuilder ResponseBuilder
	Cache           Memoizer
	PackageHost     string
	ServerAddr      string
	MaxAge          time.Duration

	server *http.Server
}

func (a *AppContext) ListenAndServe() error {
	a.server = &http.Server{
		Addr:         a.ServerAddr,
		Handler:      a.getMux(),
		ReadTimeout:  6 * time.Second,
		WriteTimeout: 6 * time.Second,
	}

	return a.server.ListenAndServe()
}

func (a *AppContext) GracefulShutdown() {
	quit := make(chan os.Signal, 1)

	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	<-quit
	log.Print("shutting down")

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	if err := a.server.Shutdown(ctx); err != nil {
		log.Fatalf("forced shutdown: %s", err)
	}
}

func (a *AppContext) getMux() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", a.handleCacheControlHeader(a.handleRequest))
	mux.HandleFunc("/.internal/health", a.handleHealth)

	return mux
}

func (a *AppContext) buildResponse(response http.ResponseWriter, request *http.Request) error {
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

func (a *AppContext) handleRequest(response http.ResponseWriter, request *http.Request) {
	if err := a.buildResponse(response, request); err != nil {
		log.Print(err)

		if errors.Is(err, repository.ErrNotFound) {
			http.Error(response, "module not found", http.StatusNotFound)
			return
		}

		http.Error(response, "bad request", http.StatusBadRequest)
	}
}

func (a *AppContext) handleHealth(response http.ResponseWriter, _ *http.Request) {
	_, _ = fmt.Fprint(response, "ok")
}

func (a *AppContext) handleCacheControlHeader(handler http.HandlerFunc) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		handler(response, request)
		response.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%.f", a.MaxAge.Seconds()))
	}
}

func handleXCacheHeader(response http.ResponseWriter, cached bool) {
	value := "Miss"
	if cached {
		value = "Hit"
	}

	response.Header().Add("X-Cache", value)
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

	appContext := &AppContext{
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

	go func() {
		if err := appContext.ListenAndServe(); err != nil {
			log.Fatal(err)
		}
	}()

	appContext.GracefulShutdown()
}
