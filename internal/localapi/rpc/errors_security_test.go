package rpc

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMapAPIErrorDoesNotExposeInternalErrorOrCredential(t *testing.T) {
	apiErr := MapError(
		"data",
		"data.fetch_blob",
		"fetch_failed",
		"data fetch blob failed",
		true,
		errors.New("upstream rejected Authorization: Bearer secret-token"),
	)

	require.Equal(t, "data fetch blob failed", apiErr.Message)
	require.Equal(t, "fetch_failed", apiErr.Reason)
	require.NotContains(t, apiErr.Message, "secret-token")
	require.NotContains(t, apiErr.Reason, "secret-token")
}
