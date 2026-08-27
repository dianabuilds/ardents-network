//go:build linux

package state_test

import (
	"syscall"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

func TestRendezvousNodeProcessDrainsActivePairOnSIGTERM(t *testing.T) {
	running := startRendezvousNodeProcess(t)
	attachment := [32]byte{0x93}
	initiator, err := openRendezvousProcessLeg(t.Context(), running.endpoint, running.fixture.initiator.certificate, running.fixture.rendezvous.public,
		running.fixture.leg(attachment, route.InitiatorRole))
	if err != nil {
		t.Fatal(err)
	}
	defer initiator.Close()
	responder, err := openRendezvousProcessLeg(t.Context(), running.endpoint, running.fixture.responder.certificate, running.fixture.rendezvous.public,
		running.fixture.leg(attachment, route.ResponderRole))
	if err != nil {
		t.Fatal(err)
	}
	defer responder.Close()
	if _, err := initiator.Write([]byte("active pair")); err != nil {
		t.Fatal(err)
	}
	if received := readProcessExact(t, responder, len("active pair")); string(received) != "active pair" {
		t.Fatalf("active pair payload = %q", received)
	}
	if err := running.process.command.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("send SIGTERM: %v", err)
	}
	waitNodeState(t, running.process, "DRAINING", 3*time.Second)
	waitNodeState(t, running.process, "WITHDRAWN", 3*time.Second)
	if err, exited := waitProcess(running.process, 3*time.Second); !exited || err != nil {
		t.Fatalf("Rendezvous process did not withdraw cleanly: %v", err)
	}
	if err := responder.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	var one [1]byte
	if _, err := responder.Read(one[:]); err == nil {
		t.Fatal("active Rendezvous leg remained open after graceful withdrawal")
	}
}
