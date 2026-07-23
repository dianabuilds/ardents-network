package access_test

import (
	"testing"

	contentcatalog "ardents/internal/content/catalog"
	"ardents/internal/hosting"
	identityaccess "ardents/internal/identity/access"
	identityprincipal "ardents/internal/identity/principal"
	"ardents/internal/workload"

	"github.com/stretchr/testify/require"
)

func TestCanonicalNonPrincipalIdentifiersCannotCrossPrincipalBoundaries(t *testing.T) {
	const cidText = "bafkreibm6jg3ux5qumhcn2b3flc3tyu6dmlb4xa7u5bf44yegnrjhc4yeq"
	const peerID = "12D3KooWPFH2Bx2tPfw6RLxN8k2wh47GRXgkt9yrAHU37zFwHWzS"
	reference, err := contentcatalog.ParseContentReference(cidText)
	require.NoError(t, err)
	workloadID, err := workload.AccessResourceID("work.echo")
	require.NoError(t, err)
	serviceID, err := hosting.ServiceAccessResourceID("svc.echo")
	require.NoError(t, err)
	for name, value := range map[string]string{
		"CID": reference.String(), "PeerID": peerID,
		"WorkloadID": workloadID, "ServiceID": serviceID,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := identityprincipal.Parse(value)
			require.Error(t, err)
			_, err = identityaccess.ParseResourceOwner(value)
			require.ErrorIs(t, err, identityaccess.ErrInvalidArgument)
		})
	}
}
