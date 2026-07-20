package observability

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const correlationHeader = "Ardents-Correlation-ID"

var validCorrelationID = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

type requestMetrics struct {
	count    *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

func newRequestMetrics() *requestMetrics {
	return &requestMetrics{
		count:    prometheus.NewCounterVec(prometheus.CounterOpts{Namespace: "ardents", Name: "http_requests_total", Help: "HTTP requests by normalized route, method, and status class."}, []string{"route", "method", "status"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{Namespace: "ardents", Name: "http_request_duration_seconds", Help: "HTTP request duration by normalized route and method.", Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 15, 30}}, []string{"route", "method"}),
	}
}

func (s *Surface) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		correlationID, err := requestCorrelationID(r.Header.Get(correlationHeader))
		if err != nil {
			http.Error(w, "correlation unavailable", http.StatusInternalServerError)
			return
		}
		started := time.Now()
		route := normalizedRoute(r.URL.Path)
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		recorder.Header().Set(correlationHeader, correlationID)
		next.ServeHTTP(recorder, r)
		duration := time.Since(started)
		method := normalizedMethod(r.Method)
		status := strconv.Itoa(recorder.status / 100)
		s.requests.count.WithLabelValues(route, method, status).Inc()
		s.requests.duration.WithLabelValues(route, method).Observe(duration.Seconds())
		slog.Info("http_request_completed", "correlation_id", correlationID, "route", route,
			"method", method, "status", recorder.status, "duration_ms", duration.Milliseconds())
	})
}

func requestCorrelationID(candidate string) (string, error) {
	if validCorrelationID.MatchString(candidate) {
		return candidate, nil
	}
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

func normalizedRoute(path string) string {
	switch path {
	case "/healthz", "/readyz", "/metrics":
		return path
	}
	if strings.HasPrefix(path, "/ardents.v1.ArdentsService/") {
		return "connect_rpc"
	}
	return "unknown"
}

func normalizedMethod(method string) string {
	switch strings.ToUpper(method) {
	case "GET", "POST", "HEAD":
		return strings.ToUpper(method)
	default:
		return "OTHER"
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (w *statusRecorder) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusRecorder) Write(data []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(data)
}

func (w *statusRecorder) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
