package content_test

import (
	applicationcall "ardents/internal/applicationapi/call"
	applicationcontent "ardents/internal/applicationapi/content"
	appcontent "ardents/internal/content"
	contentpayload "ardents/internal/content/payload"
	"ardents/sdk/go/client"
	"ardents/sdk/go/content"
	sdkerrors "ardents/sdk/go/errors"
	applicationv1 "ardents/sdk/go/protocol/applicationv1"
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
)

func TestContentPutGetCrossesPublicApplicationContract(t *testing.T) {
	domainStore := appcontent.NewInDir(t.TempDir())
	require.NoError(t, domainStore.Load())
	store := &contentOwner{store: domainStore, commands: appcontent.NewCommands(domainStore, appcontent.CommandConfig{})}
	path, handler := newPrincipalHTTPHandler(t, store)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	app, err := client.New(client.Config{
		Endpoint: server.URL, Credential: client.StaticCredential("application-secret"), HTTPClient: server.Client(),
	})
	require.NoError(t, err)
	reference, err := app.Content.Put(context.Background(), []byte("hello"), content.WithMediaType("text/plain"))
	require.NoError(t, err)
	require.Equal(t, "blob", reference.Kind)
	require.NotEmpty(t, reference.ID)
	require.Equal(t, reference.ID, storeCID(t, domainStore, reference.ID))

	payload, err := app.Content.Get(context.Background(), reference)
	require.NoError(t, err)
	require.Equal(t, []byte("hello"), payload)
	require.Equal(t, []string{applicationcontent.ActionPut, applicationcontent.ActionGet}, store.actions)
}

func storeCID(t *testing.T, store *appcontent.Service, id string) string {
	t.Helper()
	blob, ok := store.GetBlob(id)
	require.True(t, ok)
	return blob.CID
}

func TestContentClientMapsPublicStructuredError(t *testing.T) {
	store := &memoryStore{payloads: map[string][]byte{}, blobs: map[string]appcontent.Blob{}}
	path, handler := newPrincipalHTTPHandler(t, store)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	app, err := client.New(client.Config{
		Endpoint: server.URL, Credential: client.StaticCredential("application-secret"), HTTPClient: server.Client(),
	})
	require.NoError(t, err)

	_, err = app.Content.Get(context.Background(), content.Reference{Kind: "blob", ID: "missing"})
	var sdkErr *sdkerrors.Error
	require.ErrorAs(t, err, &sdkErr)
	require.Equal(t, sdkerrors.NotFound, sdkErr.Code)
	require.Equal(t, applicationcontent.ActionGet, sdkErr.Operation)
	require.False(t, sdkErr.Retryable)
}

func TestClientRejectsRemotePlaintextEndpoint(t *testing.T) {
	_, err := client.New(client.Config{
		Endpoint: "http://node.example:8080", Credential: client.StaticCredential("application-secret"),
	})
	require.ErrorContains(t, err, "must be loopback")
}

func TestApplicationClientUsesUnixSocket(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix sockets are unavailable on Windows")
	}
	store := &memoryStore{payloads: map[string][]byte{}, blobs: map[string]appcontent.Blob{}}
	path, handler := newPrincipalHTTPHandler(t, store)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	socketPath := filepath.Join(t.TempDir(), "application.sock")
	listener, err := net.Listen("unix", socketPath)
	require.NoError(t, err)
	server := &http.Server{Handler: mux}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() {
		_ = server.Close()
		_ = os.Remove(socketPath)
	})

	application, err := client.New(client.Config{
		SocketPath: socketPath, Credential: client.StaticCredential("application-secret"),
	})
	require.NoError(t, err)
	reference, err := application.Content.Put(context.Background(), []byte("over unix"))
	require.NoError(t, err)
	payload, err := application.Content.Get(context.Background(), reference)
	require.NoError(t, err)
	require.Equal(t, []byte("over unix"), payload)
}

func TestContentGetRejectsSameLengthPayloadTampering(t *testing.T) {
	store := &memoryStore{payloads: map[string][]byte{}, blobs: map[string]appcontent.Blob{}}
	path, handler := newPrincipalHTTPHandler(t, store)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	app, err := client.New(client.Config{
		Endpoint: server.URL, Credential: client.StaticCredential("application-secret"), HTTPClient: server.Client(),
	})
	require.NoError(t, err)
	reference, err := app.Content.Put(context.Background(), []byte("hello"))
	require.NoError(t, err)
	store.payloads[reference.ID] = []byte("jello")

	_, err = app.Content.Get(context.Background(), reference)
	var sdkErr *sdkerrors.Error
	require.ErrorAs(t, err, &sdkErr)
	require.Equal(t, sdkerrors.IntegrityFailed, sdkErr.Code)
}

func TestContentPutRejectsPayloadAboveUnaryLimit(t *testing.T) {
	store := &memoryStore{payloads: map[string][]byte{}, blobs: map[string]appcontent.Blob{}}
	path, handler := newPrincipalHTTPHandler(t, store)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	app, err := client.New(client.Config{
		Endpoint: server.URL, Credential: client.StaticCredential("application-secret"), HTTPClient: server.Client(),
	})
	require.NoError(t, err)

	_, err = app.Content.Put(context.Background(), make([]byte, applicationv1.MaxUnaryPayloadBytes+1))
	var sdkErr *sdkerrors.Error
	require.ErrorAs(t, err, &sdkErr)
	require.Equal(t, sdkerrors.ResourceExhausted, sdkErr.Code)
	require.Empty(t, store.blobs)
}

func TestContentGetRejectsOversizedBlobBeforeReadingPayload(t *testing.T) {
	store := &memoryStore{payloads: map[string][]byte{}, blobs: map[string]appcontent.Blob{
		"oversized": {ID: "oversized", CID: "oversized", Size: applicationv1.MaxUnaryPayloadBytes + 1},
	}}
	path, handler := newPrincipalHTTPHandler(t, store)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	app, err := client.New(client.Config{
		Endpoint: server.URL, Credential: client.StaticCredential("application-secret"), HTTPClient: server.Client(),
	})
	require.NoError(t, err)

	_, err = app.Content.Get(context.Background(), content.Reference{Kind: "blob", ID: "oversized"})
	var sdkErr *sdkerrors.Error
	require.ErrorAs(t, err, &sdkErr)
	require.Equal(t, sdkerrors.ResourceExhausted, sdkErr.Code)
}

func TestContentHandlerRejectsMissingAndForeignAdmissionChannel(t *testing.T) {
	store := &memoryStore{payloads: map[string][]byte{}, blobs: map[string]appcontent.Blob{}}
	_, extractor := applicationcall.NewChannel()
	handler, err := applicationcontent.NewHandler(store, extractor)
	require.NoError(t, err)
	request := connect.NewRequest(&applicationv1.PutContentRequest{Payload: []byte("must not mutate")})

	_, err = handler.Put(context.Background(), request)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	require.Empty(t, store.blobs)

	foreignInjector, _ := applicationcall.NewChannel()
	ctx := foreignInjector.WithPrincipal(context.Background(), applicationcall.PrincipalFacts{
		Actor: "p1_actor", Effective: "p1_actor", Node: "p1_node", Interface: 2, ProtocolMajor: 1,
		Action: applicationcontent.ActionPut, ResourceNode: "p1_node", ResourceOwner: "p1_actor", ResourceKind: "content-owner",
	})
	_, err = handler.Put(ctx, request)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	require.Empty(t, store.blobs)
}

func newPrincipalHTTPHandler(t *testing.T, store applicationcontent.Store) (string, http.Handler) {
	t.Helper()
	injector, extractor := applicationcall.NewChannel()
	interceptor := testPrincipalAdmission{injector: injector}
	path, handler, err := applicationcontent.NewHTTPHandler(store, extractor, interceptor)
	require.NoError(t, err)
	return path, handler
}

type testPrincipalAdmission struct{ injector applicationcall.Injector }

func (i testPrincipalAdmission) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, request connect.AnyRequest) (connect.AnyResponse, error) {
		rule, err := applicationcontent.RuleForProcedure(request.Spec().Procedure)
		if err != nil {
			if errors.Is(err, applicationcontent.ErrPayloadTooLarge) {
				return nil, applicationcontent.ProtocolError(applicationv1.ErrorCode_ERROR_CODE_RESOURCE_EXHAUSTED, rule.Action, "content payload exceeds the unary limit", false, connect.CodeResourceExhausted)
			}
			return nil, applicationcontent.ProtocolError(applicationv1.ErrorCode_ERROR_CODE_INVALID_ARGUMENT, rule.Action, "invalid application content request", false, connect.CodeInvalidArgument)
		}
		target, err := applicationcontent.CanonicalizeResource(request.Spec().Procedure, request.Any())
		if err != nil {
			return nil, err
		}
		ctx = i.injector.WithPrincipal(ctx, applicationcall.PrincipalFacts{
			Actor: "p1_test", Effective: "p1_test", Node: "p1_node", Interface: 2, ProtocolMajor: 1,
			Action: rule.Action, ResourceNode: "p1_node", ResourceOwner: "p1_test", ResourceKind: target.Kind, ResourceID: target.ID,
		})
		return next(ctx, request)
	}
}
func (testPrincipalAdmission) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}
func (testPrincipalAdmission) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return next
}

type memoryStore struct {
	blobs    map[string]appcontent.Blob
	payloads map[string][]byte
}

type contentOwner struct {
	store    *appcontent.Service
	commands *appcontent.Commands
	actions  []string
}

func (o *contentOwner) PublishBlob(call applicationcall.Call, command appcontent.PublishBlobCommand) (appcontent.Blob, error) {
	o.actions = append(o.actions, call.Action())
	return o.commands.PublishBlob(command)
}

func (o *contentOwner) GetBlob(call applicationcall.Call, id string) (appcontent.Blob, bool) {
	o.actions = append(o.actions, call.Action())
	return o.store.GetBlob(id)
}

func (o *contentOwner) GetBlobPayload(_ applicationcall.Call, id string) ([]byte, error) {
	return o.store.GetBlobPayload(id)
}

func (o *contentOwner) FetchBlob(context.Context, applicationcall.Call, string) (appcontent.Blob, error) {
	return appcontent.Blob{}, appcontent.ErrBlobNotFound
}

func (s *memoryStore) PublishBlob(_ applicationcall.Call, command appcontent.PublishBlobCommand) (appcontent.Blob, error) {
	hash, id, err := contentpayload.DeriveIdentity(command.Payload)
	if err != nil {
		return appcontent.Blob{}, err
	}
	blob := command.Blob
	blob.ID = id
	blob.CID = id
	blob.Hash = hash
	blob.Size = int64(len(command.Payload))
	s.blobs[id] = blob
	s.payloads[id] = append([]byte(nil), command.Payload...)
	return blob, nil
}

func (s *memoryStore) GetBlob(_ applicationcall.Call, id string) (appcontent.Blob, bool) {
	blob, ok := s.blobs[id]
	return blob, ok
}

func (s *memoryStore) GetBlobPayload(_ applicationcall.Call, id string) ([]byte, error) {
	payload, ok := s.payloads[id]
	if !ok {
		return nil, appcontent.ErrBlobNotFound
	}
	return append([]byte(nil), payload...), nil
}

func (s *memoryStore) FetchBlob(_ context.Context, _ applicationcall.Call, id string) (appcontent.Blob, error) {
	if strings.TrimSpace(id) == "" {
		return appcontent.Blob{}, errors.New("invalid blob id")
	}
	return appcontent.Blob{}, appcontent.ErrBlobNotFound
}
