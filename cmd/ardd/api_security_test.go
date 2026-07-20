package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestValidateLocalAPIListenAddrAllowsLoopbackOnly(t *testing.T) {
	for _, addr := range []string{"127.0.0.1:8080", "localhost:8080", "[::1]:8080"} {
		require.NoError(t, validateLocalAPIListenAddr(addr), addr)
	}
	for _, addr := range []string{"0.0.0.0:8080", "192.0.2.1:8080", ":8080", "api.internal:8080"} {
		err := validateLocalAPIListenAddr(addr)
		require.Error(t, err, addr)
		require.Contains(t, err.Error(), "loopback")
	}
}

func TestLocalAdminCapabilitiesAreExplicit(t *testing.T) {
	capabilities := localAdminCapabilities()
	require.NotEmpty(t, capabilities)
	require.NotContains(t, capabilities, "*")
	require.Contains(t, capabilities, "node.start")
	require.Contains(t, capabilities, "diagnostics.snapshot")
	require.Contains(t, capabilities, "transport.network_status")
}

func TestNewHTTPServerSetsBoundaryTimeouts(t *testing.T) {
	server := newHTTPServer("127.0.0.1:0", http.NotFoundHandler())
	require.Positive(t, server.ReadHeaderTimeout)
	require.Positive(t, server.ReadTimeout)
	require.Positive(t, server.WriteTimeout)
	require.Positive(t, server.IdleTimeout)
	require.Positive(t, server.MaxHeaderBytes)
}

func TestLocalAPIHandlerRejectsOversizedBody(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusNoContent)
	})
	handler := limitLocalAPIHandler(inner, 8, time.Second)
	req := httptest.NewRequest(http.MethodPost, "/rpc", strings.NewReader("0123456789"))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusRequestEntityTooLarge, recorder.Code)
}

func TestLocalAPIHandlerTimesOutWithoutLeakingRequestSecrets(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	})
	handler := limitLocalAPIHandler(inner, 1024, 10*time.Millisecond)
	req := httptest.NewRequest(http.MethodPost, "/rpc", strings.NewReader("secret-token"))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "secret-token")
}

func TestLocalAPIStreamIsNotGivenUnaryDeadline(t *testing.T) {
	deadlineObserved := true
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, deadlineObserved = r.Context().Deadline()
		w.WriteHeader(http.StatusNoContent)
	})
	handler := limitLocalAPIHandler(inner, 1024, time.Second)
	req := httptest.NewRequest(http.MethodPost, "/ardents.v1.ArdentsService/StreamNodeEvents", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusNoContent, recorder.Code)
	require.False(t, deadlineObserved)
}
