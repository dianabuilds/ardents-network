package claim

import "testing"

func TestOpenClaimWinnerRejectsIncompleteClose(t *testing.T) {
	t.Parallel()
	if _, err := OpenClaimWinner(ClaimOrder{}, ClaimProof{}); err == nil {
		t.Fatal("incomplete claim close produced a winner")
	}
}
