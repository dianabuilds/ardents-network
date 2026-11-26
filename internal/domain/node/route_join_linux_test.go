//go:build linux

package node_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/domain/hosting"
	"github.com/dianabuilds/ardents-network/internal/endpoint/funding"
	"github.com/dianabuilds/ardents-network/internal/route/prefix"
	"github.com/dianabuilds/ardents-network/internal/route/selection"
	"github.com/dianabuilds/ardents-network/internal/route/transport"
	"github.com/dianabuilds/ardents-network/tests/e2e/node/processfixture"
)

// Physical Source fixtures use the genuine selected leg and reservation. The
// maintained Publisher instead retains its logical SourceScope across gaps.
func openPublicationPrefix(ctx context.Context, plan funding.Plan, selected *selection.Owner, budget *hosting.Budget, authority funding.AdmissionAuthority, holder *stock.Owner) (*prefix.Prefix, selection.Leg, error) {
	config, err := funding.PreparePrefix(ctx, plan, selected, budget, funding.Observe(authority.Current, authority.Witness, authority.Presentation), holder)
	if err != nil {
		return nil, selection.Leg{}, err
	}
	physical, err := prefix.Open(ctx, config)
	if err != nil {
		return nil, selection.Leg{}, errors.Join(err, config.Release())
	}
	return physical, config.Leg, nil
}

// This proves prepared role-3 selection/reservation -> actual Source-bound
// forwarding on both Carriers. It supplies no qualified Publisher, capsule,
// accepted nonce or successful delivery RESULT.
func TestPublicationPreparedResponderOpensWithOriginalSourceTCP(t *testing.T) {
	for _, carrier := range []transport.CarrierProfile{transport.ClosedCarrierTCP} {
		t.Run(string(carrier), func(t *testing.T) {
			f, sockets, certificates := newJoinRouteFixture(t, carrier)
			holder := joinRouteStock(t, f, admission.AllocationPublisher)
			startJoinRouteReceivers(t, f, sockets, certificates)
			root := t.TempDir()
			plan := funding.Plan{Deadline: time.Now().Add(40 * time.Second).UTC().Truncate(time.Second), Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
			installation, err := selection.OpenInstallation(selection.InstallationConfig{EntryRoot: filepath.Join(root, "entry"), Current: f.authority.Current})
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := installation.Close(); err != nil {
					t.Error(err)
				}
			}()
			sourceSelection, err := installation.Borrow(selection.RoleConfig{InteriorRoot: filepath.Join(root, "source"), Domain: 1})
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := sourceSelection.Close(); err != nil {
					t.Error(err)
				}
			}()
			responderSelection, err := installation.Borrow(selection.RoleConfig{InteriorRoot: filepath.Join(root, "responder"), Domain: 3})
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := responderSelection.Close(); err != nil {
					t.Error(err)
				}
			}()
			budget, err := hosting.Open(processfixture.ProcessBudget(t))
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := budget.Close(); err != nil {
					t.Error(err)
				}
			}()
			source, sourceLeg, err := openPublicationPrefix(t.Context(), plan, sourceSelection, budget, f.authority, holder)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := source.Close(); err != nil {
					t.Error(err)
				}
			}()
			if err := source.CheckSourceScope(sourceLeg.Profile.Network, sourceLeg.Profile.Digest, plan.Deadline); err != nil {
				t.Fatal("genuine original Source scope unavailable", err)
			}
			if err := source.CheckSourceCaller(t.Context(), sourceLeg.Profile.Network, sourceLeg.Profile.Digest, plan.Deadline); err != nil {
				t.Fatal("original Source opening caller unavailable", err)
			}
			// Same cancellation channel and bounds do not make an attempt-local
			// wrapper the original physical Source opening caller.
			wrapped := context.WithValue(t.Context(), struct{ originalSourceCaller bool }{true}, true)
			if wrapped.Done() != t.Context().Done() {
				t.Fatal("caller wrapper control changed original cancellation")
			}
			for _, foreign := range []context.Context{nil, context.Background(), wrapped} {
				if err := source.CheckSourceCaller(foreign, sourceLeg.Profile.Network, sourceLeg.Profile.Digest, plan.Deadline); err == nil {
					t.Fatal("foreign Source opening caller accepted")
				}
			}
			changedNetwork, changedProfile := sourceLeg.Profile.Network, sourceLeg.Profile.Digest
			changedNetwork[0] ^= 1
			changedProfile[0] ^= 1
			for _, candidate := range []struct {
				network, profile [32]byte
				end              time.Time
			}{
				{changedNetwork, sourceLeg.Profile.Digest, plan.Deadline},
				{sourceLeg.Profile.Network, changedProfile, plan.Deadline},
				{sourceLeg.Profile.Network, sourceLeg.Profile.Digest, plan.Deadline.Add(time.Second)},
				{sourceLeg.Profile.Network, sourceLeg.Profile.Digest, plan.Deadline.Add(time.Nanosecond)},
				{sourceLeg.Profile.Network, sourceLeg.Profile.Digest, time.Now().Add(-time.Second).UTC().Truncate(time.Second)},
			} {
				if err := source.CheckSourceScope(candidate.network, candidate.profile, candidate.end); err == nil {
					t.Fatal("foreign or extended Source scope accepted")
				}
			}
			prepared, err := funding.PreparePrefix(t.Context(), plan, responderSelection, budget, funding.Observe(f.authority.Current, f.authority.Witness, f.authority.Presentation), holder)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := prepared.Release(); err != nil {
					t.Error(err)
				}
			}()
			responder, err := prefix.OpenResponder(t.Context(), source, prepared)
			if err != nil {
				t.Fatal(err)
			}
			if err := responder.CheckSourceScope(sourceLeg.Profile.Network, sourceLeg.Profile.Digest, plan.Deadline); err == nil {
				t.Fatal("actual Responder accepted as original Source")
			}
			if err := responder.Close(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
