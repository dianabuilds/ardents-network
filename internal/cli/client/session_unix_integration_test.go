package client

import (
	"context"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	ardentsv1 "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/protocol/ardentsv1connect"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
)

type unixIdentityHandler struct {
	ardentsv1connect.UnimplementedIdentityServiceHandler
	auth *sessionTestAuth
}

func (h unixIdentityHandler) BeginAuthentication(ctx context.Context, request *connect.Request[ardentsv1.BeginAuthenticationRequest]) (*connect.Response[ardentsv1.BeginAuthenticationResponse], error) {
	return h.auth.BeginAuthentication(ctx, request)
}

func (h unixIdentityHandler) CompleteAuthentication(ctx context.Context, request *connect.Request[ardentsv1.CompleteAuthenticationRequest]) (*connect.Response[ardentsv1.CompleteAuthenticationResponse], error) {
	return h.auth.CompleteAuthentication(ctx, request)
}

type unixNodeHandler struct {
	ardentsv1connect.UnimplementedNodeServiceHandler
	mu             sync.Mutex
	headers        []string
	streamHeaders  []string
	startAttempts  int
	startMutations int
}

func (h *unixNodeHandler) StartNode(_ context.Context, request *connect.Request[ardentsv1.StartNodeRequest]) (*connect.Response[ardentsv1.CommandAckResponse], error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.startAttempts++
	if h.startAttempts == 1 {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}
	h.startMutations++
	return connect.NewResponse(&ardentsv1.CommandAckResponse{}), nil
}

func (h *unixNodeHandler) StreamNodeEvents(_ context.Context, request *connect.Request[ardentsv1.StreamNodeEventsRequest], stream *connect.ServerStream[ardentsv1.EventEnvelope]) error {
	h.mu.Lock()
	h.streamHeaders = append(h.streamHeaders, request.Header().Get("Authorization"))
	count := len(h.streamHeaders)
	h.mu.Unlock()
	if count == 1 {
		return connect.NewError(connect.CodeUnauthenticated, nil)
	}
	return stream.Send(&ardentsv1.EventEnvelope{})
}

func (h *unixNodeHandler) GetNodeStatus(_ context.Context, request *connect.Request[ardentsv1.GetNodeStatusRequest]) (*connect.Response[ardentsv1.NodeStatusResponse], error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.headers = append(h.headers, request.Header().Get("Authorization"))
	if len(h.headers) == 1 {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}
	return connect.NewResponse(&ardentsv1.NodeStatusResponse{}), nil
}

func TestPrincipalServerStreamReauthenticatesOnceBeforeFirstEvent(t *testing.T) {
	dir, err := os.MkdirTemp("", "ardents-cli-")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, os.RemoveAll(dir)) })
	socket := filepath.Join(dir, "operator.sock")
	listener, err := net.Listen("unix", socket)
	require.NoError(t, err)

	signer := newSessionTestSigner(t)
	node := sessionTestPrincipal(t, 0x31)
	auth := &sessionTestAuth{node: node, principal: signer.principal, now: time.Now().UTC().Truncate(time.Second), secretByte: 0x50}
	nodeHandler := &unixNodeHandler{}
	mux := http.NewServeMux()
	identityPath, identityHTTP := ardentsv1connect.NewIdentityServiceHandler(unixIdentityHandler{auth: auth})
	nodePath, nodeHTTP := ardentsv1connect.NewNodeServiceHandler(nodeHandler)
	mux.Handle(identityPath, identityHTTP)
	mux.Handle(nodePath, nodeHTTP)
	server := &http.Server{Handler: mux}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { require.NoError(t, server.Close()) })

	client := New(Config{BaseURL: "unix://" + filepath.ToSlash(socket), Timeout: 5 * time.Second, ExpectedPrincipal: node, Signer: signer})
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	stream, err := client.Service().StreamNodeEvents(ctx, Request(&ardentsv1.StreamNodeEventsRequest{}))
	require.NoError(t, err)
	require.True(t, stream.Receive())
	require.NoError(t, stream.Err())
	require.Len(t, nodeHandler.streamHeaders, 2)
	require.NotEqual(t, nodeHandler.streamHeaders[0], nodeHandler.streamHeaders[1])
	require.EqualValues(t, 2, auth.beginCount.Load())
}

func TestPrincipalUnauthenticatedMutatingCallMutatesExactlyOnceAfterReplay(t *testing.T) {
	dir, err := os.MkdirTemp("", "ardents-cli-")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, os.RemoveAll(dir)) })
	socket := filepath.Join(dir, "operator.sock")
	listener, err := net.Listen("unix", socket)
	require.NoError(t, err)

	signer := newSessionTestSigner(t)
	node := sessionTestPrincipal(t, 0x31)
	auth := &sessionTestAuth{node: node, principal: signer.principal, now: time.Now().UTC().Truncate(time.Second), secretByte: 0x60}
	nodeHandler := &unixNodeHandler{}
	mux := http.NewServeMux()
	identityPath, identityHTTP := ardentsv1connect.NewIdentityServiceHandler(unixIdentityHandler{auth: auth})
	nodePath, nodeHTTP := ardentsv1connect.NewNodeServiceHandler(nodeHandler)
	mux.Handle(identityPath, identityHTTP)
	mux.Handle(nodePath, nodeHTTP)
	server := &http.Server{Handler: mux}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { require.NoError(t, server.Close()) })

	client := New(Config{BaseURL: "unix://" + filepath.ToSlash(socket), Timeout: 5 * time.Second, ExpectedPrincipal: node, Signer: signer})
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = client.Service().StartNode(ctx, Request(&ardentsv1.StartNodeRequest{}))
	require.NoError(t, err)
	require.Equal(t, 2, nodeHandler.startAttempts)
	require.Equal(t, 1, nodeHandler.startMutations)
	require.EqualValues(t, 2, auth.beginCount.Load())
}

func TestPrincipalSessionOverUnixAuthenticatesAndRetriesOnce(t *testing.T) {
	dir, err := os.MkdirTemp("", "ardents-cli-")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, os.RemoveAll(dir)) })
	socket := filepath.Join(dir, "operator.sock")
	listener, err := net.Listen("unix", socket)
	require.NoError(t, err)

	signer := newSessionTestSigner(t)
	node := sessionTestPrincipal(t, 0x31)
	auth := &sessionTestAuth{node: node, principal: signer.principal, now: time.Now().UTC().Truncate(time.Second), secretByte: 0x20}
	nodeHandler := &unixNodeHandler{}
	mux := http.NewServeMux()
	identityPath, identityHTTP := ardentsv1connect.NewIdentityServiceHandler(unixIdentityHandler{auth: auth})
	nodePath, nodeHTTP := ardentsv1connect.NewNodeServiceHandler(nodeHandler)
	mux.Handle(identityPath, identityHTTP)
	mux.Handle(nodePath, nodeHTTP)
	server := &http.Server{Handler: mux}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { require.NoError(t, server.Close()) })

	client := New(Config{BaseURL: "unix://" + filepath.ToSlash(socket), Timeout: time.Second, ExpectedPrincipal: node, Signer: signer})
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = client.Service().GetNodeStatus(ctx, Request(&ardentsv1.GetNodeStatusRequest{}))
	require.NoError(t, err)
	require.EqualValues(t, 2, auth.beginCount.Load())
	require.EqualValues(t, 2, auth.completeCount.Load())
	require.Len(t, nodeHandler.headers, 2)
	require.NotEqual(t, nodeHandler.headers[0], nodeHandler.headers[1])
	for _, header := range nodeHandler.headers {
		require.Contains(t, header, operatorSessionScheme+" ")
		require.NotContains(t, header, "Bearer")
	}
}
