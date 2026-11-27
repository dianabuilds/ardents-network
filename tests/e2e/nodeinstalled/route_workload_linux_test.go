//go:build linux && (forwarding_platform_native || descriptor_platform_native || join_platform_native)

package nodeinstalled_test

import (
	"bytes"
	"crypto/tls"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/admission"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/domain/endpoint/funding"
	"github.com/dianabuilds/ardents-network/internal/domain/endpoint/session"
	"github.com/dianabuilds/ardents-network/internal/domain/hosting"
	"github.com/dianabuilds/ardents-network/internal/domain/network/state"
	"github.com/dianabuilds/ardents-network/internal/domain/reachability"
	"github.com/dianabuilds/ardents-network/internal/domain/route/selection"
	"github.com/dianabuilds/ardents-network/internal/platform/processguard"
	"github.com/dianabuilds/ardents-network/tests/e2e/node/processfixture"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
	networkinputs "github.com/dianabuilds/ardents-network/tests/networkfixture"
)

type installedRouteFixture struct {
	spec            networkfixture.ClosedSpec
	bundle          networkfixture.Closed
	issuer          issuer.Plan
	clock           string
	ports           map[[32]byte]func()
	certificates    map[[32]byte]tls.Certificate
	holder          *state.State
	forwarding      funding.Plan
	descriptorProof byte
}

func prepareInstalledRouteFixture(t *testing.T, spec networkfixture.ClosedSpec, bundle networkfixture.Closed, issuerPlan issuer.Plan, clock string,
	ports map[[32]byte]func(), certificates map[[32]byte]tls.Certificate) installedRouteFixture {
	t.Helper()
	plan := networkinputs.SeedNetworkState(t, spec.NetworkID, [32]byte(spec.Authority[32:]), bundle, clock)
	config, err := plan.Config()
	if err != nil {
		t.Fatal(err)
	}
	holder, err := state.OpenRetained(config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := holder.Close(); err != nil {
			t.Error("original fixture holder State did not close", err)
		}
	})
	return installedRouteFixture{spec: spec, bundle: bundle, issuer: issuerPlan, clock: clock, ports: ports, certificates: certificates, holder: holder}
}

func installedHolderPlan(t *testing.T, domain uint8) funding.Plan {
	t.Helper()
	root := processfixture.TempDir(t)
	return funding.Plan{
		EntryRoot: filepath.Join(root, "entry"), InteriorRoot: filepath.Join(root, "interior"),
		SourceInteriorRoot: filepath.Join(root, "source-interior"), HostingRoot: processfixture.ProcessBudget(t),
		Domain: domain, Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10},
	}
}

// Provision the actual selected Entry as the installed Node. Retain the two
// selection roots across launches so no trial or redraw can avoid that Node.
func (f *installedRouteFixture) selectForwarding(t *testing.T) [32]byte {
	t.Helper()
	f.forwarding = installedHolderPlan(t, 3)
	chosen, err := selection.Open(selection.Config{EntryRoot: f.forwarding.EntryRoot, InteriorRoot: f.forwarding.InteriorRoot, Domain: f.forwarding.Domain, Current: f.holder.CurrentRuntime})
	if err != nil {
		t.Fatal(err)
	}
	leg, err := chosen.Select()
	if err := errors.Join(err, chosen.Close()); err != nil {
		t.Fatal(err)
	}
	return leg.Entry.NodeID
}

func (f installedRouteFixture) authority() funding.Authority {
	return funding.Observe(f.holder.CurrentRuntime, f.holder.CurrentRuntimeWitness, funding.PresentationGuard(f.holder.CurrentRuntime))
}

func (f installedRouteFixture) startForwardingPeers(t *testing.T, binary string, installed [32]byte) []*processfixture.NativeProcess {
	t.Helper()
	var peers []*processfixture.NativeProcess
	for _, member := range f.bundle.Nodes {
		id := member.NodeID
		if id == installed || member.Subrole != 1 && member.Subrole != 2 {
			continue
		}
		network := networkinputs.SeedNetworkState(t, f.spec.NetworkID, [32]byte(f.spec.Authority[32:]), f.bundle, f.clock)
		cert, key := processfixture.WriteNativeCredential(t, f.certificates[id])
		plan := map[string]any{
			"network": network, "node_id": id, "process_profile": processguard.ForwardingProfile,
			"participation_root": filepath.Join(t.TempDir(), "participation"), "spend_root": t.TempDir(),
			"hosting_root": processfixture.ProcessBudgetWithCapacity(t, 256), "certificate": cert, "private_key": key,
			"work": hosting.Traffic{Tx: 32 << 20, Rx: 32 << 20}, "termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}, "retirement_wait_ms": 5000,
		}
		f.ports[id]()
		peers = append(peers, processfixture.StartFreshRole(t, "forwarding", binary, processfixture.WriteConfig(t, plan), id[0]))
	}
	return peers
}

func (f installedRouteFixture) verifyForwarding(t *testing.T, binary, spendRoot string, installed [32]byte, phase string) {
	t.Helper()
	f.verifyPrefix(t, binary, spendRoot, installed, 0, phase)
}

func (f *installedRouteFixture) verifyDescriptor(t *testing.T, binary, spendRoot string, installed [32]byte, phase string) {
	t.Helper()
	f.descriptorProof++
	f.verifyPrefix(t, binary, spendRoot, installed, f.descriptorProof, phase)
}

func (f installedRouteFixture) verifyPrefix(t *testing.T, binary, spendRoot string, installed [32]byte, descriptorProof byte, phase string) {
	t.Helper()
	if !t.Run(phase+" authenticated Route exchange", func(t *testing.T) {
		peers := f.startForwardingPeers(t, binary, installed)
		var descriptor [32]byte
		if descriptorProof != 0 {
			descriptor = installed
		}
		holder := processfixture.FundRouteStock(t, f.spec, f.bundle, f.issuer, f.holder.CurrentRuntime, admission.AllocationUser, descriptor)
		plan := f.forwarding
		if descriptor != [32]byte{} {
			plan = installedHolderPlan(t, 1)
		}
		plan.Deadline = time.Now().Add(30 * time.Second).UTC().Truncate(time.Second)
		before := installedSpends(t, spendRoot)
		route, err := session.OpenPrefix(t.Context(), plan, f.authority(), holder)
		if err != nil {
			t.Fatal("actual Forwarding prefix failed", err)
		}
		defer route.Close()
		if descriptor != [32]byte{} {
			view, err := f.holder.CurrentRuntime()
			if err != nil {
				t.Fatal(err)
			}
			facts, _, err := networkinputs.AdmissionFacts(view)
			if err != nil {
				t.Fatal(err)
			}
			raw, target := processfixture.SignedDescriptorForAuthority(t, facts, 3, [32]byte{18}, facts.Digest, descriptorProof)
			if err := route.PublishDescriptor(t.Context(), raw); err != nil {
				t.Fatal("installed Descriptor publish/ACK failed", err)
			}
			var history reachability.History
			defer history.Close()
			got, err := route.LookupDescriptor(t.Context(), target, &history)
			if err != nil || !bytes.Equal(got, raw) {
				t.Fatal("installed Descriptor exact lookup failed", err)
			}
		}
		if err := route.Close(); err != nil {
			t.Fatal("authenticated prefix did not join", err)
		}
		requireInstalledSpend(t, spendRoot, before)
		stopInstalledPeers(t, peers)
	}) {
		t.FailNow()
	}
}

func installedSpends(t *testing.T, root string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, "closed-token-spends"))
	if err != nil || len(raw) < 8+32+32+32+8 {
		t.Fatal("installed receiver durable spend unavailable", err)
	}
	return raw
}

func requireInstalledSpend(t *testing.T, root string, before []byte) {
	t.Helper()
	after := installedSpends(t, root)
	if len(after) <= len(before) || !bytes.HasPrefix(after, before) {
		t.Fatal("authenticated operation did not append to the installed receiver's original spend")
	}
}

func stopInstalledPeers(t *testing.T, peers []*processfixture.NativeProcess) {
	t.Helper()
	for _, p := range peers {
		p.Stop(t)
	}
}
