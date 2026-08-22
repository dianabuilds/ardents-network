package updatetransaction_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/release"
	"github.com/dianabuilds/ardents-network/internal/updatetransaction"
)

// TestApplyRejectsCallerConstructedDecision proves the public boundary: an
// accepted-looking Decision does not yield an authorization and therefore
// cannot make Update inspect or mutate an update root.
func TestApplyRejectsCallerConstructedDecision(t *testing.T) {
	forged := release.Decision{Outcome: release.OutcomeReleaseAccepted}
	authorization, ok := forged.Authorization()
	if ok {
		t.Fatal("caller-constructed decision yielded authorization")
	}
	result, err := updatetransaction.Apply(context.Background(), updatetransaction.Request{
		UpdateRoot: t.TempDir(), Authorization: authorization,
	})
	if err == nil || result.Outcome != "release-invalid" {
		t.Fatalf("forged authorization result = %+v, %v", result, err)
	}
}

// TestRequestHidesRawReleaseDecisions prevents a caller from supplying a
// forged release result for either the initial activation or rollback path.
func TestRequestHidesRawReleaseDecisions(t *testing.T) {
	requestType := reflect.TypeFor[updatetransaction.Request]()
	for _, name := range []string{"Decision", "RollbackDecision"} {
		if _, found := requestType.FieldByName(name); found {
			t.Fatalf("Request exposes raw %s", name)
		}
	}
}
