package call

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"testing"

	identityaccess "ardents/internal/identity/access"
	identityprincipal "ardents/internal/identity/principal"

	"github.com/stretchr/testify/require"
)

func TestChannelRejectsZeroInvalidAndForeignAdmissions(t *testing.T) {
	injector, extractor := NewChannel()

	_, ok := extractor.Extract(context.Background())
	require.False(t, ok)
	ctx, injected := injector.WithAuthorizedCall(context.Background(), identityaccess.AuthorizedCall{}, "node", false)
	require.False(t, injected)
	_, ok = extractor.Extract(ctx)
	require.False(t, ok)
}

func TestSealedCallValidatesRegisteredOwnerRequiredAndOwnerlessResourceShapes(t *testing.T) {
	effective := testPrincipal(t, 0x31)
	other := testPrincipal(t, 0x32)
	effectiveOwner, err := identityaccess.ParseResourceOwner(effective)
	require.NoError(t, err)
	otherOwner, err := identityaccess.ParseResourceOwner(other)
	require.NoError(t, err)

	base := principalFacts{
		Actor: "actor", Effective: effective, Node: "node", Action: "application.content.get",
		ResourceNode: "node", ResourceKind: "owned-content", ResourceID: "content",
		ExpectedResourceKind: "owned-content", ExpectedResourceOwnerRequired: true,
	}
	base.ResourceOwner = effectiveOwner
	require.True(t, (Call{principal: &base}).IsAdmitted())

	ownerMismatch := base
	ownerMismatch.ResourceOwner = otherOwner
	require.False(t, (Call{principal: &ownerMismatch}).IsAdmitted())

	ownerMissing := base
	ownerMissing.ResourceOwner = identityaccess.ResourceOwner{}
	require.False(t, (Call{principal: &ownerMissing}).IsAdmitted())

	ownerless := base
	ownerless.ResourceKind = "node"
	ownerless.ExpectedResourceKind = "node"
	ownerless.ExpectedResourceOwnerRequired = false
	ownerless.ResourceID = ""
	ownerless.ResourceOwner = identityaccess.ResourceOwner{}
	require.True(t, (Call{principal: &ownerless}).IsAdmitted())

	ownerlessWithOwner := ownerless
	ownerlessWithOwner.ResourceOwner = effectiveOwner
	require.False(t, (Call{principal: &ownerlessWithOwner}).IsAdmitted())

	unknownKind := ownerless
	unknownKind.ResourceKind = "test-only-unknown"
	require.False(t, (Call{principal: &unknownKind}).IsAdmitted())
}

func testPrincipal(t *testing.T, marker byte) string {
	t.Helper()
	key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{marker}, ed25519.SeedSize))
	principal, err := identityprincipal.FromEd25519PublicKey(key.Public().(ed25519.PublicKey))
	require.NoError(t, err)
	return principal.String()
}
