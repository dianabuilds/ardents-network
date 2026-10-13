//go:build linux && text_worker_installed

package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
	"github.com/dianabuilds/ardents-network/internal/successor/publication/durable"
	"github.com/dianabuilds/ardents-network/internal/successor/publication/instance"
	"github.com/dianabuilds/ardents-network/internal/successor/reachability"
	"github.com/dianabuilds/ardents-network/internal/successor/route/receiver"
	"github.com/dianabuilds/ardents-network/internal/successor/route/role"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
)

func TestInstalledConnectionDurableReopenBothCarriers(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Fatal("invalid environment: original non-root installed Endpoint required")
	}
	for _, carrier := range []transport.CarrierProfile{transport.ClosedCarrierTCP, transport.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) { runInstalledConnectionTextLoss(t, carrier, "durable-reopen") })
	}
}

// Capture only an actually accepted token, without substituting admission or
// reservation. Verification runs after every original receiver has joined.
func installedReceiverReopen(t *testing.T, root string, binding receiving.Receiver, observe func() (receiving.Observation, error), original *receiving.Owner, budgetRoot string, budget *hosting.Budget, admit func(context.Context, receiver.Channel, []byte) (receiving.Grant, error)) (func(context.Context, receiver.Channel, []byte) (receiving.Grant, error), func() int) {
	var mu sync.Mutex
	var token []byte
	var class admission.Class
	var deadline time.Time
	wrapped := func(ctx context.Context, channel receiver.Channel, raw []byte) (receiving.Grant, error) {
		grant, err := admit(ctx, channel, raw)
		if err == nil {
			mu.Lock()
			if token == nil {
				token = bytes.Clone(raw)
				class, _ = role.AdmissionClass(channel.Hello.Purpose)
				deadline = grant.Allowance().Deadline()
			}
			mu.Unlock()
		}
		return grant, err
	}
	t.Cleanup(func() { clear(token) })
	verify := func() int {
		before, err := budget.Observe(t.Context())
		if err != nil || before.ReservedBytes != 0 {
			t.Fatal("original physical reservations not joined", before, err)
		}
		if err := errors.Join(original.Close(), budget.Close()); err != nil {
			t.Fatal(err)
		}
		reopenedBudget, err := hosting.Open(budgetRoot)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := reopenedBudget.Close(); err != nil {
				t.Error(err)
			}
		}()
		after, err := reopenedBudget.Observe(t.Context())
		if err != nil || after.ReservedBytes != 0 || after.UsedBytes < before.UsedBytes {
			t.Fatal("Hosting reopen reset original accounting", before, after, err)
		}
		reopened, err := receiving.Open(root, binding, observe)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := reopened.Close(); err != nil {
				t.Error(err)
			}
		}()
		if token == nil {
			return 0
		}
		observation, err := observe()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := receiving.VerifyToken(observation.Profile, binding, class, token, observation.Now); err != nil || !observation.Now.Before(deadline) {
			t.Fatal("replay oracle lost original valid token or deadline", err)
		}
		reserved, returned := 0, 0
		grant, err := reopened.Accept(t.Context(), class, token, deadline, func() (func() error, error) {
			release, err := networkTestReservation(t, reopenedBudget, deadline)
			if err != nil {
				return nil, err
			}
			reserved++
			return func() error { returned++; return release() }, nil
		})
		if err == nil {
			_ = grant.Release()
			t.Fatal("reopened receiver refunded an actually spent token")
		}
		if !strings.Contains(err.Error(), "closed token is already spent") || reserved != 1 || returned != 1 {
			t.Fatal("replay failed outside durable spend or leaked reservation", reserved, returned, err)
		}
		rolledBack, err := reopenedBudget.Observe(t.Context())
		if err != nil || rolledBack.ReservedBytes != 0 || rolledBack.UsedBytes < after.UsedBytes {
			t.Fatal("durable spend refusal did not return original physical capacity", rolledBack, err)
		}
		return 1
	}
	return wrapped, verify
}

func verifyInstalledConnectionReopen(t *testing.T, f *networkAdmissionFixture, store *reachability.Store, storeRoot string, plan publicationPlan, target [32]byte) {
	t.Helper()
	before, err := f.current()
	if err != nil {
		t.Fatal(err)
	}
	f.reopen()
	after, err := f.current()
	if err != nil || before.Profile().ProfileBinding != after.Profile().ProfileBinding {
		t.Fatal("Network reopen changed accepted binding", err)
	}
	checked := 0
	for _, verify := range f.afterRouteJoin {
		checked += verify()
	}
	if checked < 4 {
		t.Fatal("insufficient actual receiving spends to check", checked)
	}
	raw, status, err := store.Lookup(target, f.profile.Digest, time.Now())
	if err != nil || status != reachability.Accepted {
		t.Fatal("original stored Descriptor unavailable", status, err)
	}
	proof, err := reachability.Verify(raw, target, f.profile.NetworkID, f.profile.Digest, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := reachability.OpenStore(reachability.StoreConfig{Root: storeRoot, Network: f.profile.NetworkID})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := reopened.Close(); err != nil {
			t.Error(err)
		}
	}()
	retained, status, err := reopened.Lookup(target, f.profile.Digest, time.Now())
	if err != nil || status != reachability.Accepted || !bytes.Equal(raw, retained) {
		t.Fatal("Store reopen lost exact accepted Descriptor", status, err)
	}
	history, err := durable.Open(t.Context(), durable.Config{Root: plan.Root, Target: target, Network: f.profile.NetworkID})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := history.Close(); err != nil {
			t.Error(err)
		}
	}()
	floor, err := history.Floor(t.Context())
	if err != nil || floor != proof.Publication.Delegation().Generation {
		t.Fatal("Publication reopen reset consumed generation", floor, err)
	}
	consumed, err := instance.Open(t.Context(), plan.InstanceRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := consumed.Close(); err != nil {
			t.Error(err)
		}
	}()
	if _, err := consumed.Credential(t.Context()); !errors.Is(err, instance.ErrSuccessorRequired) {
		t.Fatal("reopen revived consumed Instance", err)
	}
	t.Log("original Network, receiving spends, Hosting, Descriptor and Publication retained across closed-owner reopen; consumed Instance unavailable", checked)
}
