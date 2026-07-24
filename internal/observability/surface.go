package observability

import (
	"ardents/internal/buildinfo"
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Surface struct {
	runtime  RuntimeSource
	token    string
	registry *prometheus.Registry
	requests *requestMetrics
}

func NewSurface(deps Dependencies, token string) (*Surface, error) {
	registry := prometheus.NewRegistry()
	requests := newRequestMetrics()
	for _, collector := range []prometheus.Collector{
		NewCollector(deps), collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		requests.count, requests.duration,
	} {
		if err := registry.Register(collector); err != nil {
			return nil, err
		}
	}
	return &Surface{runtime: deps.Runtime, token: strings.TrimSpace(token), registry: registry, requests: requests}, nil
}

func (s *Surface) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/metrics", s.metricsHandler())
	mux.HandleFunc("/healthz", s.health)
	mux.HandleFunc("/readyz", s.ready)
	return s.Middleware(mux)
}

func (s *Surface) metricsHandler() http.Handler {
	handler := promhttp.HandlerFor(s.registry, promhttp.HandlerOpts{EnableOpenMetrics: true})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.token != "" && !matchesBearer(r.Header.Get("Authorization"), s.token) {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		handler.ServeHTTP(w, r)
	})
}

func matchesBearer(header, token string) bool {
	want := "Bearer " + token
	if len(header) != len(want) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(header), []byte(want)) == 1
}

func (s *Surface) health(w http.ResponseWriter, _ *http.Request) {
	writeProbe(w, http.StatusOK, probeResponse{Status: "alive"})
}

func (s *Surface) ready(w http.ResponseWriter, _ *http.Request) {
	snapshot := s.runtime.GetNodeRuntime()
	state := lifecycleState(snapshot.Node.State)
	health := healthState(snapshot.Health.State)
	status := http.StatusServiceUnavailable
	result := "not_ready"
	if snapshot.Readiness.Ready {
		status = http.StatusOK
		result = "ready"
	}
	writeProbe(w, status, probeResponse{
		Status: result, State: state, Health: health,
		Node: snapshot.Node.Name, Principal: snapshot.Identity.Principal,
		BuildIdentity: buildinfo.Fingerprint(),
	})
}

type probeResponse struct {
	Status        string `json:"status"`
	State         string `json:"state,omitempty"`
	Health        string `json:"health,omitempty"`
	Node          string `json:"node,omitempty"`
	Principal     string `json:"principal,omitempty"`
	BuildIdentity string `json:"build_identity,omitempty"`
}

func writeProbe(w http.ResponseWriter, status int, response probeResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.Error("write probe response", "error", err)
	}
}
