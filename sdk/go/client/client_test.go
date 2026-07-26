package client

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"ardents/sdk/go/discovery"
	"ardents/sdk/go/identity"

	"github.com/stretchr/testify/require"
)

func TestClientRequiresPrincipalSessionConfiguration(t *testing.T) {
	_, err := New(Config{Signer: &clientSignerStub{}, NodePrincipal: canonicalNodePrincipal})
	require.ErrorContains(t, err, "protected Unix socket")

	_, err = New(Config{SocketPath: filepath.Join(t.TempDir(), "application.sock"), NodePrincipal: canonicalNodePrincipal})
	require.ErrorContains(t, err, "session signer is required")
}

func TestClientRejectsNoncanonicalNodePrincipalBeforeTransportSetup(t *testing.T) {
	for _, node := range []string{"p1_invalid", " " + canonicalNodePrincipal, canonicalNodePrincipal + "\t"} {
		_, err := New(Config{
			SocketPath: filepath.Join(t.TempDir(), "application.sock"),
			Signer:     &clientSignerStub{}, NodePrincipal: node,
		})
		require.ErrorContains(t, err, "canonical Node Principal")
	}
}

func TestClientExposesTypedDiscoveryService(t *testing.T) {
	field, ok := reflect.TypeFor[Client]().FieldByName("Discovery")
	require.True(t, ok)
	require.Equal(t, reflect.TypeFor[discovery.Service](), field.Type)
}

const canonicalNodePrincipal = "p1_755gnz2wffu3osamddsj7ggiasqtwnwomsooe5mxh2yipr2urmwq"

type clientSignerStub struct{}

func (*clientSignerStub) Principal(context.Context) (string, error)              { return "", nil }
func (*clientSignerStub) Credential(context.Context) (*identity.Artifact, error) { return nil, nil }
func (*clientSignerStub) SignAuthenticationChallenge(context.Context, identity.Challenge) ([]byte, error) {
	return nil, nil
}
func (*clientSignerStub) SignEnrollmentChallenge(context.Context, identity.Challenge) ([]byte, error) {
	return nil, nil
}
