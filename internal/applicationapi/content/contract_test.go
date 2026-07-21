package content_test

import (
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
	authorizer := &testAuthorizer{token: "application-secret"}
	path, handler, err := applicationcontent.NewHTTPHandler(store, authorizer)
	require.NoError(t, err)
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
	require.Equal(t, []string{applicationcontent.ActionPut, applicationcontent.ActionGet}, authorizer.actions)
}

func storeCID(t *testing.T, store *appcontent.Service, id string) string {
	t.Helper()
	blob, ok := store.GetBlob(id)
	require.True(t, ok)
	return blob.CID
}

func TestContentClientMapsPublicStructuredError(t *testing.T) {
	store := &memoryStore{payloads: map[string][]byte{}, blobs: map[string]appcontent.Blob{}}
	path, handler, err := applicationcontent.NewHTTPHandler(store, &testAuthorizer{token: "application-secret"})
	require.NoError(t, err)
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
	path, handler, err := applicationcontent.NewHTTPHandler(store, &testAuthorizer{token: "application-secret"})
	require.NoError(t, err)
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
	path, handler, err := applicationcontent.NewHTTPHandler(store, &testAuthorizer{token: "application-secret"})
	require.NoError(t, err)
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

func TestContentAuthorizationFailureIsStructuredAndRedacted(t *testing.T) {
	store := &memoryStore{payloads: map[string][]byte{}, blobs: map[string]appcontent.Blob{}}
	path, handler, err := applicationcontent.NewHTTPHandler(store, &testAuthorizer{token: "correct-secret"})
	require.NoError(t, err)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	app, err := client.New(client.Config{
		Endpoint: server.URL, Credential: client.StaticCredential("wrong-secret"), HTTPClient: server.Client(),
	})
	require.NoError(t, err)

	_, err = app.Content.Put(context.Background(), []byte("hello"))
	var sdkErr *sdkerrors.Error
	require.ErrorAs(t, err, &sdkErr)
	require.Equal(t, sdkerrors.Unauthenticated, sdkErr.Code)
	require.Equal(t, applicationcontent.ActionPut, sdkErr.Operation)
	require.NotContains(t, sdkErr.Error(), "sensitive")
}

func TestContentPutRejectsPayloadAboveUnaryLimit(t *testing.T) {
	store := &memoryStore{payloads: map[string][]byte{}, blobs: map[string]appcontent.Blob{}}
	path, handler, err := applicationcontent.NewHTTPHandler(store, &testAuthorizer{token: "application-secret"})
	require.NoError(t, err)
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
	path, handler, err := applicationcontent.NewHTTPHandler(store, &testAuthorizer{token: "application-secret"})
	require.NoError(t, err)
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

type testAuthorizer struct {
	token   string
	actions []string
}

func (a *testAuthorizer) Authorize(_ context.Context, header http.Header, action string) error {
	if header.Get("Authorization") != "ArdentsApplication "+a.token {
		return connect.NewError(connect.CodeUnauthenticated, errors.New("sensitive internal auth detail"))
	}
	a.actions = append(a.actions, action)
	return nil
}

type memoryStore struct {
	blobs    map[string]appcontent.Blob
	payloads map[string][]byte
}

type contentOwner struct {
	store    *appcontent.Service
	commands *appcontent.Commands
}

func (o *contentOwner) PublishBlob(command appcontent.PublishBlobCommand) (appcontent.Blob, error) {
	return o.commands.PublishBlob(command)
}

func (o *contentOwner) GetBlob(id string) (appcontent.Blob, bool) {
	return o.store.GetBlob(id)
}

func (o *contentOwner) GetBlobPayload(id string) ([]byte, error) {
	return o.store.GetBlobPayload(id)
}

func (o *contentOwner) FetchBlob(context.Context, string) (appcontent.Blob, error) {
	return appcontent.Blob{}, appcontent.ErrBlobNotFound
}

func (s *memoryStore) PublishBlob(command appcontent.PublishBlobCommand) (appcontent.Blob, error) {
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

func (s *memoryStore) GetBlob(id string) (appcontent.Blob, bool) {
	blob, ok := s.blobs[id]
	return blob, ok
}

func (s *memoryStore) GetBlobPayload(id string) ([]byte, error) {
	payload, ok := s.payloads[id]
	if !ok {
		return nil, appcontent.ErrBlobNotFound
	}
	return append([]byte(nil), payload...), nil
}

func (s *memoryStore) FetchBlob(_ context.Context, id string) (appcontent.Blob, error) {
	if strings.TrimSpace(id) == "" {
		return appcontent.Blob{}, errors.New("invalid blob id")
	}
	return appcontent.Blob{}, appcontent.ErrBlobNotFound
}
