package identity_test

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	contentapi "ardents/internal/content"
	identityaccess "ardents/internal/identity/access"
	identityprotocol "ardents/internal/identity/protocol"
	operatorapi "ardents/internal/localapi"
	localidentity "ardents/internal/localapi/identity"
	protocol "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/protocol/ardentsv1connect"
	"ardents/internal/localapi/rpc"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
)

type b4AdmissionGuardHandler struct {
	ardentsv1connect.UnimplementedContentServiceHandler
	ardentsv1connect.UnimplementedTransferServiceHandler
	ardentsv1connect.UnimplementedRetentionServiceHandler
	calls int32
}

func (h *b4AdmissionGuardHandler) admitted(ctx context.Context) error {
	call, ok := rpc.CallFromContext(ctx)
	if !ok || call.Actor() == "" || call.Effective() == "" {
		return connect.NewError(connect.CodeUnauthenticated, context.Canceled)
	}
	h.calls++
	return nil
}

func (h *b4AdmissionGuardHandler) PublishObject(ctx context.Context, _ *connect.Request[protocol.PublishObjectRequest]) (*connect.Response[protocol.ObjectSnapshot], error) {
	return connect.NewResponse(&protocol.ObjectSnapshot{}), h.admitted(ctx)
}
func (h *b4AdmissionGuardHandler) GetObject(ctx context.Context, _ *connect.Request[protocol.GetObjectRequest]) (*connect.Response[protocol.ObjectSnapshot], error) {
	return connect.NewResponse(&protocol.ObjectSnapshot{}), h.admitted(ctx)
}
func (h *b4AdmissionGuardHandler) ListObjects(ctx context.Context, _ *connect.Request[protocol.ListObjectsRequest]) (*connect.Response[protocol.ListObjectsResponse], error) {
	return connect.NewResponse(&protocol.ListObjectsResponse{}), h.admitted(ctx)
}
func (h *b4AdmissionGuardHandler) PublishBlob(ctx context.Context, _ *connect.Request[protocol.PublishBlobRequest]) (*connect.Response[protocol.BlobSnapshot], error) {
	return connect.NewResponse(&protocol.BlobSnapshot{}), h.admitted(ctx)
}
func (h *b4AdmissionGuardHandler) GetBlob(ctx context.Context, _ *connect.Request[protocol.GetBlobRequest]) (*connect.Response[protocol.BlobSnapshot], error) {
	return connect.NewResponse(&protocol.BlobSnapshot{}), h.admitted(ctx)
}
func (h *b4AdmissionGuardHandler) ListBlobs(ctx context.Context, _ *connect.Request[protocol.ListBlobsRequest]) (*connect.Response[protocol.ListBlobsResponse], error) {
	return connect.NewResponse(&protocol.ListBlobsResponse{}), h.admitted(ctx)
}
func (h *b4AdmissionGuardHandler) PublishManifest(ctx context.Context, _ *connect.Request[protocol.PublishManifestRequest]) (*connect.Response[protocol.ManifestSnapshot], error) {
	return connect.NewResponse(&protocol.ManifestSnapshot{}), h.admitted(ctx)
}
func (h *b4AdmissionGuardHandler) GetManifest(ctx context.Context, _ *connect.Request[protocol.GetManifestRequest]) (*connect.Response[protocol.ManifestSnapshot], error) {
	return connect.NewResponse(&protocol.ManifestSnapshot{}), h.admitted(ctx)
}
func (h *b4AdmissionGuardHandler) ListManifests(ctx context.Context, _ *connect.Request[protocol.ListManifestsRequest]) (*connect.Response[protocol.ListManifestsResponse], error) {
	return connect.NewResponse(&protocol.ListManifestsResponse{}), h.admitted(ctx)
}
func (h *b4AdmissionGuardHandler) GetDataInventory(ctx context.Context, _ *connect.Request[protocol.GetDataInventoryRequest]) (*connect.Response[protocol.DataInventorySnapshot], error) {
	return connect.NewResponse(&protocol.DataInventorySnapshot{}), h.admitted(ctx)
}
func (h *b4AdmissionGuardHandler) FetchBlob(ctx context.Context, _ *connect.Request[protocol.FetchBlobRequest]) (*connect.Response[protocol.BlobSnapshot], error) {
	return connect.NewResponse(&protocol.BlobSnapshot{}), h.admitted(ctx)
}
func (h *b4AdmissionGuardHandler) ListBlobSources(ctx context.Context, _ *connect.Request[protocol.ListBlobSourcesRequest]) (*connect.Response[protocol.ListBlobSourcesResponse], error) {
	return connect.NewResponse(&protocol.ListBlobSourcesResponse{}), h.admitted(ctx)
}
func (h *b4AdmissionGuardHandler) GetTransfer(ctx context.Context, _ *connect.Request[protocol.GetTransferRequest]) (*connect.Response[protocol.GetTransferResponse], error) {
	return connect.NewResponse(&protocol.GetTransferResponse{}), h.admitted(ctx)
}
func (h *b4AdmissionGuardHandler) ListTransfers(ctx context.Context, _ *connect.Request[protocol.ListTransfersRequest]) (*connect.Response[protocol.ListTransfersResponse], error) {
	return connect.NewResponse(&protocol.ListTransfersResponse{}), h.admitted(ctx)
}
func (h *b4AdmissionGuardHandler) RetainBlob(ctx context.Context, _ *connect.Request[protocol.RetainBlobRequest]) (*connect.Response[protocol.BlobSnapshot], error) {
	return connect.NewResponse(&protocol.BlobSnapshot{}), h.admitted(ctx)
}
func (h *b4AdmissionGuardHandler) PinBlob(ctx context.Context, _ *connect.Request[protocol.PinBlobRequest]) (*connect.Response[protocol.BlobSnapshot], error) {
	return connect.NewResponse(&protocol.BlobSnapshot{}), h.admitted(ctx)
}
func (h *b4AdmissionGuardHandler) DropBlob(ctx context.Context, _ *connect.Request[protocol.DropBlobRequest]) (*connect.Response[protocol.BlobSnapshot], error) {
	return connect.NewResponse(&protocol.BlobSnapshot{}), h.admitted(ctx)
}

var b4Actions = []identityaccess.Action{
	"data.publish_object", "data.get_object", "data.list_objects", "data.publish_blob", "data.get_blob", "data.list_blobs",
	"data.publish_manifest", "data.get_manifest", "data.list_manifests", "data.inventory", "data.fetch_blob", "data.blob_sources",
	"data.get_transfer", "data.list_transfers", "data.retain_blob", "data.pin_blob", "data.drop_blob",
}

var b4Procedures = []string{
	ardentsv1connect.ContentServicePublishObjectProcedure, ardentsv1connect.ContentServiceGetObjectProcedure, ardentsv1connect.ContentServiceListObjectsProcedure,
	ardentsv1connect.ContentServicePublishBlobProcedure, ardentsv1connect.ContentServiceGetBlobProcedure, ardentsv1connect.ContentServiceListBlobsProcedure,
	ardentsv1connect.ContentServicePublishManifestProcedure, ardentsv1connect.ContentServiceGetManifestProcedure, ardentsv1connect.ContentServiceListManifestsProcedure,
	ardentsv1connect.ContentServiceGetDataInventoryProcedure, ardentsv1connect.TransferServiceFetchBlobProcedure, ardentsv1connect.TransferServiceListBlobSourcesProcedure,
	ardentsv1connect.TransferServiceGetTransferProcedure, ardentsv1connect.TransferServiceListTransfersProcedure,
	ardentsv1connect.RetentionServiceRetainBlobProcedure, ardentsv1connect.RetentionServicePinBlobProcedure, ardentsv1connect.RetentionServiceDropBlobProcedure,
}

func TestOperatorPrincipalInterceptorGuardsEveryB4ProcedureExactlyOnce(t *testing.T) {
	service, node, _, secret, peer, source, _ := operatorAccessFixtureWithActions(t, b4Actions)
	counter := &countingAdmitter{service: service}
	interceptor, err := localidentity.NewOperatorInterceptor(localidentity.OperatorInterceptorConfig{Access: counter, Node: node, FallbackPeer: peer, FallbackSource: source, Canonicalize: operatorapi.CanonicalizeOperatorResource})
	require.NoError(t, err)
	handler := &b4AdmissionGuardHandler{}
	server, contentClient, transferClient, retentionClient := b4Server(t, interceptor, handler)
	defer server.Close()
	authorization := "ArdentsOperatorSession " + base64.RawURLEncoding.EncodeToString(secret[:])
	for _, procedure := range b4Procedures {
		beforeAdmission, beforeHandler := counter.calls.Load(), handler.calls
		require.NoError(t, invokeB4Procedure(context.Background(), contentClient, transferClient, retentionClient, procedure, authorization), procedure)
		require.Equal(t, beforeAdmission+1, counter.calls.Load(), procedure)
		require.Equal(t, beforeHandler+1, handler.calls, procedure)
	}
	require.Equal(t, int32(len(b4Actions)), counter.calls.Load())

	malformed := connect.NewRequest(&protocol.PublishBlobRequest{Blob: &protocol.BlobSnapshot{}})
	malformed.Header().Set("Authorization", authorization)
	_, err = contentClient.PublishBlob(context.Background(), malformed)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	require.Equal(t, int32(len(b4Actions)+1), counter.calls.Load())
	require.Equal(t, int32(len(b4Actions)), handler.calls)
}

func TestOperatorPrincipalInterceptorDefersPublishBlobHashUntilSessionValidation(t *testing.T) {
	service, node, _, secret, peer, source, _ := operatorAccessFixtureWithActions(t, []identityaccess.Action{"data.publish_blob"})
	counter := &countingAdmitter{service: service}
	canonicalized := 0
	canonicalize := func(procedure string, message any) (identityaccess.ResourceTarget, error) {
		canonicalized++
		return operatorapi.CanonicalizeOperatorResource(procedure, message)
	}
	interceptor, err := localidentity.NewOperatorInterceptor(localidentity.OperatorInterceptorConfig{Access: counter, Node: node, FallbackPeer: peer, FallbackSource: source, Canonicalize: canonicalize})
	require.NoError(t, err)
	handler := &b4AdmissionGuardHandler{}
	server, contentClient, _, _ := b4Server(t, interceptor, handler)
	defer server.Close()
	request := connect.NewRequest(&protocol.PublishBlobRequest{Blob: &protocol.BlobSnapshot{Payload: []byte("hello")}})
	request.Header().Set("Authorization", "ArdentsOperatorSession "+base64.RawURLEncoding.EncodeToString(make([]byte, 32)))
	_, err = contentClient.PublishBlob(context.Background(), request)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	require.Zero(t, canonicalized)
	require.Zero(t, handler.calls)

	request = connect.NewRequest(&protocol.PublishBlobRequest{Blob: &protocol.BlobSnapshot{Payload: []byte("hello")}})
	request.Header().Set("Authorization", "ArdentsOperatorSession "+base64.RawURLEncoding.EncodeToString(secret[:]))
	_, err = contentClient.PublishBlob(context.Background(), request)
	require.NoError(t, err)
	require.Equal(t, 1, canonicalized)
	require.Equal(t, int32(1), handler.calls)
}

func TestOperatorPrincipalInterceptorDeniesEveryB4SiblingAction(t *testing.T) {
	for index, grant := range b4Actions {
		sibling := b4Procedures[(index+1)%len(b4Procedures)]
		t.Run(string(grant), func(t *testing.T) {
			service, node, _, secret, peer, source, _ := operatorAccessFixtureWithActions(t, []identityaccess.Action{grant})
			counter := &countingAdmitter{service: service}
			interceptor, err := localidentity.NewOperatorInterceptor(localidentity.OperatorInterceptorConfig{Access: counter, Node: node, FallbackPeer: peer, FallbackSource: source, Canonicalize: operatorapi.CanonicalizeOperatorResource})
			require.NoError(t, err)
			handler := &b4AdmissionGuardHandler{}
			server, contentClient, transferClient, retentionClient := b4Server(t, interceptor, handler)
			defer server.Close()
			authorization := "ArdentsOperatorSession " + base64.RawURLEncoding.EncodeToString(secret[:])
			err = invokeB4Procedure(context.Background(), contentClient, transferClient, retentionClient, sibling, authorization)
			require.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
			require.Equal(t, int32(1), counter.calls.Load())
			require.Zero(t, handler.calls)
		})
	}
}

func TestOperatorPrincipalInterceptorDeniesB4SiblingExactResources(t *testing.T) {
	tests := []struct {
		name      string
		action    identityaccess.Action
		kind      identityaccess.ResourceKind
		withOwner bool
		match     string
		other     string
	}{
		{"content owner", "data.get_object", "content-object", true, "obj-1", "obj-2"},
		{"transfer", "data.get_transfer", "transfer", false, "transfer-1", "transfer-2"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service, node, principal, secret, peer, source, _ := operatorAccessFixtureWithActions(t, []identityaccess.Action{"node.status"})
			owner := ""
			if test.withOwner {
				owner = principal
			}
			issueB4ExactGrant(t, service, node, principal, secret, peer, test.action, test.kind, owner, test.match)
			counter := &countingAdmitter{service: service}
			interceptor, err := localidentity.NewOperatorInterceptor(localidentity.OperatorInterceptorConfig{Access: counter, Node: node, FallbackPeer: peer, FallbackSource: source, Canonicalize: operatorapi.CanonicalizeOperatorResource})
			require.NoError(t, err)
			handler := &b4AdmissionGuardHandler{}
			server, contentClient, transferClient, _ := b4Server(t, interceptor, handler)
			defer server.Close()
			authorization := "ArdentsOperatorSession " + base64.RawURLEncoding.EncodeToString(secret[:])
			require.NoError(t, invokeB4ExactProcedure(context.Background(), contentClient, transferClient, test.action, test.match, authorization))
			require.Equal(t, int32(1), counter.calls.Load())
			require.Equal(t, int32(1), handler.calls)

			err = invokeB4ExactProcedure(context.Background(), contentClient, transferClient, test.action, test.other, authorization)
			require.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
			require.Equal(t, int32(2), counter.calls.Load())
			require.Equal(t, int32(1), handler.calls)
		})
	}
}

func TestOperatorPrincipalInterceptorDeniesSiblingPublishBlobPayload(t *testing.T) {
	service, node, principal, secret, peer, source, _ := operatorAccessFixtureWithActions(t, []identityaccess.Action{"node.status"})
	matchingPayload := []byte("matching payload")
	id, err := contentapi.PublishBlobAccessResourceID(contentapi.PublishBlobCommand{Payload: matchingPayload})
	require.NoError(t, err)
	issueB4ExactGrant(t, service, node, principal, secret, peer, "data.publish_blob", "content-blob", principal, id)
	counter := &countingAdmitter{service: service}
	interceptor, err := localidentity.NewOperatorInterceptor(localidentity.OperatorInterceptorConfig{Access: counter, Node: node, FallbackPeer: peer, FallbackSource: source, Canonicalize: operatorapi.CanonicalizeOperatorResource})
	require.NoError(t, err)
	handler := &b4AdmissionGuardHandler{}
	server, contentClient, _, _ := b4Server(t, interceptor, handler)
	defer server.Close()
	authorization := "ArdentsOperatorSession " + base64.RawURLEncoding.EncodeToString(secret[:])

	request := connect.NewRequest(&protocol.PublishBlobRequest{Blob: &protocol.BlobSnapshot{Payload: matchingPayload}})
	request.Header().Set("Authorization", authorization)
	_, err = contentClient.PublishBlob(context.Background(), request)
	require.NoError(t, err)
	require.Equal(t, int32(1), counter.calls.Load())
	require.Equal(t, int32(1), handler.calls)

	request = connect.NewRequest(&protocol.PublishBlobRequest{Blob: &protocol.BlobSnapshot{Payload: []byte("sibling payload")}})
	request.Header().Set("Authorization", authorization)
	_, err = contentClient.PublishBlob(context.Background(), request)
	require.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
	require.Equal(t, int32(2), counter.calls.Load())
	require.Equal(t, int32(1), handler.calls)
}

func issueB4ExactGrant(t *testing.T, service *identityaccess.Service, node, principal string, secret identityaccess.SessionSecret, peer [32]byte, action identityaccess.Action, kind identityaccess.ResourceKind, owner, id string) {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	binding := identityaccess.AuthenticationBinding{
		Audience:         identityaccess.Audience{Node: node, Interface: identityprotocol.Interface_INTERFACE_OPERATOR, ProtocolMajor: 1},
		TransportProfile: identityprotocol.TransportProfile_TRANSPORT_PROFILE_UNIX_LOCAL_V1,
		PeerBinding:      peer,
	}
	parsedOwner, err := identityaccess.ParseResourceOwner(owner)
	require.NoError(t, err)
	exact, err := identityaccess.NewResourceRef(node, parsedOwner, string(kind), id)
	require.NoError(t, err)
	proposal := identityaccess.GrantProposal{
		Subject: principal, Actions: []identityaccess.Action{action}, Scope: identityaccess.ResourceScope{Kind: identityaccess.ScopeExact, Exact: exact},
		NotBefore: now, NotAfter: now.Add(time.Hour),
	}
	proposalID, err := identityaccess.GrantProposalResourceID(node, binding.Audience, proposal)
	require.NoError(t, err)
	resource, err := identityaccess.NewResourceRef(node, identityaccess.ResourceOwner{}, "grant-proposal", proposalID)
	require.NoError(t, err)
	_, err = service.IssueAccessGrant(context.Background(), identityaccess.IssueGrantRequest{
		Command: identityaccess.AdminCommand{
			RequestID: "b4-exact-" + testResourceID(t, id),
			Attempt:   identityaccess.Attempt{SessionSecret: secret, Binding: binding, Action: "identity.grant.issue", Resource: resource},
		},
		Proposal: proposal,
	})
	require.NoError(t, err)
}

func invokeB4ExactProcedure(ctx context.Context, contentClient ardentsv1connect.ContentServiceClient, transferClient ardentsv1connect.TransferServiceClient, action identityaccess.Action, id, authorization string) error {
	switch action {
	case "data.get_object":
		request := connect.NewRequest(&protocol.GetObjectRequest{Id: id})
		request.Header().Set("Authorization", authorization)
		_, err := contentClient.GetObject(ctx, request)
		return err
	case "data.get_transfer":
		request := connect.NewRequest(&protocol.GetTransferRequest{Id: id})
		request.Header().Set("Authorization", authorization)
		_, err := transferClient.GetTransfer(ctx, request)
		return err
	default:
		return connect.NewError(connect.CodeInternal, context.Canceled)
	}
}

func b4Server(t *testing.T, interceptor connect.Interceptor, handler *b4AdmissionGuardHandler) (*httptest.Server, ardentsv1connect.ContentServiceClient, ardentsv1connect.TransferServiceClient, ardentsv1connect.RetentionServiceClient) {
	t.Helper()
	mux := http.NewServeMux()
	option := connect.WithInterceptors(interceptor)
	contentPath, contentHTTP := ardentsv1connect.NewContentServiceHandler(handler, option)
	transferPath, transferHTTP := ardentsv1connect.NewTransferServiceHandler(handler, option)
	retentionPath, retentionHTTP := ardentsv1connect.NewRetentionServiceHandler(handler, option)
	mux.Handle(contentPath, contentHTTP)
	mux.Handle(transferPath, transferHTTP)
	mux.Handle(retentionPath, retentionHTTP)
	server := httptest.NewServer(mux)
	return server, ardentsv1connect.NewContentServiceClient(server.Client(), server.URL), ardentsv1connect.NewTransferServiceClient(server.Client(), server.URL), ardentsv1connect.NewRetentionServiceClient(server.Client(), server.URL)
}

func invokeB4Procedure(ctx context.Context, contentClient ardentsv1connect.ContentServiceClient, transferClient ardentsv1connect.TransferServiceClient, retentionClient ardentsv1connect.RetentionServiceClient, procedure, authorization string) error {
	set := func(request connect.AnyRequest) { request.Header().Set("Authorization", authorization) }
	switch procedure {
	case ardentsv1connect.ContentServicePublishObjectProcedure:
		r := connect.NewRequest(&protocol.PublishObjectRequest{Object: &protocol.ObjectSnapshot{Id: "obj-1"}})
		set(r)
		_, err := contentClient.PublishObject(ctx, r)
		return err
	case ardentsv1connect.ContentServiceGetObjectProcedure:
		r := connect.NewRequest(&protocol.GetObjectRequest{Id: "obj-1"})
		set(r)
		_, err := contentClient.GetObject(ctx, r)
		return err
	case ardentsv1connect.ContentServiceListObjectsProcedure:
		r := connect.NewRequest(&protocol.ListObjectsRequest{})
		set(r)
		_, err := contentClient.ListObjects(ctx, r)
		return err
	case ardentsv1connect.ContentServicePublishBlobProcedure:
		r := connect.NewRequest(&protocol.PublishBlobRequest{Blob: &protocol.BlobSnapshot{Payload: []byte("hello")}})
		set(r)
		_, err := contentClient.PublishBlob(ctx, r)
		return err
	case ardentsv1connect.ContentServiceGetBlobProcedure:
		r := connect.NewRequest(&protocol.GetBlobRequest{Id: "blob-1"})
		set(r)
		_, err := contentClient.GetBlob(ctx, r)
		return err
	case ardentsv1connect.ContentServiceListBlobsProcedure:
		r := connect.NewRequest(&protocol.ListBlobsRequest{})
		set(r)
		_, err := contentClient.ListBlobs(ctx, r)
		return err
	case ardentsv1connect.ContentServicePublishManifestProcedure:
		r := connect.NewRequest(&protocol.PublishManifestRequest{Manifest: &protocol.ManifestSnapshot{Id: "manifest-1"}})
		set(r)
		_, err := contentClient.PublishManifest(ctx, r)
		return err
	case ardentsv1connect.ContentServiceGetManifestProcedure:
		r := connect.NewRequest(&protocol.GetManifestRequest{Id: "manifest-1"})
		set(r)
		_, err := contentClient.GetManifest(ctx, r)
		return err
	case ardentsv1connect.ContentServiceListManifestsProcedure:
		r := connect.NewRequest(&protocol.ListManifestsRequest{})
		set(r)
		_, err := contentClient.ListManifests(ctx, r)
		return err
	case ardentsv1connect.ContentServiceGetDataInventoryProcedure:
		r := connect.NewRequest(&protocol.GetDataInventoryRequest{})
		set(r)
		_, err := contentClient.GetDataInventory(ctx, r)
		return err
	case ardentsv1connect.TransferServiceFetchBlobProcedure:
		r := connect.NewRequest(&protocol.FetchBlobRequest{Id: "blob-1"})
		set(r)
		_, err := transferClient.FetchBlob(ctx, r)
		return err
	case ardentsv1connect.TransferServiceListBlobSourcesProcedure:
		r := connect.NewRequest(&protocol.ListBlobSourcesRequest{Id: "blob-1"})
		set(r)
		_, err := transferClient.ListBlobSources(ctx, r)
		return err
	case ardentsv1connect.TransferServiceGetTransferProcedure:
		r := connect.NewRequest(&protocol.GetTransferRequest{Id: "transfer-1"})
		set(r)
		_, err := transferClient.GetTransfer(ctx, r)
		return err
	case ardentsv1connect.TransferServiceListTransfersProcedure:
		r := connect.NewRequest(&protocol.ListTransfersRequest{})
		set(r)
		_, err := transferClient.ListTransfers(ctx, r)
		return err
	case ardentsv1connect.RetentionServiceRetainBlobProcedure:
		r := connect.NewRequest(&protocol.RetainBlobRequest{Id: "blob-1"})
		set(r)
		_, err := retentionClient.RetainBlob(ctx, r)
		return err
	case ardentsv1connect.RetentionServicePinBlobProcedure:
		r := connect.NewRequest(&protocol.PinBlobRequest{Id: "blob-1"})
		set(r)
		_, err := retentionClient.PinBlob(ctx, r)
		return err
	case ardentsv1connect.RetentionServiceDropBlobProcedure:
		r := connect.NewRequest(&protocol.DropBlobRequest{Id: "blob-1"})
		set(r)
		_, err := retentionClient.DropBlob(ctx, r)
		return err
	default:
		return connect.NewError(connect.CodeInternal, context.Canceled)
	}
}
