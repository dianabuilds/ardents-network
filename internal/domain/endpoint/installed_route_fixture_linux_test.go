//go:build linux && document_worker_installed

package endpoint_test

import (
	"context"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/admission"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/domain/endpoint/funding"
	"github.com/dianabuilds/ardents-network/internal/domain/endpoint/session"
	"github.com/dianabuilds/ardents-network/internal/domain/hosting"
	"github.com/dianabuilds/ardents-network/internal/domain/route/introduction"
	"github.com/dianabuilds/ardents-network/tests/e2e/node/processfixture"
)

type holderCommand = session.LocalCommand

func fundingAuthority(authority admissionAuthority) funding.Authority {
	return funding.Observe(authority.Current, authority.Witness, authority.Presentation)
}

type localAdmissionReply = processfixture.AdmissionReply

func networkTestBudget(t *testing.T) *hosting.Budget {
	t.Helper()
	return networkTestOpenBudget(t, processfixture.ProcessBudget(t))
}

func networkTestOpenBudget(t *testing.T, root string) *hosting.Budget {
	t.Helper()
	budget, err := hosting.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := budget.Close(); err != nil {
			t.Error(err)
		}
	})
	return budget
}

func networkTestReservation(t *testing.T, budget *hosting.Budget, deadline time.Time) (func() error, error) {
	t.Helper()
	held, err := networkTestHold(t, budget, deadline)
	if err != nil {
		return nil, err
	}
	return func() error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		return held.Release(ctx)
	}, nil
}

func networkTestHold(t *testing.T, budget *hosting.Budget, deadline time.Time) (*hosting.Reservation, error) {
	t.Helper()
	return budget.Reserve(t.Context(), hosting.ReservationRequest{Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}, WorkUntil: deadline, HoldUntil: deadline.Add(time.Second)})
}

// routePermissionStock provisions real signed permission, without obtaining any
// token. Network issuance scenarios must earn stock through their own exchange.
func routePermissionStock(t *testing.T, f *networkAdmissionFixture, role admission.AllocationRole, maxima [3]uint32) *stock.Owner {
	t.Helper()
	return routePermissionStockAt(t, f, role, maxima, t.TempDir())
}

// The explicit root supports reopening the actual presentation history after
// holder retirement. No permission, private holder key or blinding survives it.
func routePermissionStockAt(t *testing.T, f *networkAdmissionFixture, role admission.AllocationRole, maxima [3]uint32, root string) *stock.Owner {
	t.Helper()
	return processfixture.OpenPermittedStock(t, root, role, maxima, f.authority.ObserveFacts, f.profile, f.spec.Authority, time.Now())
}

func routeConsolePermissionStock(t *testing.T, f *networkAdmissionFixture, send func(any) localAdmissionReply, maxima [3]uint32) {
	t.Helper()
	request := send(holderCommand{Operation: "request", Maxima: maxima})
	if request.Outcome != "completed" {
		t.Fatal("holder request", request.Outcome)
	}
	raw := processfixture.FixturePermission(t, request.Request, f.profile, f.spec.Authority, time.Now())
	if reply := send(holderCommand{Operation: "import", Digest: request.Digest, Payload: raw}); reply.Outcome != "completed" {
		t.Fatal("holder import", reply.Outcome)
	}
}

// prepareIntroductionRegistry uses the genuine receiving birth fact and actual
// independent History; cleanup is registered before the borrowing Receiver.
func prepareIntroductionRegistry(t *testing.T, root string, owner *receiving.Owner, binding receiving.Receiver) *introduction.Registry {
	t.Helper()
	registry, err := introduction.OpenReceivingRegistry(root, introduction.Binding{NetworkID: binding.NetworkID, ProfileDigest: binding.ProfileDigest, ReceiverNodeID: binding.NodeID, ReceiverDutyGeneration: binding.DutyGeneration}, owner)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := registry.Close(); err != nil {
			t.Error(err)
		}
	})
	return registry
}
