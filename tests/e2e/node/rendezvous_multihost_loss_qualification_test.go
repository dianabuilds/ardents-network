//go:build native_rendezvous_multihost

package state_test

import (
	"crypto/tls"
	"fmt"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

// TestNativeRendezvousMultiHostAbruptRemoteNodeLoss proves the narrow failure
// outcome of losing the remote product Node while one authenticated pair is
// active. It is not host-loss availability, recovery, retry, or fallback
// evidence: the direct legs must only observe terminal closure.
func TestNativeRendezvousMultiHostAbruptRemoteNodeLoss(t *testing.T) {
	environment := requireH42MultiHostEnvironment(t)
	endpoint := net.JoinHostPort(environment.host, strconv.Itoa(environment.port))
	fixture := newRendezvousStateFixture(t, endpoint)
	stage := stageH42RemoteRendezvous(t, fixture, environment)
	remote := nativeRendezvousMultiHostRemoteRendezvous{environment: environment}
	t.Cleanup(func() { remote.remove(t) })
	remote.start(t, stage)
	remote.waitReady(t)

	attachment := [32]byte{0xb1}
	initiator, err := openRendezvousProcessLeg(t.Context(), endpoint, fixture.initiator.certificate, fixture.rendezvous.public,
		fixture.leg(attachment, route.InitiatorRole))
	if err != nil {
		t.Fatalf("open Initiator leg before remote Node loss: %v\n%s", err, remote.logs(t))
	}
	defer initiator.Close()
	responder, err := openRendezvousProcessLeg(t.Context(), endpoint, fixture.responder.certificate, fixture.rendezvous.public,
		fixture.leg(attachment, route.ResponderRole))
	if err != nil {
		t.Fatalf("open Responder leg before remote Node loss: %v\n%s", err, remote.logs(t))
	}
	defer responder.Close()

	const payload = "native Rendezvous active rendezvous before abrupt remote Node loss"
	if _, err := initiator.Write([]byte(payload)); err != nil {
		t.Fatalf("write active Initiator payload before remote Node loss: %v", err)
	}
	if received := readProcessExact(t, responder, len(payload)); string(received) != payload {
		t.Fatalf("active remote Rendezvous carriage = %q, want %q", received, payload)
	}

	remote.kill(t)
	nativeRendezvousMultiHostRequireTerminalLegClose(t, initiator, "Initiator")
	nativeRendezvousMultiHostRequireTerminalLegClose(t, responder, "Responder")
}

func nativeRendezvousMultiHostRequireTerminalLegClose(t *testing.T, connection *tls.Conn, role string) {
	t.Helper()
	if err := connection.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("set %s loss-read deadline: %v", role, err)
	}
	defer connection.SetReadDeadline(time.Time{})
	buffer := make([]byte, 1)
	if count, err := connection.Read(buffer); err == nil {
		t.Fatalf("%s leg survived abrupt remote Node loss with %d unexpected bytes", role, count)
	} else if networkError, ok := err.(net.Error); ok && networkError.Timeout() {
		t.Fatalf("%s leg did not close within loss-read deadline: %v", role, err)
	}
}

func (remote nativeRendezvousMultiHostRemoteRendezvous) kill(t *testing.T) {
	t.Helper()
	command := fmt.Sprintf("set -eu; docker kill %s >/dev/null; docker container inspect %s >/dev/null 2>&1",
		nativeRendezvousMultiHostShellQuote(remote.environment.container), nativeRendezvousMultiHostShellQuote(remote.environment.container))
	if output, err := remote.run(t, command); err != nil {
		t.Fatalf("abruptly stop remote native Rendezvous product Node container: %v\n%s", err, output)
	}
}
