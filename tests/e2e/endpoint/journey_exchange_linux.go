//go:build linux && endpoint_pair_native && document_worker_installed

package endpoint

import (
	"net"
	"testing"
	"time"
)

// ExchangeJourneyCommand sends a command and requires its matching response.
func ExchangeJourneyCommand(t *testing.T, conn net.Conn, index int, command PairMessage) PairMessage {
	t.Helper()
	started := time.Now()
	t.Logf("journey participant %d: %s started", index, command.Operation)
	defer func() {
		t.Logf("journey participant %d: %s returned after %s", index, command.Operation, time.Since(started))
	}()
	SendPairMessage(t, conn, command)
	response := ReceivePairMessage(t, conn)
	if response.Operation != command.Operation {
		t.Fatal("journey handoff lost operation")
	}
	return response
}

// AdmitJourneyParticipant verifies both permission handovers before accepting readiness.
func AdmitJourneyParticipant(t *testing.T, conn net.Conn, approve func(role string, request []byte) []byte) PairMessage {
	t.Helper()
	return receiveJourneyAdmission(t, conn, "ready", approve)
}

// RestartJourneyParticipant requires fresh permission handovers from the restarted invocation.
func RestartJourneyParticipant(t *testing.T, conn net.Conn, approve func(role string, request []byte) []byte) PairMessage {
	t.Helper()
	SendPairMessage(t, conn, PairMessage{Operation: "restart"})
	return receiveJourneyAdmission(t, conn, "restart", approve)
}

func receiveJourneyAdmission(t *testing.T, conn net.Conn, complete string, approve func(role string, request []byte) []byte) PairMessage {
	t.Helper()
	approved := make(map[string]bool, 2)
	for {
		request := ReceivePairMessage(t, conn)
		if request.Operation == complete {
			if !approved["reader"] || !approved["publisher"] || len(request.Original[1]) != 32 || request.Original[2] != "active" {
				t.Fatalf("journey %s omitted independent admission or the active invocation", complete)
			}
			return request
		}
		if request.Operation != "permission" || (request.Role != "reader" && request.Role != "publisher") || approved[request.Role] {
			t.Fatalf("unexpected journey approval request during %s", complete)
		}
		response := approve(request.Role, request.Body)
		approved[request.Role] = true
		SendPairMessage(t, conn, PairMessage{Operation: "permission", Role: request.Role, Body: response})
	}
}
