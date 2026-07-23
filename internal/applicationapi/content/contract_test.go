package content_test

import (
	"encoding/base64"

	applicationadmission "ardents/internal/applicationapi/admission"
	applicationcall "ardents/internal/applicationapi/call"
	applicationcontent "ardents/internal/applicationapi/content"
	appcontent "ardents/internal/content"
	contentpayload "ardents/internal/content/payload"
	identityaccess "ardents/internal/identity/access"
	identityprincipal "ardents/internal/identity/principal"
	applicationv1 "ardents/sdk/go/protocol/applicationv1"
	applicationv1connect "ardents/sdk/go/protocol/applicationv1/applicationv1connect"
	"ardents/tests/testkit"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
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

func TestContentGetRequiresEffectivePrincipalOwnerBinding(t *testing.T) {
	domainStore := appcontent.NewInDir(t.TempDir())
	require.NoError(t, domainStore.Load())
	store := &contentOwner{store: domainStore, commands: appcontent.NewCommands(domainStore, appcontent.CommandConfig{})}
	firstAccess := testkit.NewApplicationPrincipalAccess(t, []identityaccess.Action{applicationcontent.ActionGet, applicationcontent.ActionPut})
	secondAccess := testkit.NewApplicationPrincipalAccess(t, []identityaccess.Action{applicationcontent.ActionGet, applicationcontent.ActionPut})
	first := newPrincipalContentClientWithAccess(t, store, firstAccess)
	second := newPrincipalContentClientWithAccess(t, store, secondAccess)

	put, err := first.Put(context.Background(), connect.NewRequest(&applicationv1.PutContentRequest{Payload: []byte("shared bytes")}))
	require.NoError(t, err)
	reference := put.Msg.GetReference()

	_, err = second.Get(context.Background(), connect.NewRequest(&applicationv1.GetContentRequest{Reference: reference}))
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
	require.Equal(t, applicationv1.ErrorCode_ERROR_CODE_NOT_FOUND, requireApplicationError(t, err).GetCode())

	secondPut, err := second.Put(context.Background(), connect.NewRequest(&applicationv1.PutContentRequest{Payload: []byte("shared bytes")}))
	require.NoError(t, err)
	require.Equal(t, reference.GetId(), secondPut.Msg.GetReference().GetId())
	require.Len(t, domainStore.ListBlobs(), 1)

	_, err = second.Get(context.Background(), connect.NewRequest(&applicationv1.GetContentRequest{Reference: reference}))
	require.NoError(t, err)
}

func storeCID(t *testing.T, store *appcontent.Service, id string) string {
	t.Helper()
	blob, ok := store.GetBlob(id)
	require.True(t, ok)
	return blob.Reference.String()
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
	_, oversized, err := contentpayload.DeriveIdentity([]byte("oversized"))
	require.NoError(t, err)
	store := &memoryStore{payloads: map[string][]byte{}, blobs: map[string]appcontent.Blob{
		oversized.String(): {Reference: oversized, Size: applicationv1.MaxUnaryPayloadBytes + 1},
	}}
	contentClient := newPrincipalContentClient(t, store)

	_, err = contentClient.Get(context.Background(), connect.NewRequest(&applicationv1.GetContentRequest{
		Reference: &applicationv1.ContentReference{Kind: "blob", Id: oversized.String()},
	}))
	require.Equal(t, connect.CodeResourceExhausted, connect.CodeOf(err))
	require.Equal(t, applicationv1.ErrorCode_ERROR_CODE_RESOURCE_EXHAUSTED, requireApplicationError(t, err).GetCode())
}

func TestContentHandlerRejectsMissingAndUnsealedAdmission(t *testing.T) {
	store := &memoryStore{payloads: map[string][]byte{}, blobs: map[string]appcontent.Blob{}}
	_, extractor := applicationcall.NewChannel()
	handler, err := applicationcontent.NewHandler(store, extractor)
	require.NoError(t, err)
	request := connect.NewRequest(&applicationv1.PutContentRequest{Payload: []byte("must not mutate")})

	_, err = handler.Put(context.Background(), request)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	require.Empty(t, store.blobs)

	injector, _ := applicationcall.NewChannel()
	ctx := injector.WithAuthorizedCall(context.Background(), identityaccess.AuthorizedCall{})
	_, err = handler.Put(ctx, request)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	require.Empty(t, store.blobs)
}

func newPrincipalContentClient(t *testing.T, store applicationcontent.Store) applicationv1connect.ContentServiceClient {
	t.Helper()
	fixture := testkit.NewApplicationPrincipalAccess(t, []identityaccess.Action{applicationcontent.ActionGet, applicationcontent.ActionPut})
	return newPrincipalContentClientWithAccess(t, store, fixture)
}

func newPrincipalContentClientWithAccess(t *testing.T, store applicationcontent.Store, fixture testkit.ApplicationPrincipalAccess) applicationv1connect.ContentServiceClient {
	t.Helper()
	injector, extractor := applicationcall.NewChannel()
	interceptor, err := applicationadmission.NewInterceptor(applicationadmission.Config{
		Access: fixture.Service, Node: fixture.Node, FallbackPeer: fixture.Peer, FallbackSource: fixture.Source, Injector: injector,
	})
	require.NoError(t, err)
	path, handler, err := applicationcontent.NewHTTPHandler(store, extractor, interceptor)
	require.NoError(t, err)
	authorization := "ArdentsApplicationSession " + base64.RawURLEncoding.EncodeToString(fixture.Session[:])
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return applicationv1connect.NewContentServiceClient(
		server.Client(),
		server.URL,
		connect.WithReadMaxBytes(applicationv1.MaxUnaryMessageBytes),
		connect.WithSendMaxBytes(applicationv1.MaxUnaryMessageBytes),
		connect.WithInterceptors(applicationSessionHeader{authorization: authorization}),
	)
}

type applicationSessionHeader struct{ authorization string }

func (i applicationSessionHeader) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, request connect.AnyRequest) (connect.AnyResponse, error) {
		request.Header().Set("Authorization", i.authorization)
		return next(ctx, request)
	}
}
func (applicationSessionHeader) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}
func (applicationSessionHeader) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return next
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
	owner, err := identityprincipal.Parse(call.Effective())
	if err != nil {
		return appcontent.Blob{}, err
	}
	return o.commands.PublishBlobForOwner(owner, command)
}

func (o *contentOwner) GetBlob(call applicationcall.Call, id string) (appcontent.Blob, bool) {
	o.actions = append(o.actions, call.Action())
	owner, err := identityprincipal.Parse(call.Effective())
	if err != nil {
		return appcontent.Blob{}, false
	}
	return o.store.GetBlobForOwner(owner, id)
}

func (o *contentOwner) GetBlobPayload(call applicationcall.Call, id string) ([]byte, error) {
	owner, err := identityprincipal.Parse(call.Effective())
	if err != nil {
		return nil, appcontent.ErrBlobNotFound
	}
	return o.store.GetBlobPayloadForOwner(owner, id)
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
	blob.Reference = id
	blob.Hash = hash
	blob.Size = int64(len(command.Payload))
	s.blobs[id.String()] = blob
	s.payloads[id.String()] = append([]byte(nil), command.Payload...)
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
