//go:build linux && node_process_native

package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission/issuance"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/quota"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/spending"
	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/network/duty"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state"
	"github.com/dianabuilds/ardents-network/internal/successor/node"
	"github.com/dianabuilds/ardents-network/internal/successor/route/role"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

// This is a native composition test, not a replacement command or installed
// qualification. The ordinary command scenarios separately prove that consumer.
// The root driver prepares signed inputs outside the measured cgroup; the child
// opens genuine original State/Node/participation/Admission/Hosting owners.
// The only injected mechanism is scheduling the return of an actual completed
// State observation. No successful authority, role or native reading is supplied.
type nativeIssuerAuthorityPlan struct {
	Network                        networkAuthorityPlan
	Participation, Spend, Hosting  string
	Certificate, PrivateKey, Fault string
	Successor                      networkfixture.Closed
	Conflict                       []byte
	Issuance                       issuer.Plan
}

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
					f, sockets, certificates := newRoleRouteFixtureConfigured(t, carrier, 1, false, true, func(f *networkAdmissionFixture, _ map[[32]byte]func(), _ map[[32]byte]tls.Certificate) {
						if fault == "member-expiry" {
							for index := range f.spec.Nodes {
								if f.spec.Nodes[index].NodeID == [32]byte{11} {
									f.spec.Nodes[index].ValidUntil = time.Now().Add(5 * time.Second).UTC().Truncate(time.Second)
								}
							}
						}
					})
					bundle, err := networkfixture.BuildClosed(f.spec)
					if err != nil {
						t.Fatal(err)
					}
					clock, closeClock := joinProcessClock(t)
					defer closeClock()
					plan := nativeIssuerAuthorityPlan{Network: seedNativeNodeState(t, f, bundle, clock), Participation: filepath.Join(t.TempDir(), "participation"), Spend: t.TempDir(), Hosting: routeProcessBudget(t), Certificate: filepath.Join(t.TempDir(), "certificate"), PrivateKey: filepath.Join(t.TempDir(), "key"), Fault: fault}
					plan.Issuance = f.plan
					certificate := certificates[[32]byte{11}]
					if err := os.WriteFile(plan.Certificate, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate.Certificate[0]}), 0600); err != nil {
						t.Fatal(err)
					}
					key, err := x509.MarshalPKCS8PrivateKey(certificate.PrivateKey)
					if err != nil {
						t.Fatal(err)
					}
					encoded := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key})
					if err := os.WriteFile(plan.PrivateKey, encoded, 0600); err != nil {
						t.Fatal(err)
					}
					clear(key)
					clear(encoded)
					next := f.spec
					next.Number++
					next.Previous = bundle.Epoch.Digest
					next.Seed[0]++
					plan.Successor, err = networkfixture.BuildClosed(next)
					if err != nil {
						t.Fatal(err)
					}
					conflict := f.spec
					conflict.IssuanceAuthority[0] ^= 1
					changed, err := networkfixture.BuildClosed(conflict)
					if err != nil || changed.Epoch.Digest != bundle.Epoch.Digest {
						t.Fatal("conflict changed the original Epoch", err)
					}
					plan.Conflict = changed.Profile
					path := hostingConfig(t, plan)
					sockets[[32]byte{11}]()
					unit := fmt.Sprintf("ardents-issuer-owner-%d-%d.service", os.Getpid(), index)
					t.Cleanup(func() { _ = exec.Command("systemctl", "stop", unit).Run() })
					command := exec.CommandContext(t.Context(), "systemd-run", "--quiet", "--wait", "--pipe", "--collect", "--unit="+unit,
						"--property=CPUQuota=100%", "--property=CPUQuotaPeriodSec=100ms", "--property=MemoryMax=1G", "--property=MemoryHigh=768M", "--property=TasksMax=256", "--property=LimitNOFILE=1024", "--property=RuntimeMaxSec=120s",
						"--setenv=GOMAXPROCS=1", "--setenv=GOMEMLIMIT=512MiB", "--setenv=ARDENTS_ISSUER_ORIGINAL_FIXTURE="+path,
						"--", binary, "-test.v=true", "-test.run=^TestNativeIssuerOriginalAuthorityAndJoinTCP$", "-test.timeout=30s")
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
	var plan nativeIssuerAuthorityPlan
	if err := json.Unmarshal(raw, &plan); err != nil {
		t.Fatal(err)
	}
	config, err := networkStateConfig(&plan.Network)
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
		if plan.Fault == "startup-cancel" && !canceledAtListener.Load() && nativeOriginalListenerOccupied(t, member) && canceledAtListener.CompareAndSwap(false, true) {
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
		checkNativeFailedStartupCleanup(t, nativeOriginalAuthorityPlan{Network: plan.Network, Participation: plan.Participation, Spend: plan.Spend, Hosting: plan.Hosting, Certificate: plan.Certificate, PrivateKey: plan.PrivateKey}, member, binding)
		checkNativeIssuerRootsReleased(t, plan.Issuance)
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
		if _, err := owner.Accept(t.Context(), plan.Successor.Epoch.Raw, plan.Successor.Epoch.Inputs, plan.Successor.Epoch.Materials); err != nil {
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
		networkExpiry = awaitNativeExpiryDenial(t, actual, retained, original.FirstFailure)
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
	checkNativeIssuerRootsHeld(t, plan.Issuance)
	release()
	select {
	case <-original.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("original role did not join after releasing actual observation return")
	}
	result := original.Close()
	checkNativeIssuerRootsReleased(t, plan.Issuance)
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

func checkNativeIssuerRootsHeld(t *testing.T, p issuer.Plan) {
	t.Helper()
	if ledger, err := quota.Open(p.AdmissionRoot, p.AdmissionBinding); !errors.Is(err, quota.ErrBusy) {
		if err == nil {
			_ = ledger.Close()
		}
		t.Fatal("unjoined Issuer lacks actual occupied quota lease", err)
	}
	if keys, err := issuance.Open(t.Context(), p.KeyRoot, p.KeyBinding); !errors.Is(err, issuance.ErrBusy) {
		if err == nil {
			_ = keys.Close()
		}
		t.Fatal("unjoined Issuer lacks actual occupied key custody", err)
	}
	lock, err := os.OpenFile(filepath.Join(p.ResultRoot, "results.lock"), os.O_RDWR, 0600)
	if err != nil {
		t.Fatal("original result lease absent", err)
	}
	defer lock.Close()
	err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if !errors.Is(err, syscall.EWOULDBLOCK) {
		t.Fatal("unjoined Issuer lacks actual occupied result lease", err)
	}
}

func checkNativeIssuerRootsReleased(t *testing.T, p issuer.Plan) {
	t.Helper()
	ledger, err := quota.Open(p.AdmissionRoot, p.AdmissionBinding)
	if err != nil {
		t.Fatal("joined quota lease unavailable", err)
	}
	defer ledger.Close()
	keys, err := issuance.Open(t.Context(), p.KeyRoot, p.KeyBinding)
	if err != nil {
		t.Fatal("joined original key custody unavailable", err)
	}
	defer keys.Close()
	results, err := issuance.OpenResults(t.Context(), p.ResultRoot, keys, p.AdmissionBinding)
	if err != nil {
		t.Fatal("joined original result lease unavailable", err)
	}
	if err := results.Close(); err != nil {
		t.Fatal(err)
	}
}
