package evaluation

import (
	"testing"

	domainnetwork "ardents/internal/network/api"

	"github.com/stretchr/testify/require"
)

func TestCheckRouteUse(t *testing.T) {
	allowed := CheckRouteUse(RouteConfig{DeniedRouteSchemes: []string{"quic"}}, domainnetwork.Candidate{Scheme: "tcp", Trusted: true})
	require.True(t, allowed.Allowed, "expected tcp route to remain allowed")

	denied := CheckRouteUse(RouteConfig{DeniedRouteSchemes: []string{"quic"}}, domainnetwork.Candidate{Scheme: "quic", Trusted: true})
	require.False(t, denied.Allowed, "expected quic route denial")
	require.Equal(t, "policy_route_denied", denied.Reason.Code)
}
