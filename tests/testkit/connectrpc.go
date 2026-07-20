package testkit

import (
	"net/http"
	"net/http/httptest"
	"testing"

	runtimeprocess "ardents/internal/runtime/process"
	rpcadapter "ardents/internal/transport/connectrpc"
	"ardents/proto/ardents/v1/ardentsv1connect"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
)

func ConnectAuthConfig() rpcadapter.AuthConfig {
	return rpcadapter.AuthConfig{
		Token:        "test-token",
		SubjectID:    "connect-test",
		Capabilities: []string{"*"},
	}
}

func AuthorizedRequest[T any](msg *T) *connect.Request[T] {
	req := connect.NewRequest(msg)
	req.Header().Set("Authorization", "Bearer "+ConnectAuthConfig().Token)
	return req
}

func ConnectDependencies(runtime runtimeprocess.NodeRuntime) rpcadapter.Dependencies {
	return rpcadapter.Dependencies{
		Node:          runtime,
		Discovery:     runtime,
		Diagnostics:   runtime,
		Workload:      runtime,
		Hosting:       runtime,
		Data:          runtime,
		Configuration: runtime,
		Audit:         runtime,
	}
}

func NewArdentsClient(t *testing.T, runtime runtimeprocess.NodeRuntime) ardentsv1connect.ArdentsServiceClient {
	t.Helper()

	mux := http.NewServeMux()
	path, handler, err := rpcadapter.NewHandler(ConnectDependencies(runtime), ConnectAuthConfig())
	require.NoError(t, err)

	mux.Handle(path, handler)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return ardentsv1connect.NewArdentsServiceClient(srv.Client(), srv.URL, connect.WithGRPC())
}
