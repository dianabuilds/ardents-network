package daemon

import (
	"net/http"
	"testing"

	identityaccess "ardents/internal/identity/access"

	"github.com/stretchr/testify/require"
)

func TestApplicationPrincipalConfigurationUsesOnlyProtectedListener(t *testing.T) {
	owners := NewOwners(Config{Name: "application-principal-test", Data: DataConfig{Dir: t.TempDir()}})
	owners.PrincipalAccess = new(identityaccess.Service)
	cfg := runtimeConfig{ApplicationSocketPath: "C:/protected/application.sock"}
	var observed []ApplicationAPIConfig
	factory := func(_ Owners, config ApplicationAPIConfig) (string, http.Handler, error) {
		observed = append(observed, config)
		return "/application", http.NotFoundHandler(), nil
	}

	_, _, err := newProtectedApplicationHandler(owners, cfg, factory)
	require.NoError(t, err)
	require.Len(t, observed, 1)
	require.True(t, observed[0].Protected)
	require.NotZero(t, observed[0].PeerBinding)
	require.NotZero(t, observed[0].Source)
}
