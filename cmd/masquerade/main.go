package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	githubClient "github.com/google/go-github/v52/github"
	"github.com/kofalt/go-memoize"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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

const moduleLabel = "module"

type Metrics struct {
	HTTPRequestsTotal *prometheus.CounterVec
	ModuleNotFound    prometheus.Counter

	enabled    bool
	registerer prometheus.Registerer
	gatherer   prometheus.Gatherer
	server     *http.Server
}

func NewMetrics(enabled bool, registerer prometheus.Registerer, gatherer prometheus.Gatherer) *Metrics {
	metrics := &Metrics{
		HTTPRequestsTotal: prometheus.V2.NewCounterVec(
			prometheus.CounterVecOpts{
				CounterOpts: prometheus.CounterOpts{
					Name: "http_requests_total",
					Help: "Total number of HTTP requests",
				},
				VariableLabels: prometheus.ConstrainedLabels{
					prometheus.ConstrainedLabel{
						Name: moduleLabel,
						Constraint: func(s string) string {
							return strings.ToLower(s)
						},
					},
				},
			},
		),
		ModuleNotFound: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "module_not_found_total",
				Help: "Total number of module not found responses",
			}),
		enabled:    enabled,
		registerer: registerer,
		gatherer:   gatherer,
	}

	registerer.MustRegister(metrics.HTTPRequestsTotal, metrics.ModuleNotFound)

	return metrics
}

func (m *Metrics) ListenAndServe() error {
	if !m.enabled {
		return nil
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(m.gatherer, promhttp.HandlerOpts{Registry: m.registerer}))

	m.server = &http.Server{
		Addr:         ":9091",
		Handler:      mux,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	}

	log.Printf("exporting metrics on %q", m.server.Addr)

	return m.server.ListenAndServe()
}

type AppContext struct {
	Metrics         *Metrics
	VCSHandler      VCSHandler
	ResponseBuilder ResponseBuilder
	Cache           Memoizer
	PackageHost     string
	ServerAddr      string
	MaxAge          time.Duration
	HomePageURL     string

	server *http.Server
}

func (a *AppContext) ListenAndServe() error {
	go func() {
		if err := a.Metrics.ListenAndServe(); err != nil {
			log.Fatal(err)
		}
	}()

	a.server = &http.Server{
		Addr:         a.ServerAddr,
		Handler:      a.getMux(),
		ReadTimeout:  6 * time.Second,
		WriteTimeout: 6 * time.Second,
	}

	log.Printf("listening on %q", a.server.Addr)

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

	if repo == "" && a.HomePageURL != "" {
		http.Redirect(response, request, a.HomePageURL, http.StatusSeeOther)
		return nil
	}

	vcsData, err, cached := a.Cache.Memoize(repo, func() (any, error) {
		return a.VCSHandler.Fetch(request.Context(), repo)
	})
	if err != nil {
		return err
	}

	handleXCacheHeader(response, cached)
	a.Metrics.HTTPRequestsTotal.With(prometheus.Labels{moduleLabel: repo}).Inc()

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
			a.Metrics.ModuleNotFound.Inc()
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
	homePageURL := flag.String("homePageURL", "", "Home page URL (requesting \"/\") redirects to this URL")
	githubOwner := flag.String("githubOwner", "", "GitHub owner")
	githubRequestRate := flag.Float64("githubRequestRate", 25, "Max. request rate to GitHub")
	githubBucketSize := flag.Int("githubBucketSize", 100, "Max. request bucket size for GitHub")
	enableMetrics := flag.Bool("enableMetrics", false, "Enable Prometheus metrics on \":9091/metrics\"")
	flag.Parse()

	if *serverAddr == "" || *packageHost == "" || *githubOwner == "" {
		flag.Usage()
		log.Fatal("invalid flag")
	}

	registry := prometheus.NewRegistry()

	appContext := &AppContext{
		Metrics: NewMetrics(*enableMetrics, registry, registry),
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
		HomePageURL:     *homePageURL,
	}

	go func() {
		log.Fatal(appContext.ListenAndServe())
	}()

	appContext.GracefulShutdown()
}
