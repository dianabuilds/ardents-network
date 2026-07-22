package content_test

import (
	applicationcall "ardents/internal/applicationapi/call"
	applicationcontent "ardents/internal/applicationapi/content"
	appcontent "ardents/internal/content"
	contentpayload "ardents/internal/content/payload"
	"ardents/sdk/go/client"
	applicationv1 "ardents/sdk/go/protocol/applicationv1"
	applicationv1connect "ardents/sdk/go/protocol/applicationv1/applicationv1connect"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
)

func TestContentPutGetCrossesPublicApplicationContract(t *testing.T) {
	domainStore := appcontent.NewInDir(t.TempDir())
	require.NoError(t, domainStore.Load())
	store := &contentOwner{store: domainStore, commands: appcontent.NewCommands(domainStore, appcontent.CommandConfig{})}
	contentClient := newPrincipalContentClient(t, store)

	put, err := contentClient.Put(context.Background(), connect.NewRequest(&applicationv1.PutContentRequest{
		Payload: []byte("hello"), MediaType: "text/plain",
	}))
	require.NoError(t, err)
	reference := put.Msg.GetReference()
	require.Equal(t, "blob", reference.GetKind())
	require.NotEmpty(t, reference.GetId())
	require.Equal(t, reference.GetId(), storeCID(t, domainStore, reference.GetId()))

	get, err := contentClient.Get(context.Background(), connect.NewRequest(&applicationv1.GetContentRequest{Reference: reference}))
	require.NoError(t, err)
	require.Equal(t, []byte("hello"), get.Msg.GetPayload())
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
	contentClient := newPrincipalContentClient(t, store)

	_, err := contentClient.Get(context.Background(), connect.NewRequest(&applicationv1.GetContentRequest{
		Reference: &applicationv1.ContentReference{Kind: "blob", Id: "missing"},
	}))
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
	protocolErr := requireApplicationError(t, err)
	require.Equal(t, applicationv1.ErrorCode_ERROR_CODE_NOT_FOUND, protocolErr.GetCode())
	require.Equal(t, applicationcontent.ActionGet, protocolErr.GetOperation())
	require.False(t, protocolErr.GetRetryable())
}

func TestApplicationClientRequiresProtectedUnixSocket(t *testing.T) {
	_, err := client.New(client.Config{})
	require.ErrorContains(t, err, "protected Unix socket")
}

func TestApplicationClientRequiresPrincipalSessionSigner(t *testing.T) {
	_, err := client.New(client.Config{SocketPath: filepath.Join(t.TempDir(), "application.sock")})
	require.ErrorContains(t, err, "session signer is required")
}

func TestContentGetRejectsSameLengthPayloadTampering(t *testing.T) {
	store := &memoryStore{payloads: map[string][]byte{}, blobs: map[string]appcontent.Blob{}}
	contentClient := newPrincipalContentClient(t, store)
	put, err := contentClient.Put(context.Background(), connect.NewRequest(&applicationv1.PutContentRequest{Payload: []byte("hello")}))
	require.NoError(t, err)
	reference := put.Msg.GetReference()
	store.payloads[reference.GetId()] = []byte("jello")

	_, err = contentClient.Get(context.Background(), connect.NewRequest(&applicationv1.GetContentRequest{Reference: reference}))
	require.Equal(t, connect.CodeDataLoss, connect.CodeOf(err))
	require.Equal(t, applicationv1.ErrorCode_ERROR_CODE_INTEGRITY_FAILED, requireApplicationError(t, err).GetCode())
}

func TestContentPutRejectsPayloadAboveUnaryLimit(t *testing.T) {
	store := &memoryStore{payloads: map[string][]byte{}, blobs: map[string]appcontent.Blob{}}
	contentClient := newPrincipalContentClient(t, store)

	_, err := contentClient.Put(context.Background(), connect.NewRequest(&applicationv1.PutContentRequest{
		Payload: make([]byte, applicationv1.MaxUnaryPayloadBytes+1),
	}))
	require.Equal(t, connect.CodeResourceExhausted, connect.CodeOf(err), err)
	require.Empty(t, store.blobs)
}

func TestContentGetRejectsOversizedBlobBeforeReadingPayload(t *testing.T) {
	store := &memoryStore{payloads: map[string][]byte{}, blobs: map[string]appcontent.Blob{
		"oversized": {ID: "oversized", CID: "oversized", Size: applicationv1.MaxUnaryPayloadBytes + 1},
	}}
	contentClient := newPrincipalContentClient(t, store)

	_, err := contentClient.Get(context.Background(), connect.NewRequest(&applicationv1.GetContentRequest{
		Reference: &applicationv1.ContentReference{Kind: "blob", Id: "oversized"},
	}))
	require.Equal(t, connect.CodeResourceExhausted, connect.CodeOf(err))
	require.Equal(t, applicationv1.ErrorCode_ERROR_CODE_RESOURCE_EXHAUSTED, requireApplicationError(t, err).GetCode())
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

func newPrincipalContentClient(t *testing.T, store applicationcontent.Store) applicationv1connect.ContentServiceClient {
	t.Helper()
	path, handler := newPrincipalHTTPHandler(t, store)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return applicationv1connect.NewContentServiceClient(
		server.Client(),
		server.URL,
		connect.WithReadMaxBytes(applicationv1.MaxUnaryMessageBytes),
		connect.WithSendMaxBytes(applicationv1.MaxUnaryMessageBytes),
	)
}

func requireApplicationError(t *testing.T, err error) *applicationv1.ApplicationError {
	t.Helper()
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	for _, detail := range connectErr.Details() {
		value, detailErr := detail.Value()
		require.NoError(t, detailErr)
		if applicationErr, ok := value.(*applicationv1.ApplicationError); ok {
			return applicationErr
		}
	}
	require.FailNow(t, "connect error has no ApplicationError detail")
	return nil
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
			if errors.Is(err, applicationcontent.ErrPayloadTooLarge) {
				return nil, applicationcontent.ProtocolError(applicationv1.ErrorCode_ERROR_CODE_RESOURCE_EXHAUSTED, rule.Action, "content payload exceeds the unary limit", false, connect.CodeResourceExhausted)
			}
			return nil, applicationcontent.ProtocolError(applicationv1.ErrorCode_ERROR_CODE_INVALID_ARGUMENT, rule.Action, "invalid application content request", false, connect.CodeInvalidArgument)
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
