package authority

import (
	"testing"

	domain "ardents/internal/authority"

	"github.com/stretchr/testify/require"
)

func TestAuthorityRecoveryErrorPreservesOnlyStableAuthorityReason(t *testing.T) {
	for _, reason := range []string{
		domain.ReasonCheckpointMissing,
		domain.ReasonCheckpointMismatch,
		domain.ReasonCheckpointHistoryPartial,
		domain.ReasonCheckpointHistoryFork,
		domain.ReasonAuthorityRollback,
		domain.ReasonAuthorityGenerationMismatch,
		domain.ReasonPersistedStateInvalid,
		domain.ReasonSignerMismatch,
	} {
		t.Run(reason, func(t *testing.T) {
			got := authorityRecoveryError(
				"verify_restore", domain.ErrRecoveryRequired, reason,
			)
			require.Equal(t, reason, got.Reason)
		})
	}

	got := authorityRecoveryError(
		"verify_restore",
		domain.ErrRecoveryRequired,
		"operator@example /secret/path",
	)
	require.Equal(t, "authority_recovery_required", got.Reason)
}
