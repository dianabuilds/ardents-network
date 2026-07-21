package docker

import (
	"ardents/internal/workload/execution"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/moby/moby/client"
	"github.com/stretchr/testify/require"
)

func TestStartNewRemovesCreatedContainerAfterStartFailure(t *testing.T) {
	removed := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/containers/create"):
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"Id": "created-1", "Warnings": []string{}}))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/containers/created-1/start"):
			http.Error(w, `{"message":"secret daemon start failure"}`, http.StatusInternalServerError)
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/containers/created-1/stop"):
			w.WriteHeader(http.StatusNotModified)
		case r.Method == http.MethodDelete && strings.HasSuffix(r.URL.Path, "/containers/created-1"):
			removed = true
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, `{"message":"unexpected Docker request"}`, http.StatusNotFound)
		}
	}))
	defer server.Close()

	engine, err := client.New(client.WithHost(server.URL), client.WithHTTPClient(server.Client()), client.WithAPIVersion("1.44"))
	require.NoError(t, err)
	executor := &Executor{client: engine, nodeID: "node-1", stopTimeout: time.Second}

	_, err = executor.startNew(t.Context(), containerSpec{Image: testDigestImage}, execution.PreparedWorkload{WorkloadID: "workload-1", Generation: 1})

	require.ErrorContains(t, err, "start workload container")
	require.NotContains(t, err.Error(), "secret daemon start failure")
	require.True(t, removed, "created container was not removed")
}
