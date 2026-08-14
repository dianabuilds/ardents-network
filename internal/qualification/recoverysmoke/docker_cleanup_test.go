package recoverysmoke

import (
	"errors"
	"strings"
	"testing"
)

func TestCleanupFailurePreservesCampaignFailure(t *testing.T) {
	cleanupErr := errors.New("reset recovery topology: Docker unavailable")
	reason := cleanupFailureReason("replacement recovery missed five seconds", cleanupErr)
	for _, wanted := range []string{"replacement recovery missed five seconds", cleanupErr.Error()} {
		if !strings.Contains(reason, wanted) {
			t.Fatalf("combined reason %q lacks %q", reason, wanted)
		}
	}
	if got := cleanupFailureReason("", cleanupErr); got != cleanupErr.Error() {
		t.Fatalf("cleanup-only reason = %q", got)
	}
}
