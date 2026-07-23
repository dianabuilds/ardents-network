package identity_test

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

type b3AdmissionGuardHandler struct {
	ardentsv1connect.UnimplementedWorkloadServiceHandler
	calls int32
}

func (h *b3AdmissionGuardHandler) admitted(ctx context.Context) error {
	call, ok := rpc.CallFromContext(ctx)
	if !ok || call.Actor() == "" || call.Effective() == "" {
		return connect.NewError(connect.CodeUnauthenticated, context.Canceled)
	}
	h.calls++
	return nil
}

func (h *b3AdmissionGuardHandler) RegisterWorkload(ctx context.Context, _ *connect.Request[protocol.RegisterWorkloadRequest]) (*connect.Response[protocol.WorkloadCommandResponse], error) {
	return connect.NewResponse(&protocol.WorkloadCommandResponse{}), h.admitted(ctx)
}

func (h *b3AdmissionGuardHandler) StartWorkload(ctx context.Context, _ *connect.Request[protocol.StartWorkloadRequest]) (*connect.Response[protocol.WorkloadCommandResponse], error) {
	return connect.NewResponse(&protocol.WorkloadCommandResponse{}), h.admitted(ctx)
}

func (h *b3AdmissionGuardHandler) StopWorkload(ctx context.Context, _ *connect.Request[protocol.StopWorkloadRequest]) (*connect.Response[protocol.WorkloadCommandResponse], error) {
	return connect.NewResponse(&protocol.WorkloadCommandResponse{}), h.admitted(ctx)
}

func (h *b3AdmissionGuardHandler) RestartWorkload(ctx context.Context, _ *connect.Request[protocol.RestartWorkloadRequest]) (*connect.Response[protocol.WorkloadCommandResponse], error) {
	return connect.NewResponse(&protocol.WorkloadCommandResponse{}), h.admitted(ctx)
}

func (h *b3AdmissionGuardHandler) GetWorkloadStatus(ctx context.Context, _ *connect.Request[protocol.GetWorkloadStatusRequest]) (*connect.Response[protocol.WorkloadStatusSnapshot], error) {
	return connect.NewResponse(&protocol.WorkloadStatusSnapshot{}), h.admitted(ctx)
}

func (h *b3AdmissionGuardHandler) ListWorkloads(ctx context.Context, _ *connect.Request[protocol.ListWorkloadsRequest]) (*connect.Response[protocol.ListWorkloadsResponse], error) {
	return connect.NewResponse(&protocol.ListWorkloadsResponse{}), h.admitted(ctx)
}

func (h *b3AdmissionGuardHandler) GetHostedService(ctx context.Context, _ *connect.Request[protocol.GetHostedServiceRequest]) (*connect.Response[protocol.GetHostedServiceResponse], error) {
	return connect.NewResponse(&protocol.GetHostedServiceResponse{}), h.admitted(ctx)
}

func (h *b3AdmissionGuardHandler) ListHostedServices(ctx context.Context, _ *connect.Request[protocol.ListHostedServicesRequest]) (*connect.Response[protocol.ListHostedServicesResponse], error) {
	return connect.NewResponse(&protocol.ListHostedServicesResponse{}), h.admitted(ctx)
}

func (h *b3AdmissionGuardHandler) GetServicePublicationStatus(ctx context.Context, _ *connect.Request[protocol.GetServicePublicationStatusRequest]) (*connect.Response[protocol.ServicePublicationStatusResponse], error) {
	return connect.NewResponse(&protocol.ServicePublicationStatusResponse{}), h.admitted(ctx)
}

func TestOperatorPrincipalInterceptorGuardsEveryB3ProcedureExactlyOnce(t *testing.T) {
	actions := []identityaccess.Action{
		"workload.register", "workload.start", "workload.stop", "workload.restart", "workload.status",
		"workload.list", "workload.hosted_service", "workload.hosted_services", "workload.service_publication",
	}
	service, node, _, secret, peer, source, _ := operatorAccessFixtureWithActions(t, actions)
	counter := &countingAdmitter{service: service}
	interceptor, err := localidentity.NewOperatorInterceptor(localidentity.OperatorInterceptorConfig{
		Access: counter, Node: node, FallbackPeer: peer, FallbackSource: source, Canonicalize: operatorapi.CanonicalizeOperatorResource,
	})
	require.NoError(t, err)
	handler := &b3AdmissionGuardHandler{}
	path, httpHandler := ardentsv1connect.NewWorkloadServiceHandler(handler, connect.WithInterceptors(interceptor))
	mux := http.NewServeMux()
	mux.Handle(path, httpHandler)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	client := ardentsv1connect.NewWorkloadServiceClient(server.Client(), server.URL)
	authorization := "ArdentsOperatorSession " + base64.RawURLEncoding.EncodeToString(secret[:])
	procedures := []string{
		ardentsv1connect.WorkloadServiceRegisterWorkloadProcedure,
		ardentsv1connect.WorkloadServiceStartWorkloadProcedure,
		ardentsv1connect.WorkloadServiceStopWorkloadProcedure,
		ardentsv1connect.WorkloadServiceRestartWorkloadProcedure,
		ardentsv1connect.WorkloadServiceGetWorkloadStatusProcedure,
		ardentsv1connect.WorkloadServiceListWorkloadsProcedure,
		ardentsv1connect.WorkloadServiceGetHostedServiceProcedure,
		ardentsv1connect.WorkloadServiceListHostedServicesProcedure,
		ardentsv1connect.WorkloadServiceGetServicePublicationStatusProcedure,
	}
	for _, procedure := range procedures {
		beforeAdmission, beforeHandler := counter.calls.Load(), handler.calls
		require.NoError(t, invokeB3Procedure(context.Background(), client, procedure, authorization))
		require.Equal(t, beforeAdmission+1, counter.calls.Load(), procedure)
		require.Equal(t, beforeHandler+1, handler.calls, procedure)
	}
	require.Equal(t, int32(len(actions)), counter.calls.Load())

	malformed := connect.NewRequest(&protocol.StartWorkloadRequest{})
	malformed.Header().Set("Authorization", authorization)
	_, err = client.StartWorkload(context.Background(), malformed)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	require.Equal(t, int32(len(actions)), counter.calls.Load())
	require.Equal(t, int32(len(actions)), handler.calls)
}

func TestOperatorPrincipalInterceptorDeniesEveryB3SiblingAction(t *testing.T) {
	tests := []struct {
		grant   identityaccess.Action
		sibling string
	}{
		{"workload.register", ardentsv1connect.WorkloadServiceStartWorkloadProcedure},
		{"workload.start", ardentsv1connect.WorkloadServiceStopWorkloadProcedure},
		{"workload.stop", ardentsv1connect.WorkloadServiceRestartWorkloadProcedure},
		{"workload.restart", ardentsv1connect.WorkloadServiceGetWorkloadStatusProcedure},
		{"workload.status", ardentsv1connect.WorkloadServiceListWorkloadsProcedure},
		{"workload.list", ardentsv1connect.WorkloadServiceGetHostedServiceProcedure},
		{"workload.hosted_service", ardentsv1connect.WorkloadServiceListHostedServicesProcedure},
		{"workload.hosted_services", ardentsv1connect.WorkloadServiceGetServicePublicationStatusProcedure},
		{"workload.service_publication", ardentsv1connect.WorkloadServiceRegisterWorkloadProcedure},
	}
	for _, test := range tests {
		t.Run(string(test.grant), func(t *testing.T) {
			service, node, _, secret, peer, source, _ := operatorAccessFixtureWithActions(t, []identityaccess.Action{test.grant})
			counter := &countingAdmitter{service: service}
			interceptor, err := localidentity.NewOperatorInterceptor(localidentity.OperatorInterceptorConfig{
				Access: counter, Node: node, FallbackPeer: peer, FallbackSource: source, Canonicalize: operatorapi.CanonicalizeOperatorResource,
			})
			require.NoError(t, err)
			handler := &b3AdmissionGuardHandler{}
			path, httpHandler := ardentsv1connect.NewWorkloadServiceHandler(handler, connect.WithInterceptors(interceptor))
			mux := http.NewServeMux()
			mux.Handle(path, httpHandler)
			server := httptest.NewServer(mux)
			t.Cleanup(server.Close)
			client := ardentsv1connect.NewWorkloadServiceClient(server.Client(), server.URL)
			authorization := "ArdentsOperatorSession " + base64.RawURLEncoding.EncodeToString(secret[:])
			err = invokeB3Procedure(context.Background(), client, test.sibling, authorization)
			require.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
			require.Equal(t, int32(1), counter.calls.Load())
			require.Zero(t, handler.calls)
		})
	}
}

func TestOperatorPrincipalInterceptorDeniesB3SiblingExactResources(t *testing.T) {
	tests := []struct {
		name   string
		action identityaccess.Action
		kind   identityaccess.ResourceKind
		match  string
		other  string
	}{
		{"workload", "workload.status", "workload", "work.echo", "work.other"},
		{"service", "workload.hosted_service", "service", "svc.echo", "svc.other"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service, node, principal, secret, peer, source, _ := operatorAccessFixtureWithActions(t, []identityaccess.Action{"node.status"})
			issueB3ExactGrant(t, service, node, principal, secret, peer, test.action, test.kind, test.match)
			counter := &countingAdmitter{service: service}
			interceptor, err := localidentity.NewOperatorInterceptor(localidentity.OperatorInterceptorConfig{
				Access: counter, Node: node, FallbackPeer: peer, FallbackSource: source, Canonicalize: operatorapi.CanonicalizeOperatorResource,
			})
			require.NoError(t, err)
			handler := &b3AdmissionGuardHandler{}
			path, httpHandler := ardentsv1connect.NewWorkloadServiceHandler(handler, connect.WithInterceptors(interceptor))
			mux := http.NewServeMux()
			mux.Handle(path, httpHandler)
			server := httptest.NewServer(mux)
			t.Cleanup(server.Close)
			client := ardentsv1connect.NewWorkloadServiceClient(server.Client(), server.URL)
			authorization := "ArdentsOperatorSession " + base64.RawURLEncoding.EncodeToString(secret[:])

			require.NoError(t, invokeB3ExactProcedure(context.Background(), client, test.action, test.match, authorization))
			require.Equal(t, int32(1), counter.calls.Load())
			require.Equal(t, int32(1), handler.calls)

			err = invokeB3ExactProcedure(context.Background(), client, test.action, test.other, authorization)
			require.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
			require.Equal(t, int32(2), counter.calls.Load())
			require.Equal(t, int32(1), handler.calls)
		})
	}
}

func issueB3ExactGrant(t *testing.T, service *identityaccess.Service, node, principal string, secret identityaccess.SessionSecret, peer [32]byte, action identityaccess.Action, kind identityaccess.ResourceKind, id string) {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	binding := identityaccess.AuthenticationBinding{
		Audience:         identityaccess.Audience{Node: node, Interface: identityprotocol.Interface_INTERFACE_OPERATOR, ProtocolMajor: 1},
		TransportProfile: identityprotocol.TransportProfile_TRANSPORT_PROFILE_UNIX_LOCAL_V1,
		PeerBinding:      peer,
	}
	exact, err := identityaccess.NewResourceRef(node, identityaccess.ResourceOwner{}, string(kind), id)
	require.NoError(t, err)
	proposal := identityaccess.GrantProposal{
		Subject:   principal,
		Actions:   []identityaccess.Action{action},
		Scope:     identityaccess.ResourceScope{Kind: identityaccess.ScopeExact, Exact: exact},
		NotBefore: now,
		NotAfter:  now.Add(time.Hour),
	}
	proposalID, err := identityaccess.GrantProposalResourceID(node, binding.Audience, proposal)
	require.NoError(t, err)
	resource, err := identityaccess.NewResourceRef(node, identityaccess.ResourceOwner{}, "grant-proposal", proposalID)
	require.NoError(t, err)
	_, err = service.IssueAccessGrant(context.Background(), identityaccess.IssueGrantRequest{
		Command: identityaccess.AdminCommand{
			RequestID: "b3-exact-" + testResourceID(t, id),
			Attempt:   identityaccess.Attempt{SessionSecret: secret, Binding: binding, Action: "identity.grant.issue", Resource: resource},
		},
		Proposal: proposal,
	})
	require.NoError(t, err)
}

func testResourceID(t *testing.T, id string) string {
	t.Helper()
	return base64.RawURLEncoding.EncodeToString([]byte(id))
}

func invokeB3ExactProcedure(ctx context.Context, client ardentsv1connect.WorkloadServiceClient, action identityaccess.Action, id, authorization string) error {
	switch action {
	case "workload.status":
		request := connect.NewRequest(&protocol.GetWorkloadStatusRequest{Id: id})
		request.Header().Set("Authorization", authorization)
		_, err := client.GetWorkloadStatus(ctx, request)
		return err
	case "workload.hosted_service":
		request := connect.NewRequest(&protocol.GetHostedServiceRequest{Id: id})
		request.Header().Set("Authorization", authorization)
		_, err := client.GetHostedService(ctx, request)
		return err
	default:
		return connect.NewError(connect.CodeInternal, context.Canceled)
	}
}

func invokeB3Procedure(ctx context.Context, client ardentsv1connect.WorkloadServiceClient, procedure, authorization string) error {
	setAuthorization := func(request connect.AnyRequest) { request.Header().Set("Authorization", authorization) }
	switch procedure {
	case ardentsv1connect.WorkloadServiceRegisterWorkloadProcedure:
		request := connect.NewRequest(&protocol.RegisterWorkloadRequest{Spec: &protocol.WorkloadSpecSnapshot{Id: "work.echo"}})
		setAuthorization(request)
		_, err := client.RegisterWorkload(ctx, request)
		return err
	case ardentsv1connect.WorkloadServiceStartWorkloadProcedure:
		request := connect.NewRequest(&protocol.StartWorkloadRequest{Id: "work.echo"})
		setAuthorization(request)
		_, err := client.StartWorkload(ctx, request)
		return err
	case ardentsv1connect.WorkloadServiceStopWorkloadProcedure:
		request := connect.NewRequest(&protocol.StopWorkloadRequest{Id: "work.echo"})
		setAuthorization(request)
		_, err := client.StopWorkload(ctx, request)
		return err
	case ardentsv1connect.WorkloadServiceRestartWorkloadProcedure:
		request := connect.NewRequest(&protocol.RestartWorkloadRequest{Id: "work.echo"})
		setAuthorization(request)
		_, err := client.RestartWorkload(ctx, request)
		return err
	case ardentsv1connect.WorkloadServiceGetWorkloadStatusProcedure:
		request := connect.NewRequest(&protocol.GetWorkloadStatusRequest{Id: "work.echo"})
		setAuthorization(request)
		_, err := client.GetWorkloadStatus(ctx, request)
		return err
	case ardentsv1connect.WorkloadServiceListWorkloadsProcedure:
		request := connect.NewRequest(&protocol.ListWorkloadsRequest{})
		setAuthorization(request)
		_, err := client.ListWorkloads(ctx, request)
		return err
	case ardentsv1connect.WorkloadServiceGetHostedServiceProcedure:
		request := connect.NewRequest(&protocol.GetHostedServiceRequest{Id: "svc.echo"})
		setAuthorization(request)
		_, err := client.GetHostedService(ctx, request)
		return err
	case ardentsv1connect.WorkloadServiceListHostedServicesProcedure:
		request := connect.NewRequest(&protocol.ListHostedServicesRequest{})
		setAuthorization(request)
		_, err := client.ListHostedServices(ctx, request)
		return err
	case ardentsv1connect.WorkloadServiceGetServicePublicationStatusProcedure:
		request := connect.NewRequest(&protocol.GetServicePublicationStatusRequest{Id: "svc.echo"})
		setAuthorization(request)
		_, err := client.GetServicePublicationStatus(ctx, request)
		return err
	default:
		return connect.NewError(connect.CodeInternal, context.Canceled)
	}
}
