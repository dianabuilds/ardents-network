//go:build linux && document_worker_installed

package endpoint_test

import (
	"os"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/route/transport"
)

func TestInstalledConnectionAuthorityTCP(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Fatal("invalid environment: original non-root installed Endpoint required")
	}
	for _, carrier := range []transport.CarrierProfile{transport.ClosedCarrierTCP} {
		t.Run(string(carrier), func(t *testing.T) {
			for _, role := range []string{"initiator", "publisher"} {
				t.Run(role, func(t *testing.T) {
					for _, fault := range []string{"worker", "network", "deadline"} {
						t.Run(fault, func(t *testing.T) { runInstalledConnectionTextLoss(t, carrier, "authority-"+role+"-"+fault) })
					}
				})
			}
		})
	}
}

// Separate actual State roots; only signed public inputs and fixture issuer
// preparation are shared. Closing this local observer cannot revoke receivers.
func installedConnectionLocalNetwork(t *testing.T, original *networkAdmissionFixture) *networkAdmissionFixture {
	local := publicationReceivingNetwork(t, original)
	local.plan, local.spec, local.digest, local.receiver = original.plan, original.spec, original.digest, original.receiver
	return local
}

// Only the actual StopUnit scenarios may retain these role-specific protocol
// failures. Every leaf must be expected; physical cleanup is checked separately.
func onlyInstalledConnectionStoppedWorker(err error, role string) bool {
	if err == nil {
		return false
	}
	if onlyInstalledConnectionCancellation(err) {
		return true
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		children := joined.Unwrap()
		if len(children) == 0 {
			return false
		}
		for _, child := range children {
			if !onlyInstalledConnectionStoppedWorker(child, role) {
				return false
			}
		}
		return true
	}
	if wrapped, ok := err.(interface{ Unwrap() error }); ok {
		return onlyInstalledConnectionStoppedWorker(wrapped.Unwrap(), role)
	}
	if role == "initiator" {
		return err.Error() == "document reader worker ended or emitted output before Connection transfer"
	}
	return role == "publisher" && (err.Error() == "document Publisher worker exchange was interrupted" ||
		err.Error() == "qualified worker attachment ended unexpectedly")
}
