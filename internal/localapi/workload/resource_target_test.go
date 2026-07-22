package workload

import (
	"testing"

	identityaccess "ardents/internal/identity/access"
	protocol "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/protocol/ardentsv1connect"

	"github.com/stretchr/testify/require"
)

func TestCanonicalizeResourceUsesExactWorkloadAndServiceIDs(t *testing.T) {
	tests := []struct {
		name      string
		procedure string
		message   any
		kind      identityaccess.ResourceKind
		id        string
	}{
		{"register", ardentsv1connect.WorkloadServiceRegisterWorkloadProcedure, &protocol.RegisterWorkloadRequest{Spec: &protocol.WorkloadSpecSnapshot{Id: "work.echo", Owner: "client-claim-must-not-be-authority"}}, "workload", "work.echo"},
		{"start", ardentsv1connect.WorkloadServiceStartWorkloadProcedure, &protocol.StartWorkloadRequest{Id: "work.echo"}, "workload", "work.echo"},
		{"stop", ardentsv1connect.WorkloadServiceStopWorkloadProcedure, &protocol.StopWorkloadRequest{Id: "work.echo"}, "workload", "work.echo"},
		{"restart", ardentsv1connect.WorkloadServiceRestartWorkloadProcedure, &protocol.RestartWorkloadRequest{Id: "work.echo"}, "workload", "work.echo"},
		{"status", ardentsv1connect.WorkloadServiceGetWorkloadStatusProcedure, &protocol.GetWorkloadStatusRequest{Id: "work.echo"}, "workload", "work.echo"},
		{"list", ardentsv1connect.WorkloadServiceListWorkloadsProcedure, &protocol.ListWorkloadsRequest{}, "workload-collection", ""},
		{"hosted service", ardentsv1connect.WorkloadServiceGetHostedServiceProcedure, &protocol.GetHostedServiceRequest{Id: "svc.echo"}, "service", "svc.echo"},
		{"hosted services", ardentsv1connect.WorkloadServiceListHostedServicesProcedure, &protocol.ListHostedServicesRequest{}, "service-collection", ""},
		{"publication", ardentsv1connect.WorkloadServiceGetServicePublicationStatusProcedure, &protocol.GetServicePublicationStatusRequest{Id: "svc.echo"}, "service", "svc.echo"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			target, err := CanonicalizeResource(test.procedure, test.message, test.kind)
			require.NoError(t, err)
			require.Equal(t, test.kind, target.Kind)
			require.Equal(t, test.id, target.ID)
		})
	}
}

func TestCanonicalizeResourceRejectsMalformedBeforeAdmission(t *testing.T) {
	tests := []struct {
		procedure string
		message   any
	}{
		{ardentsv1connect.WorkloadServiceRegisterWorkloadProcedure, &protocol.RegisterWorkloadRequest{}},
		{ardentsv1connect.WorkloadServiceRegisterWorkloadProcedure, &protocol.RegisterWorkloadRequest{Spec: &protocol.WorkloadSpecSnapshot{}}},
		{ardentsv1connect.WorkloadServiceStartWorkloadProcedure, &protocol.StartWorkloadRequest{Id: " work.echo"}},
		{ardentsv1connect.WorkloadServiceGetHostedServiceProcedure, &protocol.GetHostedServiceRequest{}},
		{ardentsv1connect.WorkloadServiceGetServicePublicationStatusProcedure, &protocol.GetServicePublicationStatusRequest{Id: "svc.echo\n"}},
		{ardentsv1connect.WorkloadServiceListWorkloadsProcedure, &protocol.ListHostedServicesRequest{}},
		{"/ardents.v1.WorkloadService/Unknown", &protocol.ListWorkloadsRequest{}},
	}
	for _, test := range tests {
		_, err := CanonicalizeResource(test.procedure, test.message, "workload")
		require.ErrorIs(t, err, ErrInvalidResourceTarget)
	}
}
