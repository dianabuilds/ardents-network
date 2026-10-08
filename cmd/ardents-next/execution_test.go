package main

import "testing"

func TestLiveExecutionRouteConsumerCannotSupplyPrivateServiceAuthority(t *testing.T) {
	for _, operation := range []string{"registration-open", "registration-withdraw", "registration-close", "descriptor-publish", "join-prefix-open", "join-open", "join-close", "rendezvous"} {
		if executionRouteOperation(operation) {
			t.Fatalf("live Job expanded to missing Service owner: %s", operation)
		}
	}
	for _, operation := range []string{"prefix-open", "prefix-replenish", "prefix-close", "bootstrap-open", "bootstrap-close", "issuer-issue", "descriptor-lookup", "close"} {
		if !executionRouteOperation(operation) {
			t.Fatalf("bounded genuine Route operation refused: %s", operation)
		}
	}
}

func TestJoinedPreparationCannotAuthorizeLiveWorkerOrPublicationOperations(t *testing.T) {
	for _, operation := range []string{"request", "import", "bootstrap-open", "issuer-issue", "bootstrap-close", "status", "close"} {
		if !preparationOperation(operation) {
			t.Fatalf("permission bootstrap refused %q", operation)
		}
	}
	for _, operation := range []string{"prefix-open", "join-open", "registration-open", "descriptor-publish", "descriptor-lookup", "take", "issue", "complete", "", "unknown"} {
		if preparationOperation(operation) {
			t.Fatalf("dead worker authorized %q", operation)
		}
	}
}
