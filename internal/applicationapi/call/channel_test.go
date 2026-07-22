package call

import (
	"context"
	"testing"

	identityaccess "ardents/internal/identity/access"

	"github.com/stretchr/testify/require"
)

func TestChannelRejectsZeroInvalidAndForeignAdmissions(t *testing.T) {
	injector, extractor := NewChannel()

	_, ok := extractor.Extract(context.Background())
	require.False(t, ok)
	_, ok = extractor.Extract(injector.WithAuthorizedCall(context.Background(), identityaccess.AuthorizedCall{}))
	require.False(t, ok)
}
