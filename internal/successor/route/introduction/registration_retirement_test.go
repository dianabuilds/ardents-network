package introduction

import (
	"context"
	"errors"
	"net"
	"testing"
)

// This callback oracle isolates original child/caller stop ordering and actual
// pipe Close. It supplies no authenticated Prefix, REGISTER ACK or readiness.
func TestRegistrationPhysicalInterruptionRetainsOriginalCancellation(t *testing.T) {
	for _, mode := range []string{"child-loss", "caller-loss", "owning-stop"} {
		t.Run(mode, func(t *testing.T) {
			caller, cancelCaller := context.WithCancel(t.Context())
			defer cancelCaller()
			child, cancelChild := context.WithCancel(caller)
			defer cancelChild()
			conn, peer := net.Pipe()
			defer peer.Close()
			r := &HolderRegistration{caller: caller, ctx: child, cancel: cancelChild, conn: conn,
				retiring: make(chan struct{}), physicalDone: make(chan struct{})}
			switch mode {
			case "child-loss":
				cancelChild()
				if caller.Err() != nil {
					t.Fatal("child interruption canceled original caller")
				}
			case "caller-loss":
				cancelCaller()
			case "owning-stop":
				r.stop(nil)
			}
			r.interruptPhysical()
			select {
			case <-r.physicalDone:
			default:
				t.Fatal("original physical callback did not finish")
			}
			select {
			case <-r.retiring:
			default:
				t.Fatal("original acquisition remained open after interruption")
			}
			if mode == "owning-stop" {
				if r.failure != nil {
					t.Fatal("private cancellation replaced successful owning stop", r.failure)
				}
			} else if !errors.Is(r.failure, context.Canceled) {
				t.Fatal("unexpected original interruption reported graceful completion", r.failure)
			}
			if r.physicalErr != nil {
				t.Fatal("original pipe close failed", r.physicalErr)
			}
		})
	}
}
