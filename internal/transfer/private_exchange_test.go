package transfer

import "testing"

func TestReplicaHealthResultRoutesToRegisteredResponseWaiter(t *testing.T) {
	if !isReplicaResponse("health_result") {
		t.Fatal("health result was routed as a request")
	}
	if isReplicaResponse("health_query") {
		t.Fatal("health query was routed as a response")
	}
}
