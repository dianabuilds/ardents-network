package authority

import (
	"testing"

	domain "ardents/internal/authority"

	"github.com/stretchr/testify/require"
)

func TestAuthorityRecoveryErrorPreservesOnlyStableAuthorityReason(t *testing.T) {
	for _, reason := range []string{
		domain.ReasonRepositoryUnavailable,
		domain.ReasonSignerUnavailable,
		domain.ReasonStoreUnavailable,
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
			domainError := domain.ErrRecoveryRequired
			if reason == domain.ReasonRepositoryUnavailable ||
				reason == domain.ReasonSignerUnavailable ||
				reason == domain.ReasonStoreUnavailable {
				domainError = domain.ErrUnavailable
			}
			got := authorityRecoveryError("verify_restore", domainError, reason)
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
