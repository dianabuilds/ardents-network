package processfixture

import (
	"errors"
	"fmt"
	"testing"
)

func TestFailureBranchesRequireOriginalCauseInEveryBranch(t *testing.T) {
	authorityFailure := errors.New("observed Network refusal")
	unrelated := errors.New("unrelated release failure")
	acceptOriginal := func(err error) bool { return err == authorityFailure }
	if !OnlyFailureBranches(errors.Join(authorityFailure, fmt.Errorf("original callback: %w", authorityFailure)), acceptOriginal) {
		t.Fatal("exact original Network refusal rejected")
	}
	for _, err := range []error{
		nil,
		unrelated,
		errors.Join(authorityFailure, unrelated),
		fmt.Errorf("original callback: %w", errors.Join(authorityFailure, unrelated)),
		errors.New(authorityFailure.Error()),
	} {
		if OnlyFailureBranches(err, acceptOriginal) {
			t.Fatalf("accepted unrelated or reconstructed cause: %v", err)
		}
	}
}
