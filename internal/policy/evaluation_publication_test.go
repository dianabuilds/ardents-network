package policy

import (
	"testing"

	hostingservice "ardents/internal/workload/registry"

	"github.com/stretchr/testify/require"
)

func TestServicePublicationDecision(t *testing.T) {
	result := CheckServicePublication(ServicePublicationConfig{
		DisableNetworkPublishedServices: true,
	}, hostingservice.ServiceSpec{ID: "svc.echo", Type: "echo", Mode: "NetworkPublished"})
	require.False(t, result.Allowed, "expected denial for network-published service")
	require.Equal(t, "policy_publication_denied", result.Reason.Code)
}
