package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"ardents/internal/buildinfo"

	"github.com/stretchr/testify/require"
)

func TestVersionDoesNotRequireNodeOrCredential(t *testing.T) {
	previousVersion, previousCommit, previousDate := buildinfo.Version, buildinfo.Commit, buildinfo.BuildDate
	t.Cleanup(func() {
		buildinfo.Version, buildinfo.Commit, buildinfo.BuildDate = previousVersion, previousCommit, previousDate
	})
	buildinfo.Version, buildinfo.Commit, buildinfo.BuildDate = "v1.2.3", "abc123", "2026-07-20T00:00:00Z"
	t.Setenv("ARDENTS_API_TOKEN", "")
	t.Setenv("ARDENTS_API_TOKEN_FILE", "")
	t.Setenv("ARDENTS_ADDR", "127.0.0.1:1")

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"--output", "json", "version"}, &stdout, &stderr)
	require.Zero(t, code, stderr.String())
	require.Empty(t, stderr.String())
	var info buildinfo.Info
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &info))
	require.Equal(t, "v1.2.3", info.Version)
	require.Equal(t, "abc123", info.Commit)
	require.Equal(t, "2026-07-20T00:00:00Z", info.BuildDate)
}

func TestVersionHumanOutputContainsTarget(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"version"}, &stdout, &stderr)
	require.Zero(t, code, stderr.String())
	require.Contains(t, stdout.String(), "ardentsctl ")
	require.Contains(t, stdout.String(), buildinfo.Current().OS+"/"+buildinfo.Current().Arch)
}
