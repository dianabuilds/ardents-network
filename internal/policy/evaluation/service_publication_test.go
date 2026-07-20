package evaluation

import (
	"errors"
	"testing"

	hostingexposure "ardents/internal/hosting/exposure"
	hostingservice "ardents/internal/hosting/service"
	"ardents/internal/workload/observedstate"
	domainworkload "ardents/internal/workload/workload"

	"github.com/stretchr/testify/require"
)

func TestServicePublicationDecision(t *testing.T) {
	result := CheckServicePublication(ServicePublicationConfig{
		DisableNetworkPublishedServices: true,
	}, hostingservice.Spec{ID: "svc.echo", Type: "echo", Mode: "NetworkPublished"})
	require.False(t, result.Allowed, "expected denial for network-published service")
	require.Equal(t, "policy_publication_denied", result.Reason.Code)
}

func TestEffectivePublishedServicesRemainsConsumerOwned(t *testing.T) {
	status := observedstate.Status{
		PublishedServices: []observedstate.PublishedServiceStatus{{
			ID:        "svc.echo",
			Type:      "echo",
			Published: true,
		}},
	}
	status.PublishedServices = hostingexposure.EffectivePublishedServices(status.PublishedServices, func(domainworkload.ServiceSpec) error {
		return errors.New("policy_publication_denied: service type is denied by policy")
	})
	require.False(t, status.PublishedServices[0].Published, "expected denial to clear publication truth")
	require.NotEmpty(t, status.PublishedServices[0].Reason, "expected policy denial reason")
}
