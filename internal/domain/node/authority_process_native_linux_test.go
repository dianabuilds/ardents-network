//go:build linux && node_process_native

package node_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission/issuance"
	"github.com/dianabuilds/ardents-network/internal/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/admission/spending"
	"github.com/dianabuilds/ardents-network/internal/domain/node"
	"github.com/dianabuilds/ardents-network/internal/hosting"
	"github.com/dianabuilds/ardents-network/internal/network"
	"github.com/dianabuilds/ardents-network/internal/network/duty"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/reachability"
	"github.com/dianabuilds/ardents-network/internal/route/introduction"
	"github.com/dianabuilds/ardents-network/internal/route/role"
	"github.com/dianabuilds/ardents-network/internal/route/transport"
	"github.com/dianabuilds/ardents-network/tests/e2e/node/processfixture"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
	networkinputs "github.com/dianabuilds/ardents-network/tests/networkfixture"
)

// This is a native composition test, not a replacement command or installed
// qualification. The ordinary command scenarios separately prove that consumer.
// The root driver prepares signed inputs outside the measured cgroup; the child
// opens genuine original State/Node/participation/Admission/Hosting owners.
// The only injected mechanism is scheduling the return of an actual completed
// State observation. No successful authority, role or native reading is supplied.
type nativeOriginalAuthorityPlan struct {
	Network                        state.AuthorityDeclaration
	Participation, Spend, Hosting  string
	Certificate, PrivateKey, Fault string
	Successor                      networkfixture.Closed
	Conflict                       []byte
}

func TestNativeNodeOriginalAuthorityAndJoinTCP(t *testing.T) {
	if path := os.Getenv("ARDENTS_NODE_ORIGINAL_FIXTURE"); path != "" {
		testNativeOriginalAuthorityChild(t, path)
		return
	}
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, carrier := range []transport.CarrierProfile{transport.ClosedCarrierTCP} {
		t.Run(string(carrier), func(t *testing.T) {
			for index, fault := range []string{"stop", "caller", "stale-measurement", "successor", "profile-conflict", "member-expiry", "startup-cancel", "deferred-quic"} {
				t.Run(fault, func(t *testing.T) {
					selectedCarrier := carrier
					if fault == "deferred-quic" {
						selectedCarrier = transport.ClosedCarrierQUIC
					}
					spec, _, _, member, releaseSocket := authorityProcessInput(t, selectedCarrier, 12, 1, 1, fault)
					bundle, err := networkfixture.BuildClosed(spec)
					if err != nil {
						t.Fatal(err)
					}
					clock, closeClock := networkinputs.StartObservedClock(t)
					defer closeClock()
					plan := nativeOriginalAuthorityPlan{Network: networkinputs.SeedNetworkState(t, spec.NetworkID, [32]byte(spec.Authority[32:]), bundle, clock), Participation: filepath.Join(t.TempDir(), "participation"), Spend: t.TempDir(), Hosting: processfixture.ProcessBudget(t), Fault: fault}
					plan.Certificate, plan.PrivateKey = processfixture.WriteNativeCredential(t, processfixture.RoleCertificate(t, member.PrivateKey))
					next := spec
					next.Number++
					next.Previous = bundle.Epoch.Digest
					next.Seed[0]++
					plan.Successor, err = networkfixture.BuildClosed(next)
					if err != nil {
						t.Fatal(err)
					}
					conflict := spec
					conflict.IssuanceAuthority[0] ^= 1
					changed, err := networkfixture.BuildClosed(conflict)
					if err != nil || changed.Epoch.Digest != bundle.Epoch.Digest {
						t.Fatal("conflict changed the original Epoch", err)
					}
					plan.Conflict = changed.Profile
					path := writeAuthorityProcessPlan(t, plan)
					releaseSocket()
					if fault == "deferred-quic" {
						// A tripwire only: never accept or supply a successful peer.
						// Historical signed QUIC may not be reinterpreted as TCP.
						tcp, err := net.Listen("tcp", member.Endpoint)
						if err != nil {
							t.Fatal("TCP fallback tripwire unavailable", err)
						}
						defer func() {
							if err := tcp.Close(); err != nil {
								t.Error(err)
							}
						}()
					}
					unit := fmt.Sprintf("ardents-forwarding-owner-%d-%d.service", os.Getpid(), index)
					t.Cleanup(func() { _ = exec.Command("systemctl", "stop", unit).Run() })
					command := processfixture.OwnerTestCommand(t, unit, binary, "TestNativeNodeOriginalAuthorityAndJoinTCP", "ARDENTS_NODE_ORIGINAL_FIXTURE", path, 120)
					output, err := command.CombinedOutput()
					if err != nil {
						t.Fatalf("genuine original native composition failed: %v\n%s", err, output)
					}
				})
			}
		})
	}
}

func testNativeOriginalAuthorityChild(t *testing.T, path string) {
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var plan nativeOriginalAuthorityPlan
	if err := json.Unmarshal(raw, &plan); err != nil {
		t.Fatal(err)
	}
	config, err := plan.Network.Config()
	if err != nil {
		t.Fatal(err)
	}
	config.LocalRoleStateRoot = plan.Participation
	owner, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := owner.Close(); err != nil {
			t.Error("original State close", err)
		}
	})
	view, err := owner.CurrentRuntime()
	if err != nil {
		t.Fatal(err)
	}
	member, err := view.Member([32]byte{12}, view.ObservedAt())
	if err != nil {
		t.Fatal(err)
	}
	retained, err := view.RetainDuty(member.NodeID, view.ObservedAt())
	if err != nil {
		t.Fatal(err)
	}
	var armed, held atomic.Bool
	var canceledAtListener atomic.Bool
	entered, resume := make(chan struct{}), make(chan struct{})
	var resumeOnce sync.Once
	release := func() { resumeOnce.Do(func() { close(resume) }) }
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	current := func() (network.RuntimeView, error) {
		actual, err := owner.CurrentRuntime()
		if plan.Fault == "startup-cancel" && !canceledAtListener.Load() && processfixture.OriginalListenerOccupied(t, member) && canceledAtListener.CompareAndSwap(false, true) {
			// Cancel after an actual listener effect and a completed real State
			// read, before the original consumer's successful final handoff.
			cancel()
		}
		if armed.Load() && held.CompareAndSwap(false, true) {
			close(entered)
			<-resume
		}
		return actual, err
	}
	var budgetBefore []byte
	if plan.Fault == "deferred-quic" {
		budgetBefore, err = os.ReadFile(filepath.Join(plan.Hosting, "budget.json"))
		if err != nil {
			t.Fatal(err)
		}
	}
	original, err := node.ListenForwarding(ctx, node.ForwardingConfig{Authority: role.Authority{Current: current, Duty: retained, Profile: view.Profile().ProfileBinding},
		ParticipationRoot: plan.Participation, SpendRoot: plan.Spend, HostingRoot: plan.Hosting, Certificate: plan.Certificate, PrivateKey: plan.PrivateKey,
		Work: hosting.Traffic{Tx: 32 << 20, Rx: 32 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}})
	binding := spending.Binding{NetworkID: view.Profile().Network, ProfileDigest: view.Profile().Digest, ReceiverNodeID: member.NodeID, ReceiverDutyGeneration: member.DutyGeneration}
	if plan.Fault == "deferred-quic" {
		if original != nil {
			_ = original.Close()
			t.Fatal("deferred Carrier returned an original invocation")
		}
		var onlyDeferred func(error) bool
		seenDeferred := false
		onlyDeferred = func(cause error) bool {
			if cause == nil {
				return false
			}
			if joined, ok := cause.(interface{ Unwrap() []error }); ok {
				children := joined.Unwrap()
				if len(children) == 0 {
					return false
				}
				for _, child := range children {
					if !onlyDeferred(child) {
						return false
					}
				}
				return true
			}
			if _, wrapped := cause.(interface{ Unwrap() error }); wrapped {
				return false
			}
			switch cause.Error() {
			case "route Carrier profile is unsupported":
				seenDeferred = true
				return true
			case "original Forwarding assignment unavailable":
				// The Node's fixed startup context accompanies Authority.Member's
				// refusal; it is not a physical cleanup wrapper or another cause.
				return true
			default:
				return false
			}
		}
		if member.CarrierProfile != string(transport.ClosedCarrierQUIC) || node.StartupFailure(err) == nil || !onlyDeferred(errors.Unwrap(err)) || !seenDeferred {
			t.Fatal("actual process Guard/Node did not produce the exact deferred-Carrier refusal", err, errors.Unwrap(err))
		}
		if _, err := os.Lstat(plan.Participation); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("deferred Carrier created a producer root", err)
		}
		entries, err := os.ReadDir(plan.Spend)
		if err != nil || len(entries) != 0 {
			t.Fatal("deferred Carrier opened receiving history", err)
		}
		budgetAfter, err := os.ReadFile(filepath.Join(plan.Hosting, "budget.json"))
		if err != nil || !bytes.Equal(budgetBefore, budgetAfter) {
			t.Fatal("deferred Carrier opened or changed provider accounting", err)
		}
		if processfixture.OriginalListenerOccupied(t, member) {
			t.Fatal("deferred Carrier retained a UDP listener")
		}
		return
	}
	if plan.Fault == "startup-cancel" {
		if original != nil {
			_ = original.Close()
			t.Fatal("late canceled acquisition returned an accepting original")
		}
		if !canceledAtListener.Load() || !errors.Is(err, context.Canceled) {
			t.Fatal("late startup cancellation lost its original cause", err)
		}
		processfixture.RequireFailedStartupCleanup(t, plan.Participation, plan.Spend, plan.Hosting, member, binding)
		return
	}
	if err != nil {
		t.Fatal("original native Node refused real startup", err)
	}
	t.Cleanup(func() { release(); original.Stop(); _ = original.Close() })
	armed.Store(true)
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("real original State observation never entered the held return")
	}
	var networkExpiry error
	switch plan.Fault {
	case "stop":
		stopped := make(chan struct{})
		go func() { original.Stop(); close(stopped) }()
		select {
		case <-stopped:
		case <-time.After(200 * time.Millisecond):
			t.Fatal("synchronous Node denial waited for foreign authority I/O")
		}
	case "caller":
		cancel()
	case "stale-measurement":
		timer := time.NewTimer(1200 * time.Millisecond)
		select {
		case <-timer.C:
		case <-t.Context().Done():
			t.Fatal(t.Context().Err())
		}
		timer.Stop()
	case "successor":
		if _, err := owner.AcceptEpoch(t.Context(), plan.Successor.Epoch.Raw, plan.Successor.Epoch.Inputs, plan.Successor.Epoch.Materials); err != nil {
			t.Fatal(err)
		}
		if _, err := owner.AcceptClosedProfile(plan.Successor.Profile); err != nil {
			t.Fatal(err)
		}
		changed, err := owner.CurrentRuntime()
		if err != nil || changed.MatchDuty(retained, changed.ObservedAt()) == nil {
			t.Fatal("genuine successor preserved original duty", err)
		}
	case "profile-conflict":
		if _, err := owner.AcceptClosedProfile(plan.Conflict); err == nil {
			t.Fatal("signed second profile accepted as original")
		}
		if _, err := owner.CurrentRuntime(); err == nil {
			t.Fatal("genuine State conflict supplied authority")
		}
	case "member-expiry":
		timer := time.NewTimer(time.Until(member.NotAfter()) + 150*time.Millisecond)
		select {
		case <-timer.C:
		case <-t.Context().Done():
			t.Fatal(t.Context().Err())
		}
		timer.Stop()
		actual, err := owner.CurrentRuntime()
		if err != nil {
			t.Fatal(err)
		}
		networkExpiry = processfixture.AwaitNativeExpiryDenial(t, actual, retained, func() bool { return original.FirstFailure() != nil })
	default:
		t.Fatal("unselected original authority fault", plan.Fault)
	}
	// A delayed original watcher is still a physical borrower. A bounded wait
	// cannot finish the Node, remove its producer or release its spend root.
	select {
	case <-original.Done():
		t.Fatal("original completion was published before held work joined")
	case <-time.After(30 * time.Millisecond):
	}
	journal, err := duty.OpenOperation(t.Context(), duty.Config{Root: plan.Participation, Clock: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	collision, err := journal.Conflict(member.NodeID, member.FamilyID)
	if closeErr := journal.Close(); err != nil || closeErr != nil || !collision {
		t.Fatal("unjoined original producer disappeared", collision, err, closeErr)
	}
	if ledger, err := spending.Open(plan.Spend, binding); err == nil {
		_ = ledger.Close()
		t.Fatal("unjoined Node returned the original spend root")
	}
	release()
	select {
	case <-original.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("original role did not join after releasing actual observation return")
	}
	result := original.Close()
	first, _ := original.JoinedFailureOwners()
	expected := "network"
	if plan.Fault == "stop" || plan.Fault == "caller" {
		expected = "caller"
		if !errors.Is(result, context.Canceled) || original.JoinedOutcome() != "canceled" {
			t.Fatal("original cancellation lost its cause or retained a counterfeit cleanup failure", result, original.JoinedOutcome())
		}
	} else {
		if plan.Fault == "stale-measurement" {
			expected = "process"
		}
		if result == nil || original.JoinedOutcome() != "failed" {
			t.Fatal("original authority/resource retirement reported success", result)
		}
		if plan.Fault == "member-expiry" {
			expected = nativeExpiryFirstOwner(t, result, networkExpiry, original.FirstFailure())
		}
	}
	if first != expected {
		t.Fatal("original first-failure owner changed", first, expected, result)
	}
	copy := *original
	if copy.Done() != original.Done() || copy.Close() != result || copy.JoinedOutcome() != original.JoinedOutcome() {
		t.Fatal("late copied handle changed the original result")
	}
	journal, err = duty.OpenOperation(t.Context(), duty.Config{Root: plan.Participation, Clock: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	collision, err = journal.Conflict(member.NodeID, member.FamilyID)
	if closeErr := journal.Close(); err != nil || closeErr != nil || collision {
		t.Fatal("joined producer remains live", collision, err, closeErr)
	}
	ledger, err := spending.Open(plan.Spend, binding)
	if err != nil {
		t.Fatal("original physical join did not release spend root", err)
	}
	if err := ledger.Close(); err != nil {
		t.Fatal(err)
	}
	budget, err := hosting.Open(plan.Hosting)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := budget.Observe(t.Context())
	if closeErr := budget.Close(); err != nil || closeErr != nil || snapshot.ReservedBytes != 0 {
		t.Fatal("original join changed capacity", snapshot, err, closeErr)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := state.Open(config)
	if err != nil {
		t.Fatal("original State root failed genuine reopen", err)
	}
	defer reopened.Close()
	recovered, err := reopened.CurrentRuntime()
	if plan.Fault == "profile-conflict" {
		if err == nil {
			t.Fatal("profile conflict vanished on reopen")
		}
	} else if err != nil {
		t.Fatal(err)
	} else if plan.Fault == "successor" && recovered.MatchDuty(retained, recovered.ObservedAt()) == nil {
		t.Fatal("original duty rebound to recovered successor")
	}
}

func nativeExpiryFirstOwner(t *testing.T, result, networkExpiry error, fact *node.FailureDiagnostic) string {
	t.Helper()
	joined, ok := result.(interface{ Unwrap() []error })
	if !ok || len(joined.Unwrap()) == 0 || networkExpiry == nil {
		t.Fatal("original first expiry cause unavailable", result)
	}
	// Producer.Check precedes foreign Network I/O; MatchDuty after it can cross
	// the same signed expiry first. Require the exact owner of the retained cause,
	// never relabel a failure to impose scheduler order or accept another error.
	cause := joined.Unwrap()[0]
	expected, code := "participation", "owner-refusal"
	switch cause.Error() {
	case "original Node participation expired":
	case networkExpiry.Error():
		expected, code = "network", "authority-refusal"
	default:
		t.Fatal("expiry replaced by unrelated first cause", cause)
	}
	if fact == nil || fact.Kind != "logical" || fact.Phase != "serving" || fact.Cause != code || fact.Resource != nil {
		t.Fatal("original expiry diagnostic differs from its retained cause", fact)
	}
	return expected
}

func authorityProcessInput(t *testing.T, carrier transport.CarrierProfile, id, domain, subrole byte, fault string) (networkfixture.ClosedSpec, issuer.Plan, issuance.Inventory, networkfixture.ClosedNode, func()) {
	t.Helper()
	spec, issuerPlan, inventory := networkinputs.PrepareNetworkGenesis(t, t.TempDir())
	member := &spec.Nodes[1]
	if id == 11 {
		member = &spec.Nodes[0]
	}
	member.NodeID, member.RoleDomain, member.Subrole = [32]byte{id}, domain, subrole
	member.Carrier = string(carrier)
	if fault == "member-expiry" {
		member.ValidUntil = time.Now().Add(5 * time.Second).UTC().Truncate(time.Second)
	}
	var socket net.PacketConn
	var listener net.Listener
	var err error
	if carrier == transport.ClosedCarrierQUIC {
		socket, err = net.ListenPacket("udp", "127.0.0.1:0")
	} else {
		listener, err = net.Listen("tcp", "127.0.0.1:0")
	}
	if err != nil {
		t.Fatal(err)
	}
	var closeSocket func() error
	if socket != nil {
		member.Endpoint, closeSocket = socket.LocalAddr().String(), socket.Close
	} else {
		member.Endpoint, closeSocket = listener.Addr().String(), listener.Close
	}
	release := sync.OnceFunc(func() {
		if err := closeSocket(); err != nil {
			t.Error(err)
		}
	})
	t.Cleanup(release)
	return spec, issuerPlan, inventory, *member, release
}

func writeAuthorityProcessPlan(t *testing.T, plan any) string {
	t.Helper()
	raw, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "authority.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

type nativeDescriptorCaller struct {
	context.Context
	denied atomic.Bool
}

func (caller *nativeDescriptorCaller) Err() error {
	if caller.denied.Load() {
		return context.Canceled
	}
	return caller.Context.Err()
}

// Native composition holds only a completed genuine State return, never a fake
// accepting observation. The actual command has a separate positive workload.
func TestNativeDescriptorOriginalAuthorityAndJoinTCP(t *testing.T) {
	if path := os.Getenv("ARDENTS_DESCRIPTOR_ORIGINAL_FIXTURE"); path != "" {
		nativeDescriptorAuthorityChild(t, path)
		return
	}
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, carrier := range []transport.CarrierProfile{transport.ClosedCarrierTCP} {
		t.Run(string(carrier), func(t *testing.T) {
			for index, fault := range []string{"stop", "caller", "direct-caller", "stale-measurement", "successor", "profile-conflict", "member-expiry", "startup-cancel", "store-root-loss", "store-records-loss", "store-marker-loss", "store-lock-loss", "store-lease-fd-loss"} {
				t.Run(fault, func(t *testing.T) {
					spec, _, _, member, releaseSocket := authorityProcessInput(t, carrier, 17, 2, 5, fault)
					bundle, err := networkfixture.BuildClosed(spec)
					if err != nil {
						t.Fatal(err)
					}
					clock, closeClock := networkinputs.StartObservedClock(t)
					defer closeClock()
					cert, key := processfixture.WriteNativeCredential(t, processfixture.RoleCertificate(t, member.PrivateKey))
					plan := processfixture.DescriptorAuthorityPlan{Network: networkinputs.SeedNetworkState(t, spec.NetworkID, [32]byte(spec.Authority[32:]), bundle, clock), Participation: filepath.Join(t.TempDir(), "participation"), Spend: t.TempDir(), Hosting: processfixture.ProcessBudget(t), Descriptor: t.TempDir(), Certificate: cert, PrivateKey: key, Fault: fault}
					next := spec
					next.Number++
					next.Previous = bundle.Epoch.Digest
					next.Seed[0]++
					plan.Successor, err = networkfixture.BuildClosed(next)
					if err != nil {
						t.Fatal(err)
					}
					conflict := spec
					conflict.IssuanceAuthority[0] ^= 1
					changed, err := networkfixture.BuildClosed(conflict)
					if err != nil || changed.Epoch.Digest != bundle.Epoch.Digest {
						t.Fatal("signed conflict changed original Epoch", err)
					}
					plan.Conflict = changed.Profile
					path := writeAuthorityProcessPlan(t, plan)
					releaseSocket()
					unit := fmt.Sprintf("ardents-descriptor-owner-%d-%d.service", os.Getpid(), index)
					t.Cleanup(func() { _ = exec.Command("systemctl", "stop", unit).Run() })
					command := processfixture.OwnerTestCommand(t, unit, binary, "TestNativeDescriptorOriginalAuthorityAndJoinTCP", "ARDENTS_DESCRIPTOR_ORIGINAL_FIXTURE", path, 120)
					if output, err := command.CombinedOutput(); err != nil {
						t.Fatalf("genuine original Descriptor composition failed: %v\n%s", err, output)
					}
				})
			}
		})
	}
}

func nativeDescriptorAuthorityChild(t *testing.T, path string) {
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var plan processfixture.DescriptorAuthorityPlan
	if err := json.Unmarshal(raw, &plan); err != nil {
		t.Fatal(err)
	}
	config, err := plan.Network.Config()
	if err != nil {
		t.Fatal(err)
	}
	config.LocalRoleStateRoot = plan.Participation
	owner, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := owner.Close(); err != nil {
			t.Error("original State close", err)
		}
	})
	view, err := owner.CurrentRuntime()
	if err != nil {
		t.Fatal(err)
	}
	member, err := view.Member([32]byte{17}, view.ObservedAt())
	if err != nil {
		t.Fatal(err)
	}
	retained, err := view.RetainDuty(member.NodeID, view.ObservedAt())
	if err != nil {
		t.Fatal(err)
	}
	var armed, held, canceledAtListener atomic.Bool
	entered, resume := make(chan struct{}), make(chan struct{})
	var once sync.Once
	release := func() { once.Do(func() { close(resume) }) }
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	directCaller := &nativeDescriptorCaller{Context: ctx}
	var originalCaller context.Context = ctx
	if plan.Fault == "direct-caller" {
		originalCaller = directCaller
	}
	current := func() (network.RuntimeView, error) {
		actual, err := owner.CurrentRuntime()
		if plan.Fault == "startup-cancel" && !canceledAtListener.Load() && processfixture.OriginalListenerOccupied(t, member) && canceledAtListener.CompareAndSwap(false, true) {
			cancel()
		}
		if armed.Load() && held.CompareAndSwap(false, true) {
			close(entered)
			<-resume
		}
		return actual, err
	}
	original, err := node.ListenDescriptor(originalCaller, node.DescriptorConfig{Authority: role.Authority{Current: current, Duty: retained, Profile: view.Profile().ProfileBinding}, ParticipationRoot: plan.Participation, SpendRoot: plan.Spend, HostingRoot: plan.Hosting, DescriptorRoot: plan.Descriptor, Certificate: plan.Certificate, PrivateKey: plan.PrivateKey, Work: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}})
	binding := spending.Binding{NetworkID: view.Profile().Network, ProfileDigest: view.Profile().Digest, ReceiverNodeID: member.NodeID, ReceiverDutyGeneration: member.DutyGeneration}
	if plan.Fault == "startup-cancel" {
		if original != nil {
			_ = original.Close()
			t.Fatal("late canceled startup handed over accepting Descriptor")
		}
		if !canceledAtListener.Load() || !errors.Is(err, context.Canceled) {
			t.Fatal("late canceled startup lost original cause", err)
		}
		processfixture.RequireFailedStartupCleanup(t, plan.Participation, plan.Spend, plan.Hosting, member, binding)
		store, err := reachability.OpenStore(reachability.StoreConfig{Root: plan.Descriptor, Network: view.Profile().Network})
		if err != nil {
			t.Fatal("rejected partial startup retained Store lease", err)
		}
		if err := store.Close(); err != nil {
			t.Fatal(err)
		}
		return
	}
	if err != nil {
		t.Fatal("genuine Descriptor startup refused", err)
	}
	t.Cleanup(func() { release(); original.Stop(); _ = original.Close() })
	armed.Store(true)
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("original actual State observation did not enter held return")
	}
	originalLock := filepath.Join(plan.Descriptor, ".ardents-reachability-store-lock")
	var restorePath, restoreFrom string
	var networkExpiry error
	switch plan.Fault {
	case "stop":
		stopped := make(chan struct{})
		go func() { original.Stop(); close(stopped) }()
		select {
		case <-stopped:
		case <-time.After(200 * time.Millisecond):
			t.Fatal("synchronous denial held Node state lock across foreign observation")
		}
	case "caller":
		cancel()
	case "direct-caller":
		directCaller.denied.Store(true)
	case "stale-measurement":
		time.Sleep(1200 * time.Millisecond)
	case "successor":
		if _, err := owner.AcceptEpoch(t.Context(), plan.Successor.Epoch.Raw, plan.Successor.Epoch.Inputs, plan.Successor.Epoch.Materials); err != nil {
			t.Fatal(err)
		}
		if _, err := owner.AcceptClosedProfile(plan.Successor.Profile); err != nil {
			t.Fatal(err)
		}
		changed, err := owner.CurrentRuntime()
		if err != nil || changed.MatchDuty(retained, changed.ObservedAt()) == nil {
			t.Fatal("signed successor preserved original authority", err)
		}
	case "profile-conflict":
		if _, err := owner.AcceptClosedProfile(plan.Conflict); err == nil {
			t.Fatal("signed second profile accepted")
		}
		if _, err := owner.CurrentRuntime(); err == nil {
			t.Fatal("signed conflict supplied current authority")
		}
	case "member-expiry":
		time.Sleep(time.Until(member.NotAfter()) + 150*time.Millisecond)
		actual, err := owner.CurrentRuntime()
		if err != nil {
			t.Fatal(err)
		}
		networkExpiry = processfixture.AwaitNativeExpiryDenial(t, actual, retained, func() bool { return original.FirstFailure() != nil })
	case "store-lease-fd-loss":
		processfixture.RequireReceivingRootsHeld(t, plan.Participation, plan.Spend, member, binding, originalLock)
		entries, err := os.ReadDir("/proc/self/fd")
		if err != nil {
			t.Fatal(err)
		}
		fd := -1
		for _, entry := range entries {
			path, err := os.Readlink(filepath.Join("/proc/self/fd", entry.Name()))
			if err == nil && path == originalLock {
				if fd >= 0 {
					t.Fatal("original Store lease descriptor ambiguous")
				}
				fd, err = strconv.Atoi(entry.Name())
				if err != nil {
					t.Fatal(err)
				}
			}
		}
		if fd < 0 {
			t.Fatal("original Store lease descriptor unavailable")
		}
		if err := syscall.Close(fd); err != nil {
			t.Fatal("actual original descriptor loss failed", err)
		}
		// The fixture physically stole that descriptor. Do not pretend the
		// kernel still holds its flock or classify this as an owner release.
		originalLock = ""
	case "store-root-loss", "store-records-loss", "store-marker-loss", "store-lock-loss":
		object := plan.Descriptor
		if plan.Fault == "store-records-loss" {
			object = filepath.Join(object, "records")
		}
		if plan.Fault == "store-marker-loss" {
			object = filepath.Join(object, ".ardents-reachability-store-v3")
		}
		if plan.Fault == "store-lock-loss" {
			object = filepath.Join(object, ".ardents-reachability-store-lock")
		}
		moved := object + ".original"
		if err := os.Rename(object, moved); err != nil {
			t.Fatal("actual native custody loss unavailable", err)
		}
		restorePath, restoreFrom = object, moved
		if plan.Fault == "store-root-loss" {
			originalLock = filepath.Join(moved, ".ardents-reachability-store-lock")
		}
		if plan.Fault == "store-lock-loss" {
			originalLock = moved
		}
	default:
		t.Fatal("unselected Descriptor authority control", plan.Fault)
	}
	select {
	case <-original.Done():
		t.Fatal("original Done reported before held user physically joined")
	case <-time.After(30 * time.Millisecond):
	}
	processfixture.RequireReceivingRootsHeld(t, plan.Participation, plan.Spend, member, binding, originalLock)
	release()
	select {
	case <-original.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("original Descriptor did not join released actual observer")
	}
	result := original.Close()
	if restorePath != "" {
		if err := os.Rename(restoreFrom, restorePath); err != nil {
			t.Fatal(err)
		}
	}
	first, cleanup := original.JoinedFailureOwners()
	expected := "network"
	if plan.Fault == "stop" || plan.Fault == "caller" || plan.Fault == "direct-caller" {
		expected = "caller"
		if plan.Fault == "stop" {
			expected = ""
		}
		if ((plan.Fault == "caller" || plan.Fault == "direct-caller") && !errors.Is(result, context.Canceled)) || (plan.Fault == "stop" && result != nil) || original.JoinedOutcome() != "canceled" {
			t.Fatal("original retirement lost cause or counterfeit cleanup failure", result, original.JoinedOutcome())
		}
	} else {
		if plan.Fault == "stale-measurement" {
			expected = "process"
		}
		if plan.Fault == "member-expiry" {
			expected = nativeExpiryFirstOwner(t, result, networkExpiry, original.FirstFailure())
		}
		if plan.Fault == "store-root-loss" || plan.Fault == "store-records-loss" || plan.Fault == "store-marker-loss" || plan.Fault == "store-lock-loss" || plan.Fault == "store-lease-fd-loss" {
			expected = "reachability"
		}
		if plan.Fault == "store-lease-fd-loss" && (cleanup != "reachability" || !errors.Is(result, syscall.EBADF)) {
			t.Fatal("actual failed physical lease release lost cause/owner", cleanup, result)
		}
		if result == nil || original.JoinedOutcome() != "failed" {
			t.Fatal("original owner loss reported successful withdrawal", result)
		}
	}
	if first != expected {
		t.Fatal("original failure owner changed", first, expected, result)
	}
	copy := *original
	if copy.Done() != original.Done() || copy.Close() != result || copy.JoinedOutcome() != original.JoinedOutcome() {
		t.Fatal("copied handle replaced original result")
	}
	store, err := reachability.OpenStore(reachability.StoreConfig{Root: plan.Descriptor, Network: view.Profile().Network})
	if err != nil {
		t.Fatal("Store not released after actual role join", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	journal, err := duty.OpenOperation(t.Context(), duty.Config{Root: plan.Participation, Clock: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	collision, err := journal.Conflict(member.NodeID, member.FamilyID)
	if closeErr := journal.Close(); err != nil || closeErr != nil || collision {
		t.Fatal("joined producer remains live", collision, err, closeErr)
	}
	ledger, err := spending.Open(plan.Spend, binding)
	if err != nil {
		t.Fatal("joined Descriptor retained receiving root", err)
	}
	if err := ledger.Close(); err != nil {
		t.Fatal(err)
	}
	budget, err := hosting.Open(plan.Hosting)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := budget.Observe(t.Context())
	if closeErr := budget.Close(); err != nil || closeErr != nil || snapshot.ReservedBytes != 0 {
		t.Fatal("original join did not return Hosting", snapshot, err, closeErr)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := state.Open(config)
	if err != nil {
		t.Fatal("State native reopen failed", err)
	}
	defer reopened.Close()
	recovered, err := reopened.CurrentRuntime()
	if plan.Fault == "profile-conflict" {
		if err == nil {
			t.Fatal("conflict disappeared on reopen")
		}
	} else if err != nil {
		t.Fatal(err)
	} else if plan.Fault == "successor" && recovered.MatchDuty(retained, recovered.ObservedAt()) == nil {
		t.Fatal("reopen rebound old duty")
	}
}

// A cancellation-state fixture delays only notification. It supplies no
// successful authority; original Err must deny before derived Done propagates.
type nativeIntroductionCaller struct {
	context.Context
	denied atomic.Bool
}

func (caller *nativeIntroductionCaller) Err() error {
	if caller.denied.Load() {
		return context.Canceled
	}
	return caller.Context.Err()
}

// Native composition holds only a completed genuine State return, never a fake
// accepting observation. The actual command has a separate positive workload.
func TestNativeIntroductionOriginalAuthorityAndJoinTCP(t *testing.T) {
	if path := os.Getenv("ARDENTS_INTRODUCTION_ORIGINAL_FIXTURE"); path != "" {
		nativeIntroductionAuthorityChild(t, path)
		return
	}
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, carrier := range []transport.CarrierProfile{transport.ClosedCarrierTCP} {
		t.Run(string(carrier), func(t *testing.T) {
			for index, fault := range []string{"stop", "caller", "direct-caller", "stale-measurement", "memory-pressure", "process-fd-loss", "successor", "profile-conflict", "member-expiry", "startup-cancel", "history-root-loss", "history-loss", "history-lock-loss", "history-lease-fd-loss"} {
				t.Run(fault, func(t *testing.T) {
					spec, _, _, member, releaseSocket := authorityProcessInput(t, carrier, 25, 4, 3, fault)
					bundle, err := networkfixture.BuildClosed(spec)
					if err != nil {
						t.Fatal(err)
					}
					clock, closeClock := networkinputs.StartObservedClock(t)
					defer closeClock()
					cert, key := processfixture.WriteNativeCredential(t, processfixture.RoleCertificate(t, member.PrivateKey))
					plan := processfixture.IntroductionAuthorityPlan{Network: networkinputs.SeedNetworkState(t, spec.NetworkID, [32]byte(spec.Authority[32:]), bundle, clock), Participation: filepath.Join(t.TempDir(), "participation"), Spend: t.TempDir(), Hosting: processfixture.ProcessBudget(t), Introduction: t.TempDir(), Certificate: cert, PrivateKey: key, Fault: fault}
					if err := os.Chmod(plan.Introduction, 0700); err != nil {
						t.Fatal(err)
					}
					next := spec
					next.Number++
					next.Previous = bundle.Epoch.Digest
					next.Seed[0]++
					plan.Successor, err = networkfixture.BuildClosed(next)
					if err != nil {
						t.Fatal(err)
					}
					conflict := spec
					conflict.IssuanceAuthority[0] ^= 1
					changed, err := networkfixture.BuildClosed(conflict)
					if err != nil || changed.Epoch.Digest != bundle.Epoch.Digest {
						t.Fatal("signed conflict changed original Epoch", err)
					}
					plan.Conflict = changed.Profile
					path := writeAuthorityProcessPlan(t, plan)
					releaseSocket()
					unit := fmt.Sprintf("ardents-introduction-owner-%d-%d.service", os.Getpid(), index)
					t.Cleanup(func() { _ = exec.Command("systemctl", "stop", unit).Run() })
					command := processfixture.OwnerTestCommand(t, unit, binary, "TestNativeIntroductionOriginalAuthorityAndJoinTCP", "ARDENTS_INTRODUCTION_ORIGINAL_FIXTURE", path, 120)
					if output, err := command.CombinedOutput(); err != nil {
						t.Fatalf("genuine original Introduction composition failed: %v\n%s", err, output)
					}
				})
			}
		})
	}
}

func nativeIntroductionAuthorityChild(t *testing.T, path string) {
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var plan processfixture.IntroductionAuthorityPlan
	if err := json.Unmarshal(raw, &plan); err != nil {
		t.Fatal(err)
	}
	config, err := plan.Network.Config()
	if err != nil {
		t.Fatal(err)
	}
	config.LocalRoleStateRoot = plan.Participation
	owner, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := owner.Close(); err != nil {
			t.Error("original State close", err)
		}
	})
	view, err := owner.CurrentRuntime()
	if err != nil {
		t.Fatal(err)
	}
	member, err := view.Member([32]byte{25}, view.ObservedAt())
	if err != nil {
		t.Fatal(err)
	}
	retained, err := view.RetainDuty(member.NodeID, view.ObservedAt())
	if err != nil {
		t.Fatal(err)
	}
	var armed, held, canceledAtListener atomic.Bool
	entered, resume := make(chan struct{}), make(chan struct{})
	var once sync.Once
	release := func() { once.Do(func() { close(resume) }) }
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	directCaller := &nativeIntroductionCaller{Context: ctx}
	var originalCaller context.Context = ctx
	if plan.Fault == "direct-caller" {
		originalCaller = directCaller
	}
	current := func() (network.RuntimeView, error) {
		actual, err := owner.CurrentRuntime()
		if plan.Fault == "startup-cancel" && !canceledAtListener.Load() && processfixture.OriginalListenerOccupied(t, member) && canceledAtListener.CompareAndSwap(false, true) {
			cancel()
		}
		if armed.Load() && held.CompareAndSwap(false, true) {
			close(entered)
			<-resume
		}
		return actual, err
	}
	original, err := node.ListenIntroduction(originalCaller, node.IntroductionConfig{Authority: role.Authority{Current: current, Duty: retained, Profile: view.Profile().ProfileBinding}, ParticipationRoot: plan.Participation, SpendRoot: plan.Spend, HostingRoot: plan.Hosting, IntroductionRoot: plan.Introduction, Certificate: plan.Certificate, PrivateKey: plan.PrivateKey, Work: hosting.Traffic{Tx: 1 << 20, Rx: 1 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}})
	binding := spending.Binding{NetworkID: view.Profile().Network, ProfileDigest: view.Profile().Digest, ReceiverNodeID: member.NodeID, ReceiverDutyGeneration: member.DutyGeneration}
	if plan.Fault == "startup-cancel" {
		if original != nil {
			_ = original.Close()
			t.Fatal("late canceled startup handed over accepting Introduction")
		}
		if !canceledAtListener.Load() || !errors.Is(err, context.Canceled) {
			t.Fatal("late canceled startup lost original cause", err)
		}
		processfixture.RequireFailedStartupCleanup(t, plan.Participation, plan.Spend, plan.Hosting, member, binding)
		history, err := introduction.OpenHistory(plan.Introduction, introduction.Binding{NetworkID: view.Profile().Network, ProfileDigest: view.Profile().Digest, ReceiverNodeID: member.NodeID, ReceiverDutyGeneration: member.DutyGeneration})
		if err != nil {
			t.Fatal("rejected partial startup retained History lease", err)
		}
		if err := history.Close(); err != nil {
			t.Fatal(err)
		}
		return
	}
	if err != nil {
		t.Fatal("genuine Introduction startup refused", err)
	}
	t.Cleanup(func() { release(); original.Stop(); _ = original.Close() })
	armed.Store(true)
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("original actual State observation did not enter held return")
	}
	originalLock := filepath.Join(plan.Introduction, ".ardents-introduction-lock")
	var restorePath, restoreFrom string
	var pressureMemory []byte
	var networkExpiry error
	switch plan.Fault {
	case "stop":
		stopped := make(chan struct{})
		go func() { original.Stop(); close(stopped) }()
		select {
		case <-stopped:
		case <-time.After(200 * time.Millisecond):
			t.Fatal("synchronous denial held Node state lock across foreign observation")
		}
	case "caller":
		cancel()
	case "direct-caller":
		directCaller.denied.Store(true)
	case "stale-measurement":
		time.Sleep(1200 * time.Millisecond)
	case "memory-pressure":
		pressureMemory = make([]byte, 513<<20)
		for index := 0; index < len(pressureMemory); index += os.Getpagesize() {
			pressureMemory[index] = 1
		}
	case "process-fd-loss":
		entries, err := os.ReadDir("/proc/self/fd")
		if err != nil {
			t.Fatal(err)
		}
		fd := -1
		for _, entry := range entries {
			target, err := os.Readlink(filepath.Join("/proc/self/fd", entry.Name()))
			if err == nil && target == filepath.Join("/proc", strconv.Itoa(os.Getpid())) {
				if fd >= 0 {
					t.Fatal("original process descriptor ambiguous")
				}
				fd, err = strconv.Atoi(entry.Name())
				if err != nil {
					t.Fatal(err)
				}
			}
		}
		if fd < 0 {
			t.Fatal("actual original process descriptor absent")
		}
		if err := syscall.Close(fd); err != nil {
			t.Fatal(err)
		}
	case "successor":
		if _, err := owner.AcceptEpoch(t.Context(), plan.Successor.Epoch.Raw, plan.Successor.Epoch.Inputs, plan.Successor.Epoch.Materials); err != nil {
			t.Fatal(err)
		}
		if _, err := owner.AcceptClosedProfile(plan.Successor.Profile); err != nil {
			t.Fatal(err)
		}
		changed, err := owner.CurrentRuntime()
		if err != nil || changed.MatchDuty(retained, changed.ObservedAt()) == nil {
			t.Fatal("signed successor preserved original authority", err)
		}
	case "profile-conflict":
		if _, err := owner.AcceptClosedProfile(plan.Conflict); err == nil {
			t.Fatal("signed second profile accepted")
		}
		if _, err := owner.CurrentRuntime(); err == nil {
			t.Fatal("signed conflict supplied current authority")
		}
	case "member-expiry":
		time.Sleep(time.Until(member.NotAfter()) + 150*time.Millisecond)
		actual, err := owner.CurrentRuntime()
		if err != nil {
			t.Fatal(err)
		}
		networkExpiry = processfixture.AwaitNativeExpiryDenial(t, actual, retained, func() bool { return original.FirstFailure() != nil })
	case "history-lease-fd-loss":
		processfixture.RequireReceivingRootsHeld(t, plan.Participation, plan.Spend, member, binding, originalLock)
		entries, err := os.ReadDir("/proc/self/fd")
		if err != nil {
			t.Fatal(err)
		}
		fd := -1
		for _, entry := range entries {
			path, err := os.Readlink(filepath.Join("/proc/self/fd", entry.Name()))
			if err == nil && path == originalLock {
				if fd >= 0 {
					t.Fatal("original History lease descriptor ambiguous")
				}
				fd, err = strconv.Atoi(entry.Name())
				if err != nil {
					t.Fatal(err)
				}
			}
		}
		if fd < 0 {
			t.Fatal("original History lease descriptor unavailable")
		}
		if err := syscall.Close(fd); err != nil {
			t.Fatal("actual original descriptor loss failed", err)
		}
		// The fixture physically stole that descriptor. Do not pretend the
		// kernel still holds its flock or classify this as an owner release.
		originalLock = ""
	case "history-root-loss", "history-loss", "history-lock-loss":
		object := plan.Introduction
		if plan.Fault == "history-loss" {
			object = filepath.Join(object, "closed-introduction-slots")
		}
		if plan.Fault == "history-lock-loss" {
			object = filepath.Join(object, ".ardents-introduction-lock")
		}
		moved := object + ".original"
		if err := os.Rename(object, moved); err != nil {
			t.Fatal("actual native custody loss unavailable", err)
		}
		restorePath, restoreFrom = object, moved
		if plan.Fault == "history-root-loss" {
			originalLock = filepath.Join(moved, ".ardents-introduction-lock")
		}
		if plan.Fault == "history-lock-loss" {
			originalLock = moved
		}
	default:
		t.Fatal("unselected Introduction authority control", plan.Fault)
	}
	select {
	case <-original.Done():
		t.Fatal("original Done reported before held user physically joined")
	case <-time.After(30 * time.Millisecond):
	}
	processfixture.RequireReceivingRootsHeld(t, plan.Participation, plan.Spend, member, binding, originalLock)
	release()
	select {
	case <-original.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("original Introduction did not join released actual observer")
	}
	result := original.Close()
	runtime.KeepAlive(pressureMemory)
	pressureMemory = nil
	runtime.GC()
	if restorePath != "" {
		if err := os.Rename(restoreFrom, restorePath); err != nil {
			t.Fatal(err)
		}
	}
	first, cleanup := original.JoinedFailureOwners()
	expected := "network"
	if plan.Fault == "stop" || plan.Fault == "caller" || plan.Fault == "direct-caller" {
		expected = "caller"
		if plan.Fault == "stop" {
			expected = ""
		}
		if ((plan.Fault == "caller" || plan.Fault == "direct-caller") && !errors.Is(result, context.Canceled)) || (plan.Fault == "stop" && result != nil) || original.JoinedOutcome() != "canceled" {
			t.Fatal("original retirement lost cause or counterfeit cleanup failure", result, original.JoinedOutcome())
		}
	} else {
		if plan.Fault == "stale-measurement" || plan.Fault == "memory-pressure" || plan.Fault == "process-fd-loss" {
			expected = "process"
		}
		if plan.Fault == "member-expiry" {
			expected = nativeExpiryFirstOwner(t, result, networkExpiry, original.FirstFailure())
		}
		if plan.Fault == "history-root-loss" || plan.Fault == "history-loss" || plan.Fault == "history-lock-loss" || plan.Fault == "history-lease-fd-loss" {
			expected = "introduction"
		}
		if plan.Fault == "history-lease-fd-loss" && (cleanup != "introduction" || !errors.Is(result, syscall.EBADF)) {
			t.Fatal("actual failed physical lease release lost cause/owner", cleanup, result)
		}
		if result == nil || original.JoinedOutcome() != "failed" {
			t.Fatal("original owner loss reported successful withdrawal", result)
		}
	}
	if plan.Fault == "process-fd-loss" && (cleanup != "process" || !errors.Is(result, syscall.EBADF)) {
		t.Fatal("actual process physical release failure lost", cleanup, result)
	}
	if first != expected {
		t.Fatal("original failure owner changed", first, expected, result)
	}
	copy := *original
	if copy.Done() != original.Done() || copy.Close() != result || copy.JoinedOutcome() != original.JoinedOutcome() {
		t.Fatal("copied handle replaced original result")
	}
	history, err := introduction.OpenHistory(plan.Introduction, introduction.Binding{NetworkID: view.Profile().Network, ProfileDigest: view.Profile().Digest, ReceiverNodeID: member.NodeID, ReceiverDutyGeneration: member.DutyGeneration})
	if err != nil {
		t.Fatal("History not released after actual role join", err)
	}
	if err := history.Close(); err != nil {
		t.Fatal(err)
	}
	journal, err := duty.OpenOperation(t.Context(), duty.Config{Root: plan.Participation, Clock: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	collision, err := journal.Conflict(member.NodeID, member.FamilyID)
	if closeErr := journal.Close(); err != nil || closeErr != nil || collision {
		t.Fatal("joined producer remains live", collision, err, closeErr)
	}
	ledger, err := spending.Open(plan.Spend, binding)
	if err != nil {
		t.Fatal("joined Introduction retained receiving root", err)
	}
	if err := ledger.Close(); err != nil {
		t.Fatal(err)
	}
	budget, err := hosting.Open(plan.Hosting)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := budget.Observe(t.Context())
	if closeErr := budget.Close(); err != nil || closeErr != nil || snapshot.ReservedBytes != 0 {
		t.Fatal("original join did not return Hosting", snapshot, err, closeErr)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := state.Open(config)
	if err != nil {
		t.Fatal("State native reopen failed", err)
	}
	defer reopened.Close()
	recovered, err := reopened.CurrentRuntime()
	if plan.Fault == "profile-conflict" {
		if err == nil {
			t.Fatal("conflict disappeared on reopen")
		}
	} else if err != nil {
		t.Fatal(err)
	} else if plan.Fault == "successor" && recovered.MatchDuty(retained, recovered.ObservedAt()) == nil {
		t.Fatal("reopen rebound old duty")
	}
}

// This is a native composition test, not a replacement command or installed
// qualification. The ordinary command scenarios separately prove that consumer.
// The root driver prepares signed inputs outside the measured cgroup; the child
// opens genuine original State/Node/participation/Admission/Hosting owners.
// The only injected mechanism is scheduling the return of an actual completed
// State observation. No successful authority, role or native reading is supplied.
func TestNativeIssuerOriginalAuthorityAndJoinTCP(t *testing.T) {
	if path := os.Getenv("ARDENTS_ISSUER_ORIGINAL_FIXTURE"); path != "" {
		testNativeIssuerAuthorityChild(t, path)
		return
	}
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, carrier := range []transport.CarrierProfile{transport.ClosedCarrierTCP} {
		t.Run(string(carrier), func(t *testing.T) {
			for index, fault := range []string{"stop", "caller", "stale-measurement", "successor", "profile-conflict", "member-expiry", "startup-cancel"} {
				t.Run(fault, func(t *testing.T) {
					spec, issuerPlan, inventory, member, releaseSocket := authorityProcessInput(t, carrier, 11, 2, 6, fault)
					bundle, err := networkfixture.BuildClosed(spec)
					if err != nil {
						t.Fatal(err)
					}
					clock, closeClock := networkinputs.StartObservedClock(t)
					defer closeClock()
					plan := processfixture.IssuerAuthorityPlan{Network: networkinputs.SeedNetworkState(t, spec.NetworkID, [32]byte(spec.Authority[32:]), bundle, clock), Participation: filepath.Join(t.TempDir(), "participation"), Spend: t.TempDir(), Hosting: processfixture.ProcessBudget(t), Fault: fault}
					networkinputs.PrepareIssuer(t, &issuerPlan, inventory, spec, member, sha256.Sum256(bundle.Profile))
					plan.Issuance = issuerPlan
					plan.Certificate, plan.PrivateKey = processfixture.WriteNativeCredential(t, processfixture.RoleCertificate(t, member.PrivateKey))
					next := spec
					next.Number++
					next.Previous = bundle.Epoch.Digest
					next.Seed[0]++
					plan.Successor, err = networkfixture.BuildClosed(next)
					if err != nil {
						t.Fatal(err)
					}
					conflict := spec
					conflict.IssuanceAuthority[0] ^= 1
					changed, err := networkfixture.BuildClosed(conflict)
					if err != nil || changed.Epoch.Digest != bundle.Epoch.Digest {
						t.Fatal("conflict changed the original Epoch", err)
					}
					plan.Conflict = changed.Profile
					path := writeAuthorityProcessPlan(t, plan)
					releaseSocket()
					unit := fmt.Sprintf("ardents-issuer-owner-%d-%d.service", os.Getpid(), index)
					t.Cleanup(func() { _ = exec.Command("systemctl", "stop", unit).Run() })
					command := processfixture.OwnerTestCommand(t, unit, binary, "TestNativeIssuerOriginalAuthorityAndJoinTCP", "ARDENTS_ISSUER_ORIGINAL_FIXTURE", path, 120)
					output, err := command.CombinedOutput()
					if err != nil {
						t.Fatalf("genuine original native composition failed: %v\n%s", err, output)
					}
				})
			}
		})
	}
}

func testNativeIssuerAuthorityChild(t *testing.T, path string) {
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var plan processfixture.IssuerAuthorityPlan
	if err := json.Unmarshal(raw, &plan); err != nil {
		t.Fatal(err)
	}
	config, err := plan.Network.Config()
	if err != nil {
		t.Fatal(err)
	}
	config.LocalRoleStateRoot = plan.Participation
	owner, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := owner.Close(); err != nil {
			t.Error("original State close", err)
		}
	})
	view, err := owner.CurrentRuntime()
	if err != nil {
		t.Fatal(err)
	}
	member, err := view.Member([32]byte{11}, view.ObservedAt())
	if err != nil {
		t.Fatal(err)
	}
	retained, err := view.RetainDuty(member.NodeID, view.ObservedAt())
	if err != nil {
		t.Fatal(err)
	}
	var armed, held atomic.Bool
	var canceledAtListener atomic.Bool
	entered, resume := make(chan struct{}), make(chan struct{})
	var resumeOnce sync.Once
	release := func() { resumeOnce.Do(func() { close(resume) }) }
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	current := func() (network.RuntimeView, error) {
		actual, err := owner.CurrentRuntime()
		if plan.Fault == "startup-cancel" && !canceledAtListener.Load() && processfixture.OriginalListenerOccupied(t, member) && canceledAtListener.CompareAndSwap(false, true) {
			// Cancel after an actual listener effect and a completed real State
			// read, before the original consumer's successful final handoff.
			cancel()
		}
		if armed.Load() && held.CompareAndSwap(false, true) {
			close(entered)
			<-resume
		}
		return actual, err
	}
	original, err := node.ListenIssuer(ctx, node.IssuerConfig{Authority: role.Authority{Current: current, Duty: retained, Profile: view.Profile().ProfileBinding},
		ParticipationRoot: plan.Participation, SpendRoot: plan.Spend, HostingRoot: plan.Hosting, Certificate: plan.Certificate, PrivateKey: plan.PrivateKey,
		Issuance: plan.Issuance, Work: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}, BootstrapWork: hosting.Traffic{Tx: 128 << 10, Rx: 128 << 10}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}})
	binding := spending.Binding{NetworkID: view.Profile().Network, ProfileDigest: view.Profile().Digest, ReceiverNodeID: member.NodeID, ReceiverDutyGeneration: member.DutyGeneration}
	if plan.Fault == "startup-cancel" {
		if original != nil {
			_ = original.Close()
			t.Fatal("late canceled acquisition returned an accepting original")
		}
		if !canceledAtListener.Load() || !errors.Is(err, context.Canceled) {
			t.Fatal("late startup cancellation lost its original cause", err)
		}
		processfixture.RequireFailedStartupCleanup(t, plan.Participation, plan.Spend, plan.Hosting, member, binding)
		processfixture.RequireIssuerRootsReleased(t, plan.Issuance)
		return
	}
	if err != nil {
		t.Fatal("original native Node refused real startup", err)
	}
	t.Cleanup(func() { release(); original.Stop(); _ = original.Close() })
	armed.Store(true)
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("real original State observation never entered the held return")
	}
	var networkExpiry error
	switch plan.Fault {
	case "stop":
		stopped := make(chan struct{})
		go func() { original.Stop(); close(stopped) }()
		select {
		case <-stopped:
		case <-time.After(200 * time.Millisecond):
			t.Fatal("synchronous Node denial waited for foreign authority I/O")
		}
	case "caller":
		cancel()
	case "stale-measurement":
		timer := time.NewTimer(1200 * time.Millisecond)
		select {
		case <-timer.C:
		case <-t.Context().Done():
			t.Fatal(t.Context().Err())
		}
		timer.Stop()
	case "successor":
		if _, err := owner.AcceptEpoch(t.Context(), plan.Successor.Epoch.Raw, plan.Successor.Epoch.Inputs, plan.Successor.Epoch.Materials); err != nil {
			t.Fatal(err)
		}
		if _, err := owner.AcceptClosedProfile(plan.Successor.Profile); err != nil {
			t.Fatal(err)
		}
		changed, err := owner.CurrentRuntime()
		if err != nil || changed.MatchDuty(retained, changed.ObservedAt()) == nil {
			t.Fatal("genuine successor preserved original duty", err)
		}
	case "profile-conflict":
		if _, err := owner.AcceptClosedProfile(plan.Conflict); err == nil {
			t.Fatal("signed second profile accepted as original")
		}
		if _, err := owner.CurrentRuntime(); err == nil {
			t.Fatal("genuine State conflict supplied authority")
		}
	case "member-expiry":
		timer := time.NewTimer(time.Until(member.NotAfter()) + 150*time.Millisecond)
		select {
		case <-timer.C:
		case <-t.Context().Done():
			t.Fatal(t.Context().Err())
		}
		timer.Stop()
		actual, err := owner.CurrentRuntime()
		if err != nil {
			t.Fatal(err)
		}
		networkExpiry = processfixture.AwaitNativeExpiryDenial(t, actual, retained, func() bool { return original.FirstFailure() != nil })
	default:
		t.Fatal("unselected original authority fault", plan.Fault)
	}
	// A delayed original watcher is still a physical borrower. A bounded wait
	// cannot finish the Node, remove its producer or release its spend root.
	select {
	case <-original.Done():
		t.Fatal("original completion was published before held work joined")
	case <-time.After(30 * time.Millisecond):
	}
	journal, err := duty.OpenOperation(t.Context(), duty.Config{Root: plan.Participation, Clock: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	collision, err := journal.Conflict(member.NodeID, member.FamilyID)
	if closeErr := journal.Close(); err != nil || closeErr != nil || !collision {
		t.Fatal("unjoined original producer disappeared", collision, err, closeErr)
	}
	if ledger, err := spending.Open(plan.Spend, binding); err == nil {
		_ = ledger.Close()
		t.Fatal("unjoined Node returned the original spend root")
	}
	processfixture.RequireIssuerRootsHeld(t, plan.Issuance)
	release()
	select {
	case <-original.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("original role did not join after releasing actual observation return")
	}
	result := original.Close()
	processfixture.RequireIssuerRootsReleased(t, plan.Issuance)
	first, _ := original.JoinedFailureOwners()
	expected := "network"
	if plan.Fault == "stop" || plan.Fault == "caller" {
		expected = "caller"
		if plan.Fault == "stop" {
			expected = ""
		}
		if (plan.Fault == "caller" && !errors.Is(result, context.Canceled)) || (plan.Fault == "stop" && result != nil) || original.JoinedOutcome() != "canceled" {
			t.Fatal("original cancellation lost its cause or retained a counterfeit cleanup failure", result, original.JoinedOutcome())
		}
	} else {
		if plan.Fault == "stale-measurement" {
			expected = "process"
		}
		if plan.Fault == "member-expiry" {
			expected = nativeExpiryFirstOwner(t, result, networkExpiry, original.FirstFailure())
		}
		if result == nil || original.JoinedOutcome() != "failed" {
			t.Fatal("original authority/resource retirement reported success", result)
		}
	}
	if first != expected {
		t.Fatal("original first-failure owner changed", first, expected, result)
	}
	copy := *original
	if copy.Done() != original.Done() || copy.Close() != result || copy.JoinedOutcome() != original.JoinedOutcome() {
		t.Fatal("late copied handle changed the original result")
	}
	journal, err = duty.OpenOperation(t.Context(), duty.Config{Root: plan.Participation, Clock: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	collision, err = journal.Conflict(member.NodeID, member.FamilyID)
	if closeErr := journal.Close(); err != nil || closeErr != nil || collision {
		t.Fatal("joined producer remains live", collision, err, closeErr)
	}
	ledger, err := spending.Open(plan.Spend, binding)
	if err != nil {
		t.Fatal("original physical join did not release spend root", err)
	}
	if err := ledger.Close(); err != nil {
		t.Fatal(err)
	}
	budget, err := hosting.Open(plan.Hosting)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := budget.Observe(t.Context())
	if closeErr := budget.Close(); err != nil || closeErr != nil || snapshot.ReservedBytes != 0 {
		t.Fatal("original join changed capacity", snapshot, err, closeErr)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := state.Open(config)
	if err != nil {
		t.Fatal("original State root failed genuine reopen", err)
	}
	defer reopened.Close()
	recovered, err := reopened.CurrentRuntime()
	if plan.Fault == "profile-conflict" {
		if err == nil {
			t.Fatal("profile conflict vanished on reopen")
		}
	} else if err != nil {
		t.Fatal(err)
	} else if plan.Fault == "successor" && recovered.MatchDuty(retained, recovered.ObservedAt()) == nil {
		t.Fatal("original duty rebound to recovered successor")
	}
}

// A cancellation-state fixture delays only notification. It supplies no
// successful authority; original Err must deny before derived Done propagates.
type nativeJoinCaller struct {
	context.Context
	denied atomic.Bool
}

func (caller *nativeJoinCaller) Err() error {
	if caller.denied.Load() {
		return context.Canceled
	}
	return caller.Context.Err()
}

// Native composition holds only a completed genuine State return, never a fake
// accepting observation. The actual command has a separate positive workload.
func TestNativeJoinOriginalAuthorityAndJoinTCP(t *testing.T) {
	if path := os.Getenv("ARDENTS_JOIN_ORIGINAL_FIXTURE"); path != "" {
		nativeJoinAuthorityChild(t, path)
		return
	}
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, carrier := range []transport.CarrierProfile{transport.ClosedCarrierTCP} {
		t.Run(string(carrier), func(t *testing.T) {
			for index, fault := range []string{"stop", "caller", "direct-caller", "stale-measurement", "memory-pressure", "process-fd-loss", "successor", "profile-conflict", "member-expiry", "startup-cancel", "credential-loss", "participation-loss"} {
				t.Run(fault, func(t *testing.T) {
					spec, _, _, member, releaseSocket := authorityProcessInput(t, carrier, 20, 2, 4, fault)
					bundle, err := networkfixture.BuildClosed(spec)
					if err != nil {
						t.Fatal(err)
					}
					clock, closeClock := networkinputs.StartObservedClock(t)
					defer closeClock()
					cert, key := processfixture.WriteNativeCredential(t, processfixture.RoleCertificate(t, member.PrivateKey))
					plan := processfixture.JoinAuthorityPlan{Network: networkinputs.SeedNetworkState(t, spec.NetworkID, [32]byte(spec.Authority[32:]), bundle, clock), Participation: filepath.Join(t.TempDir(), "participation"), Spend: t.TempDir(), Hosting: processfixture.ProcessBudget(t), Certificate: cert, PrivateKey: key, Fault: fault}
					next := spec
					next.Number++
					next.Previous = bundle.Epoch.Digest
					next.Seed[0]++
					plan.Successor, err = networkfixture.BuildClosed(next)
					if err != nil {
						t.Fatal(err)
					}
					conflict := spec
					conflict.IssuanceAuthority[0] ^= 1
					changed, err := networkfixture.BuildClosed(conflict)
					if err != nil || changed.Epoch.Digest != bundle.Epoch.Digest {
						t.Fatal("signed conflict changed original Epoch", err)
					}
					plan.Conflict = changed.Profile
					path := writeAuthorityProcessPlan(t, plan)
					releaseSocket()
					unit := fmt.Sprintf("ardents-join-owner-%d-%d.service", os.Getpid(), index)
					t.Cleanup(func() { _ = exec.Command("systemctl", "stop", unit).Run() })
					command := processfixture.OwnerTestCommand(t, unit, binary, "TestNativeJoinOriginalAuthorityAndJoinTCP", "ARDENTS_JOIN_ORIGINAL_FIXTURE", path, 120)
					if output, err := command.CombinedOutput(); err != nil {
						t.Fatalf("genuine original Join composition failed: %v\n%s", err, output)
					}
				})
			}
		})
	}
}

func nativeJoinAuthorityChild(t *testing.T, path string) {
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var plan processfixture.JoinAuthorityPlan
	if err := json.Unmarshal(raw, &plan); err != nil {
		t.Fatal(err)
	}
	config, err := plan.Network.Config()
	if err != nil {
		t.Fatal(err)
	}
	config.LocalRoleStateRoot = plan.Participation
	owner, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := owner.Close(); err != nil {
			t.Error("original State close", err)
		}
	})
	view, err := owner.CurrentRuntime()
	if err != nil {
		t.Fatal(err)
	}
	member, err := view.Member([32]byte{20}, view.ObservedAt())
	if err != nil {
		t.Fatal(err)
	}
	retained, err := view.RetainDuty(member.NodeID, view.ObservedAt())
	if err != nil {
		t.Fatal(err)
	}
	var armed, held, canceledAtListener atomic.Bool
	entered, resume := make(chan struct{}), make(chan struct{})
	var once sync.Once
	release := func() { once.Do(func() { close(resume) }) }
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	directCaller := &nativeJoinCaller{Context: ctx}
	var originalCaller context.Context = ctx
	if plan.Fault == "direct-caller" {
		originalCaller = directCaller
	}
	current := func() (network.RuntimeView, error) {
		actual, err := owner.CurrentRuntime()
		if plan.Fault == "startup-cancel" && !canceledAtListener.Load() && processfixture.OriginalListenerOccupied(t, member) && canceledAtListener.CompareAndSwap(false, true) {
			cancel()
		}
		if armed.Load() && held.CompareAndSwap(false, true) {
			close(entered)
			<-resume
		}
		return actual, err
	}
	original, err := node.ListenJoin(originalCaller, node.JoinConfig{Authority: role.Authority{Current: current, Duty: retained, Profile: view.Profile().ProfileBinding}, ParticipationRoot: plan.Participation, SpendRoot: plan.Spend, HostingRoot: plan.Hosting, Certificate: plan.Certificate, PrivateKey: plan.PrivateKey, Work: hosting.Traffic{Tx: 32 << 20, Rx: 32 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}})
	binding := spending.Binding{NetworkID: view.Profile().Network, ProfileDigest: view.Profile().Digest, ReceiverNodeID: member.NodeID, ReceiverDutyGeneration: member.DutyGeneration}
	if plan.Fault == "startup-cancel" {
		if original != nil {
			_ = original.Close()
			t.Fatal("late canceled startup handed over accepting Join")
		}
		if !canceledAtListener.Load() || !errors.Is(err, context.Canceled) {
			t.Fatal("late canceled startup lost original cause", err)
		}
		processfixture.RequireFailedStartupCleanup(t, plan.Participation, plan.Spend, plan.Hosting, member, binding)
		return
	}
	if err != nil {
		t.Fatal("genuine Join startup refused", err)
	}
	t.Cleanup(func() { release(); original.Stop(); _ = original.Close() })
	armed.Store(true)
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("original actual State observation did not enter held return")
	}
	var restorePath, restoreFrom string
	var pressureMemory []byte
	var networkExpiry error
	switch plan.Fault {
	case "stop":
		stopped := make(chan struct{})
		go func() { original.Stop(); close(stopped) }()
		select {
		case <-stopped:
		case <-time.After(200 * time.Millisecond):
			t.Fatal("synchronous denial held Node state lock across foreign observation")
		}
	case "caller":
		cancel()
	case "direct-caller":
		directCaller.denied.Store(true)
	case "stale-measurement":
		time.Sleep(1200 * time.Millisecond)
	case "memory-pressure":
		pressureMemory = make([]byte, 513<<20)
		for index := 0; index < len(pressureMemory); index += os.Getpagesize() {
			pressureMemory[index] = 1
		}
	case "process-fd-loss":
		entries, err := os.ReadDir("/proc/self/fd")
		if err != nil {
			t.Fatal(err)
		}
		fd := -1
		for _, entry := range entries {
			target, err := os.Readlink(filepath.Join("/proc/self/fd", entry.Name()))
			if err == nil && target == filepath.Join("/proc", strconv.Itoa(os.Getpid())) {
				if fd >= 0 {
					t.Fatal("original process descriptor ambiguous")
				}
				fd, err = strconv.Atoi(entry.Name())
				if err != nil {
					t.Fatal(err)
				}
			}
		}
		if fd < 0 {
			t.Fatal("actual original process descriptor absent")
		}
		if err := syscall.Close(fd); err != nil {
			t.Fatal(err)
		}
	case "successor":
		if _, err := owner.AcceptEpoch(t.Context(), plan.Successor.Epoch.Raw, plan.Successor.Epoch.Inputs, plan.Successor.Epoch.Materials); err != nil {
			t.Fatal(err)
		}
		if _, err := owner.AcceptClosedProfile(plan.Successor.Profile); err != nil {
			t.Fatal(err)
		}
		changed, err := owner.CurrentRuntime()
		if err != nil || changed.MatchDuty(retained, changed.ObservedAt()) == nil {
			t.Fatal("signed successor preserved original authority", err)
		}
	case "profile-conflict":
		if _, err := owner.AcceptClosedProfile(plan.Conflict); err == nil {
			t.Fatal("signed second profile accepted")
		}
		if _, err := owner.CurrentRuntime(); err == nil {
			t.Fatal("signed conflict supplied current authority")
		}
	case "member-expiry":
		time.Sleep(time.Until(member.NotAfter()) + 150*time.Millisecond)
		actual, err := owner.CurrentRuntime()
		if err != nil {
			t.Fatal(err)
		}
		networkExpiry = processfixture.AwaitNativeExpiryDenial(t, actual, retained, func() bool { return original.FirstFailure() != nil })
	case "credential-loss", "participation-loss":
		object := plan.PrivateKey
		if plan.Fault == "participation-loss" {
			object = plan.Participation
		}
		moved := object + ".original"
		if err := os.Rename(object, moved); err != nil {
			t.Fatal(err)
		}
		restorePath, restoreFrom = object, moved
	default:
		t.Fatal("unselected Join authority control", plan.Fault)
	}
	select {
	case <-original.Done():
		t.Fatal("original Done reported before held user physically joined")
	case <-time.After(30 * time.Millisecond):
	}
	heldPlan := plan
	if plan.Fault == "participation-loss" {
		heldPlan.Participation = restoreFrom
	}
	processfixture.RequireReceivingRootsHeld(t, heldPlan.Participation, heldPlan.Spend, member, binding, "")
	release()
	select {
	case <-original.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("original Join did not join released actual observer")
	}
	result := original.Close()
	runtime.KeepAlive(pressureMemory)
	pressureMemory = nil
	runtime.GC()
	if restorePath != "" {
		if err := os.Rename(restoreFrom, restorePath); err != nil {
			t.Fatal(err)
		}
	}
	first, cleanup := original.JoinedFailureOwners()
	expected := "network"
	if plan.Fault == "stop" || plan.Fault == "caller" || plan.Fault == "direct-caller" {
		expected = "caller"
		if plan.Fault == "stop" {
			expected = ""
		}
		if ((plan.Fault == "caller" || plan.Fault == "direct-caller") && !errors.Is(result, context.Canceled)) || (plan.Fault == "stop" && result != nil) || original.JoinedOutcome() != "canceled" {
			t.Fatal("original retirement lost cause or counterfeit cleanup failure", result, original.JoinedOutcome())
		}
	} else {
		if plan.Fault == "stale-measurement" || plan.Fault == "memory-pressure" || plan.Fault == "process-fd-loss" {
			expected = "process"
		}
		if plan.Fault == "member-expiry" {
			expected = nativeExpiryFirstOwner(t, result, networkExpiry, original.FirstFailure())
		}
		if plan.Fault == "credential-loss" {
			expected = "credential"
		}
		if plan.Fault == "participation-loss" {
			expected = "participation"
		}
		if result == nil || original.JoinedOutcome() != "failed" {
			t.Fatal("original owner loss reported successful withdrawal", result)
		}
	}
	if plan.Fault == "process-fd-loss" && (cleanup != "process" || !errors.Is(result, syscall.EBADF)) {
		t.Fatal("actual process physical release failure lost", cleanup, result)
	}
	if first != expected {
		t.Fatal("original failure owner changed", first, expected, result)
	}
	copy := *original
	if copy.Done() != original.Done() || copy.Close() != result || copy.JoinedOutcome() != original.JoinedOutcome() {
		t.Fatal("copied handle replaced original result")
	}
	journal, err := duty.OpenOperation(t.Context(), duty.Config{Root: plan.Participation, Clock: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	collision, err := journal.Conflict(member.NodeID, member.FamilyID)
	if closeErr := journal.Close(); err != nil || closeErr != nil || collision != (plan.Fault == "participation-loss") {
		t.Fatal("joined producer remains live", collision, err, closeErr)
	}
	ledger, err := spending.Open(plan.Spend, binding)
	if err != nil {
		t.Fatal("joined Join retained receiving root", err)
	}
	if err := ledger.Close(); err != nil {
		t.Fatal(err)
	}
	budget, err := hosting.Open(plan.Hosting)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := budget.Observe(t.Context())
	if closeErr := budget.Close(); err != nil || closeErr != nil || snapshot.ReservedBytes != 0 {
		t.Fatal("original join did not return Hosting", snapshot, err, closeErr)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := state.Open(config)
	if err != nil {
		t.Fatal("State native reopen failed", err)
	}
	defer reopened.Close()
	recovered, err := reopened.CurrentRuntime()
	if plan.Fault == "profile-conflict" {
		if err == nil {
			t.Fatal("conflict disappeared on reopen")
		}
	} else if err != nil {
		t.Fatal(err)
	} else if plan.Fault == "successor" && recovered.MatchDuty(retained, recovered.ObservedAt()) == nil {
		t.Fatal("reopen rebound old duty")
	}
}
