package identity_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	discoveryrecord "ardents/internal/discovery/records"
	identityaccess "ardents/internal/identity/access"
	identityprincipal "ardents/internal/identity/principal"
	identityprotocol "ardents/internal/identity/protocol"
	operatorapi "ardents/internal/localapi"
	localidentity "ardents/internal/localapi/identity"
	protocol "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/protocol/ardentsv1connect"
	"ardents/internal/localapi/rpc"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type b2AdmissionGuardHandler struct {
	ardentsv1connect.UnimplementedNetworkServiceHandler
	ardentsv1connect.UnimplementedDiagnosticsServiceHandler
	calls int32
}

func (h *b2AdmissionGuardHandler) admitted(ctx context.Context) error {
	call, ok := rpc.CallFromContext(ctx)
	if !ok || call.Actor() == "" || call.Effective() == "" {
		return connect.NewError(connect.CodeUnauthenticated, context.Canceled)
	}
	h.calls++
	return nil
}

func (h *b2AdmissionGuardHandler) GetNetworkStatus(ctx context.Context, _ *connect.Request[protocol.GetNetworkStatusRequest]) (*connect.Response[protocol.NetworkStatusResponse], error) {
	return connect.NewResponse(&protocol.NetworkStatusResponse{}), h.admitted(ctx)
}
func (h *b2AdmissionGuardHandler) GetDiscoveryStatus(ctx context.Context, _ *connect.Request[protocol.GetDiscoveryStatusRequest]) (*connect.Response[protocol.DiscoveryStatusResponse], error) {
	return connect.NewResponse(&protocol.DiscoveryStatusResponse{}), h.admitted(ctx)
}
func (h *b2AdmissionGuardHandler) GetLocalPresence(ctx context.Context, _ *connect.Request[protocol.GetLocalPresenceRequest]) (*connect.Response[protocol.LocalPresenceResponse], error) {
	return connect.NewResponse(&protocol.LocalPresenceResponse{}), h.admitted(ctx)
}
func (h *b2AdmissionGuardHandler) ListPeers(ctx context.Context, _ *connect.Request[protocol.ListPeersRequest]) (*connect.Response[protocol.ListPeersResponse], error) {
	return connect.NewResponse(&protocol.ListPeersResponse{}), h.admitted(ctx)
}
func (h *b2AdmissionGuardHandler) ListRouteCandidates(ctx context.Context, _ *connect.Request[protocol.ListRouteCandidatesRequest]) (*connect.Response[protocol.ListRouteCandidatesResponse], error) {
	return connect.NewResponse(&protocol.ListRouteCandidatesResponse{}), h.admitted(ctx)
}
func (h *b2AdmissionGuardHandler) ResolveRecord(ctx context.Context, _ *connect.Request[protocol.ResolveRecordRequest]) (*connect.Response[protocol.DiscoveryResult], error) {
	return connect.NewResponse(&protocol.DiscoveryResult{}), h.admitted(ctx)
}
func (h *b2AdmissionGuardHandler) ResolveService(ctx context.Context, _ *connect.Request[protocol.ResolveServiceRequest]) (*connect.Response[protocol.ServiceResult], error) {
	return connect.NewResponse(&protocol.ServiceResult{}), h.admitted(ctx)
}
func (h *b2AdmissionGuardHandler) ListRecords(ctx context.Context, _ *connect.Request[protocol.ListRecordsRequest]) (*connect.Response[protocol.ListRecordsResponse], error) {
	return connect.NewResponse(&protocol.ListRecordsResponse{}), h.admitted(ctx)
}
func (h *b2AdmissionGuardHandler) ImportRecord(ctx context.Context, _ *connect.Request[protocol.ImportRecordRequest]) (*connect.Response[protocol.RecordImportResponse], error) {
	return connect.NewResponse(&protocol.RecordImportResponse{}), h.admitted(ctx)
}
func (h *b2AdmissionGuardHandler) GetDiagnostics(ctx context.Context, _ *connect.Request[protocol.GetDiagnosticsRequest]) (*connect.Response[protocol.DiagnosticsSnapshotResponse], error) {
	return connect.NewResponse(&protocol.DiagnosticsSnapshotResponse{}), h.admitted(ctx)
}
func (h *b2AdmissionGuardHandler) GetPendingOperations(ctx context.Context, _ *connect.Request[protocol.GetPendingOperationsRequest]) (*connect.Response[protocol.PendingOperationsResponse], error) {
	return connect.NewResponse(&protocol.PendingOperationsResponse{}), h.admitted(ctx)
}
func (h *b2AdmissionGuardHandler) GetHealthSummary(ctx context.Context, _ *connect.Request[protocol.GetHealthSummaryRequest]) (*connect.Response[protocol.HealthSummaryResponse], error) {
	return connect.NewResponse(&protocol.HealthSummaryResponse{}), h.admitted(ctx)
}
func (h *b2AdmissionGuardHandler) ExplainFailure(ctx context.Context, _ *connect.Request[protocol.ExplainFailureRequest]) (*connect.Response[protocol.FailureExplanationResponse], error) {
	return connect.NewResponse(&protocol.FailureExplanationResponse{}), h.admitted(ctx)
}
func (h *b2AdmissionGuardHandler) ListRecentEvents(ctx context.Context, _ *connect.Request[protocol.ListRecentEventsRequest]) (*connect.Response[protocol.ListEventsResponse], error) {
	return connect.NewResponse(&protocol.ListEventsResponse{}), h.admitted(ctx)
}

func TestOperatorPrincipalInterceptorGuardsEveryB2ProcedureExactlyOnce(t *testing.T) {
	actions := []identityaccess.Action{
		"transport.network_status", "discovery.status", "discovery.local_presence", "discovery.peers", "transport.route_candidates",
		"discovery.resolve_record", "discovery.resolve_service", "discovery.list_records", "discovery.import",
		"diagnostics.snapshot", "diagnostics.pending_operations", "diagnostics.health_summary", "diagnostics.explain_failure", "diagnostics.recent_events",
	}
	service, node, _, secret, peer, source, _ := operatorAccessFixtureWithActions(t, actions)
	counter := &countingAdmitter{service: service}
	interceptor, err := localidentity.NewOperatorInterceptor(localidentity.OperatorInterceptorConfig{Access: counter, Node: node, FallbackPeer: peer, FallbackSource: source, Canonicalize: operatorapi.CanonicalizeOperatorResource})
	require.NoError(t, err)
	handler := &b2AdmissionGuardHandler{}
	mux := http.NewServeMux()
	networkPath, networkHTTP := ardentsv1connect.NewNetworkServiceHandler(handler, connect.WithInterceptors(interceptor))
	diagnosticsPath, diagnosticsHTTP := ardentsv1connect.NewDiagnosticsServiceHandler(handler, connect.WithInterceptors(interceptor))
	mux.Handle(networkPath, networkHTTP)
	mux.Handle(diagnosticsPath, diagnosticsHTTP)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	network := ardentsv1connect.NewNetworkServiceClient(server.Client(), server.URL)
	diagnostics := ardentsv1connect.NewDiagnosticsServiceClient(server.Client(), server.URL)
	authorization := "ArdentsOperatorSession " + base64.RawURLEncoding.EncodeToString(secret[:])
	directID, err := discoveryrecord.AccessResourceID("node", b2SignedNodeRecord(t).GetNodeFacts().GetPrincipal())
	require.NoError(t, err)
	directResource, err := identityaccess.NewResourceRef(node, identityaccess.ResourceOwner{}, "discovery-record", directID)
	require.NoError(t, err)
	directBinding := identityaccess.AuthenticationBinding{
		Audience:         identityaccess.Audience{Node: node, Interface: identityprotocol.Interface_INTERFACE_OPERATOR, ProtocolMajor: 1},
		TransportProfile: identityprotocol.TransportProfile_TRANSPORT_PROFILE_UNIX_LOCAL_V1, PeerBinding: peer,
	}
	_, err = service.Admit(context.Background(), identityaccess.Attempt{SessionSecret: secret, Binding: directBinding, Action: "discovery.resolve_record", Resource: directResource})
	require.NoError(t, err)
	call := func(request connect.AnyRequest, invoke func() error) {
		request.Header().Set("Authorization", authorization)
		beforeAdmission, beforeHandler := counter.calls.Load(), handler.calls
		require.NoError(t, invoke())
		require.Equal(t, beforeAdmission+1, counter.calls.Load())
		require.Equal(t, beforeHandler+1, handler.calls)
	}

	networkStatus := connect.NewRequest(&protocol.GetNetworkStatusRequest{})
	call(networkStatus, func() error { _, err := network.GetNetworkStatus(context.Background(), networkStatus); return err })
	discoveryStatus := connect.NewRequest(&protocol.GetDiscoveryStatusRequest{})
	call(discoveryStatus, func() error { _, err := network.GetDiscoveryStatus(context.Background(), discoveryStatus); return err })
	presence := connect.NewRequest(&protocol.GetLocalPresenceRequest{})
	call(presence, func() error { _, err := network.GetLocalPresence(context.Background(), presence); return err })
	peers := connect.NewRequest(&protocol.ListPeersRequest{})
	call(peers, func() error { _, err := network.ListPeers(context.Background(), peers); return err })
	routes := connect.NewRequest(&protocol.ListRouteCandidatesRequest{})
	call(routes, func() error { _, err := network.ListRouteCandidates(context.Background(), routes); return err })
	record := b2SignedNodeRecord(t)
	resolveRecord := connect.NewRequest(&protocol.ResolveRecordRequest{Subject: record.GetNodeFacts().GetPrincipal(), Kind: "node"})
	call(resolveRecord, func() error { _, err := network.ResolveRecord(context.Background(), resolveRecord); return err })
	resolveService := connect.NewRequest(&protocol.ResolveServiceRequest{Service: "svc.echo"})
	call(resolveService, func() error { _, err := network.ResolveService(context.Background(), resolveService); return err })
	records := connect.NewRequest(&protocol.ListRecordsRequest{})
	call(records, func() error { _, err := network.ListRecords(context.Background(), records); return err })
	importRecord := connect.NewRequest(&protocol.ImportRecordRequest{Record: record})
	call(importRecord, func() error { _, err := network.ImportRecord(context.Background(), importRecord); return err })
	snapshot := connect.NewRequest(&protocol.GetDiagnosticsRequest{})
	call(snapshot, func() error { _, err := diagnostics.GetDiagnostics(context.Background(), snapshot); return err })
	pending := connect.NewRequest(&protocol.GetPendingOperationsRequest{})
	call(pending, func() error { _, err := diagnostics.GetPendingOperations(context.Background(), pending); return err })
	health := connect.NewRequest(&protocol.GetHealthSummaryRequest{})
	call(health, func() error { _, err := diagnostics.GetHealthSummary(context.Background(), health); return err })
	explain := connect.NewRequest(&protocol.ExplainFailureRequest{Scope: "service", ResourceId: "svc.echo"})
	call(explain, func() error { _, err := diagnostics.ExplainFailure(context.Background(), explain); return err })
	events := connect.NewRequest(&protocol.ListRecentEventsRequest{Limit: 10})
	call(events, func() error { _, err := diagnostics.ListRecentEvents(context.Background(), events); return err })
	require.Equal(t, int32(len(actions)), counter.calls.Load())

	malformed := connect.NewRequest(&protocol.ResolveRecordRequest{Kind: "node"})
	malformed.Header().Set("Authorization", authorization)
	_, err = network.ResolveRecord(context.Background(), malformed)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	require.Equal(t, int32(len(actions)), counter.calls.Load())
}

func TestOperatorPrincipalInterceptorDeniesEveryB2SiblingAction(t *testing.T) {
	tests := []struct {
		grant   identityaccess.Action
		sibling string
	}{
		{"transport.network_status", ardentsv1connect.NetworkServiceListRouteCandidatesProcedure},
		{"transport.route_candidates", ardentsv1connect.NetworkServiceGetNetworkStatusProcedure},
		{"discovery.status", ardentsv1connect.NetworkServiceGetLocalPresenceProcedure},
		{"discovery.local_presence", ardentsv1connect.NetworkServiceListPeersProcedure},
		{"discovery.peers", ardentsv1connect.NetworkServiceListRecordsProcedure},
		{"discovery.list_records", ardentsv1connect.NetworkServiceGetDiscoveryStatusProcedure},
		{"discovery.resolve_record", ardentsv1connect.NetworkServiceResolveServiceProcedure},
		{"discovery.resolve_service", ardentsv1connect.NetworkServiceResolveRecordProcedure},
		{"discovery.import", ardentsv1connect.NetworkServiceListRecordsProcedure},
		{"diagnostics.snapshot", ardentsv1connect.DiagnosticsServiceGetHealthSummaryProcedure},
		{"diagnostics.health_summary", ardentsv1connect.DiagnosticsServiceGetDiagnosticsProcedure},
		{"diagnostics.pending_operations", ardentsv1connect.DiagnosticsServiceGetDiagnosticsProcedure},
		{"diagnostics.explain_failure", ardentsv1connect.DiagnosticsServiceListRecentEventsProcedure},
		{"diagnostics.recent_events", ardentsv1connect.DiagnosticsServiceExplainFailureProcedure},
	}
	for _, test := range tests {
		t.Run(string(test.grant), func(t *testing.T) {
			service, node, _, secret, peer, source, _ := operatorAccessFixtureWithActions(t, []identityaccess.Action{test.grant})
			counter := &countingAdmitter{service: service}
			interceptor, err := localidentity.NewOperatorInterceptor(localidentity.OperatorInterceptorConfig{
				Access: counter, Node: node, FallbackPeer: peer, FallbackSource: source, Canonicalize: operatorapi.CanonicalizeOperatorResource,
			})
			require.NoError(t, err)
			handler := &b2AdmissionGuardHandler{}
			mux := http.NewServeMux()
			networkPath, networkHTTP := ardentsv1connect.NewNetworkServiceHandler(handler, connect.WithInterceptors(interceptor))
			diagnosticsPath, diagnosticsHTTP := ardentsv1connect.NewDiagnosticsServiceHandler(handler, connect.WithInterceptors(interceptor))
			mux.Handle(networkPath, networkHTTP)
			mux.Handle(diagnosticsPath, diagnosticsHTTP)
			server := httptest.NewServer(mux)
			t.Cleanup(server.Close)
			network := ardentsv1connect.NewNetworkServiceClient(server.Client(), server.URL)
			diagnostics := ardentsv1connect.NewDiagnosticsServiceClient(server.Client(), server.URL)
			authorization := "ArdentsOperatorSession " + base64.RawURLEncoding.EncodeToString(secret[:])
			err = invokeB2Procedure(context.Background(), network, diagnostics, test.sibling, authorization, b2SignedNodeRecord(t))
			require.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
			require.Equal(t, int32(1), counter.calls.Load())
			require.Zero(t, handler.calls)
		})
	}
}

func invokeB2Procedure(ctx context.Context, network ardentsv1connect.NetworkServiceClient, diagnostics ardentsv1connect.DiagnosticsServiceClient, procedure, authorization string, record *protocol.DiscoveryRecord) error {
	setAuthorization := func(request connect.AnyRequest) { request.Header().Set("Authorization", authorization) }
	switch procedure {
	case ardentsv1connect.NetworkServiceGetNetworkStatusProcedure:
		request := connect.NewRequest(&protocol.GetNetworkStatusRequest{})
		setAuthorization(request)
		_, err := network.GetNetworkStatus(ctx, request)
		return err
	case ardentsv1connect.NetworkServiceGetDiscoveryStatusProcedure:
		request := connect.NewRequest(&protocol.GetDiscoveryStatusRequest{})
		setAuthorization(request)
		_, err := network.GetDiscoveryStatus(ctx, request)
		return err
	case ardentsv1connect.NetworkServiceGetLocalPresenceProcedure:
		request := connect.NewRequest(&protocol.GetLocalPresenceRequest{})
		setAuthorization(request)
		_, err := network.GetLocalPresence(ctx, request)
		return err
	case ardentsv1connect.NetworkServiceListPeersProcedure:
		request := connect.NewRequest(&protocol.ListPeersRequest{})
		setAuthorization(request)
		_, err := network.ListPeers(ctx, request)
		return err
	case ardentsv1connect.NetworkServiceListRouteCandidatesProcedure:
		request := connect.NewRequest(&protocol.ListRouteCandidatesRequest{})
		setAuthorization(request)
		_, err := network.ListRouteCandidates(ctx, request)
		return err
	case ardentsv1connect.NetworkServiceResolveRecordProcedure:
		request := connect.NewRequest(&protocol.ResolveRecordRequest{Kind: "node", Subject: record.GetNodeFacts().GetPrincipal()})
		setAuthorization(request)
		_, err := network.ResolveRecord(ctx, request)
		return err
	case ardentsv1connect.NetworkServiceResolveServiceProcedure:
		request := connect.NewRequest(&protocol.ResolveServiceRequest{Service: "svc.echo"})
		setAuthorization(request)
		_, err := network.ResolveService(ctx, request)
		return err
	case ardentsv1connect.NetworkServiceListRecordsProcedure:
		request := connect.NewRequest(&protocol.ListRecordsRequest{})
		setAuthorization(request)
		_, err := network.ListRecords(ctx, request)
		return err
	case ardentsv1connect.NetworkServiceImportRecordProcedure:
		request := connect.NewRequest(&protocol.ImportRecordRequest{Record: record})
		setAuthorization(request)
		_, err := network.ImportRecord(ctx, request)
		return err
	case ardentsv1connect.DiagnosticsServiceGetDiagnosticsProcedure:
		request := connect.NewRequest(&protocol.GetDiagnosticsRequest{})
		setAuthorization(request)
		_, err := diagnostics.GetDiagnostics(ctx, request)
		return err
	case ardentsv1connect.DiagnosticsServiceGetPendingOperationsProcedure:
		request := connect.NewRequest(&protocol.GetPendingOperationsRequest{})
		setAuthorization(request)
		_, err := diagnostics.GetPendingOperations(ctx, request)
		return err
	case ardentsv1connect.DiagnosticsServiceGetHealthSummaryProcedure:
		request := connect.NewRequest(&protocol.GetHealthSummaryRequest{})
		setAuthorization(request)
		_, err := diagnostics.GetHealthSummary(ctx, request)
		return err
	case ardentsv1connect.DiagnosticsServiceExplainFailureProcedure:
		request := connect.NewRequest(&protocol.ExplainFailureRequest{Scope: "service", ResourceId: "svc.echo"})
		setAuthorization(request)
		_, err := diagnostics.ExplainFailure(ctx, request)
		return err
	case ardentsv1connect.DiagnosticsServiceListRecentEventsProcedure:
		request := connect.NewRequest(&protocol.ListRecentEventsRequest{Limit: 10})
		setAuthorization(request)
		_, err := diagnostics.ListRecentEvents(ctx, request)
		return err
	default:
		return connect.NewError(connect.CodeInternal, context.Canceled)
	}
}

func b2SignedNodeRecord(t *testing.T) *protocol.DiscoveryRecord {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	principal, err := identityprincipal.FromEd25519PublicKey(public)
	require.NoError(t, err)
	now := time.Now().UTC().Truncate(time.Second)
	record := discoveryrecord.Record{
		Version: discoveryrecord.Version, Node: &discoveryrecord.NodeFacts{Principal: principal, PublicKey: base64.StdEncoding.EncodeToString(public)},
		IssuedAt: now, ExpiresAt: now.Add(time.Hour),
	}
	canonical, err := discoveryrecord.Canonical(record)
	require.NoError(t, err)
	record.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(private, canonical))
	return &protocol.DiscoveryRecord{
		Version: record.Version, Facts: &protocol.DiscoveryRecord_NodeFacts{NodeFacts: &protocol.NodeDiscoveryFacts{Principal: principal.String(), PublicKey: record.Node.PublicKey}},
		IssuedAtV1: timestamppb.New(record.IssuedAt), ExpiresAtV1: timestamppb.New(record.ExpiresAt), SignatureV1: record.Signature,
	}
}
