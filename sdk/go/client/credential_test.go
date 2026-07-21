package client

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFileCredentialReadsProvisionedApplicationToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "application-token")
	require.NoError(t, os.WriteFile(path, []byte("application-secret\n"), 0o640))
	credential, err := FileCredential(path)
	require.NoError(t, err)
	token, err := credential.Credential(context.Background())
	require.NoError(t, err)
	require.Equal(t, "application-secret", token)
}
