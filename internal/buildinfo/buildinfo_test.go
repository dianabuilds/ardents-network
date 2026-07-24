package buildinfo

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCurrentIncludesRuntimeAndInjectedIdentity(t *testing.T) {
	previousVersion, previousCommit, previousDate := Version, Commit, BuildDate
	t.Cleanup(func() { Version, Commit, BuildDate = previousVersion, previousCommit, previousDate })
	Version, Commit, BuildDate = "v1.2.3", "abc123", "2026-07-20T00:00:00Z"

	info := Current()
	require.Equal(t, "v1.2.3", info.Version)
	require.Equal(t, "abc123", info.Commit)
	require.Equal(t, "2026-07-20T00:00:00Z", info.BuildDate)
	require.Equal(t, runtime.Version(), info.GoVersion)
	require.NotEmpty(t, info.OS)
	require.NotEmpty(t, info.Arch)
}

func TestFingerprintMatchesVersionJSON(t *testing.T) {
	encoded, err := json.Marshal(Current())
	require.NoError(t, err)
	sum := sha256.Sum256(encoded)

	require.Equal(t, hex.EncodeToString(sum[:]), Fingerprint())
}
