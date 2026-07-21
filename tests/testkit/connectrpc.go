package testkit

import (
	"net/http"
	"net/http/httptest"
	"testing"

	cliclient "ardents/internal/cli/client"
	runtimeprocess "ardents/internal/daemon"
	rpcadapter "ardents/internal/localapi"
	localauth "ardents/internal/localapi/auth"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
)

func ConnectAuthConfig() localauth.Config {
	return localauth.Config{
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

func ConnectDependencies(runtime *runtimeprocess.Node) rpcadapter.Dependencies {
	owners, ok := runtimeprocess.OwnersFor(runtime)
	if !ok {
		return rpcadapter.Dependencies{}
	}
	return rpcadapter.Dependencies{
		Node:             runtime,
		Discovery:        runtime,
		DiscoveryRecords: owners.DiscoveryCommands,
		Network:          runtime,
		Diagnostics:      owners.Diagnostics,
		Workload:         owners.Workloads,
		Hosting:          owners.Hosting,
		Content:          owners.Content,
		Sources:          owners.Content,
		Transfers:        owners.Transfers,
		Data:             owners.ContentCommands,
		DataFetch:        runtime,
		Configuration:    runtime,
		Audit:            owners.Events,
	}
}

func NewArdentsClient(t *testing.T, runtime *runtimeprocess.Node) cliclient.Service {
	t.Helper()

	mux := http.NewServeMux()
	path, handler, err := rpcadapter.NewHandler(ConnectDependencies(runtime), ConnectAuthConfig())
	require.NoError(t, err)

	mux.Handle(path, handler)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return cliclient.NewService(srv.Client(), srv.URL, connect.WithGRPC())
}
