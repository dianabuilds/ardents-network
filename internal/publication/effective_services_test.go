package publication

import (
	"errors"
	"testing"

	"ardents/internal/workload/execution"
	"ardents/internal/workload/registry"

	"github.com/stretchr/testify/require"
)

func TestEffectivePublishedServicesRemainsConsumerOwned(t *testing.T) {
	status := execution.Status{PublishedServices: []execution.PublishedServiceStatus{{
		ID: "svc.echo", Type: "echo", Published: true,
	}}}
	status.PublishedServices = EffectivePublishedServices(status.PublishedServices, func(registry.ServiceSpec) error {
		return errors.New("policy_publication_denied: service type is denied by policy")
	})
	require.False(t, status.PublishedServices[0].Published, "expected denial to clear publication truth")
	require.NotEmpty(t, status.PublishedServices[0].Reason, "expected policy denial reason")
}
