//go:build linux

package main

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network/duty"
	"github.com/dianabuilds/ardents-network/internal/successor/network/source"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

// Independent authorities remain test fixtures. State genuinely authenticates
// the signed material and owns its real durable root, Source and exposure.
func sourceStateCustodyFixture(t *testing.T) (string, sourceServePlan) {
	t.Helper()
	f := newNetworkAdmissionFixture(t)
	bundle, err := networkfixture.BuildClosed(f.spec)
	if err != nil {
		t.Fatal(err)
	}
	path, plan := sourceCustodyFixture(t)
	clock, stopClock := joinProcessClock(t)
	t.Cleanup(stopClock)
	var pin [32]byte
	copy(pin[:], f.spec.Authority.Public().(ed25519.PublicKey))
	plan.Network = &networkAuthorityPlan{Root: plan.Network.Root, NetworkID: f.spec.NetworkID, Authorities: [][32]byte{pin}, Threshold: 1, ProfileAuthority: pin, ClockObservationFile: clock}
	config, err := networkStateConfig(plan.Network)
	if err != nil {
		t.Fatal(err)
	}
	seed, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	_, accepted := seed.Accept(t.Context(), bundle.Epoch.Raw, bundle.Epoch.Inputs, bundle.Epoch.Materials)
	if err := errors.Join(accepted, seed.Close()); err != nil {
		t.Fatal(err)
	}
	server, _, _, root, _, clientPin := sourceNativeTLS(t)
	plan.ClientPins = [][32]byte{clientPin}
	der, err := x509.MarshalPKCS8PrivateKey(server.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(der)
	for name, raw := range map[string][]byte{plan.Certificate: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate[0]}), plan.PrivateKey: pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), plan.ClientRoot: root} {
		if err := os.WriteFile(name, raw, 0600); err != nil {
			t.Fatal(err)
		}
		clear(raw)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	plan.Address = listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	return path, plan
}

func TestSourceOriginalInputLossAfterStateRecoveryPrecedesExposureAndListener(t *testing.T) {
	for _, name := range []string{"plan", "certificate", "key", "ca"} {
		for _, fault := range []string{"replace", "mutate", "unlink", "access", "ancestor-loss"} {
			for stage, boundary := range map[string]int{"before-root": 1, "before-exposure": 2, "after-exposure": 3} {
				t.Run(name+"/"+fault+"/"+stage, func(t *testing.T) {
					path, plan := sourceStateCustodyFixture(t)
					selected := map[string]string{"plan": path, "certificate": plan.Certificate, "key": plan.PrivateKey, "ca": plan.ClientRoot}[name]
					inputs, _, err := acquireSourceInputs(t.Context(), path)
					if err != nil {
						t.Fatal(err)
					}
					defer inputs.Close()
					config, err := networkStateConfig(plan.Network)
					if err != nil {
						t.Fatal(err)
					}
					config.LocalRoleStateRoot = plan.ParticipationRoot
					if err := sourceServingInputs(&config, plan, inputs, inputs.Check); err != nil {
						t.Fatal(err)
					}
					calls := 0
					config.PermitWork = func() error {
						calls++
						if calls == boundary { // Exact pre-root, post-recovery or post-exposure observation.
							var err error
							switch fault {
							case "replace":
								err = os.Rename(selected, selected+".original")
								if err == nil {
									err = os.WriteFile(selected, []byte("replacement"), 0600)
								}
							case "mutate":
								err = os.WriteFile(selected, []byte("changed"), 0600)
							case "unlink":
								err = os.Remove(selected)
							case "access":
								err = os.Chmod(selected, 0400)
							case "ancestor-loss":
								err = os.Rename(filepath.Dir(selected), filepath.Dir(selected)+".original")
							}
							if err != nil {
								t.Fatal(err)
							}
						}
						return inputs.Check()
					}
					owner, err := state.Open(config)
					if owner != nil || !errors.Is(err, errSourceInput) {
						if owner != nil {
							_ = owner.Close()
						}
						t.Fatal("State startup accepted lost original input", err)
					}
					if _, err := os.Stat(plan.ParticipationRoot); boundary <= 2 && !errors.Is(err, os.ErrNotExist) {
						t.Fatal("pre-admission refusal created Source exposure", err)
					}
					sentinel, err := net.Listen("tcp", plan.Address)
					if err != nil {
						t.Fatal("pre-admission refusal left a listener", err)
					}
					if err := sentinel.Close(); err != nil {
						t.Fatal(err)
					}
					config.Source, config.PermitWork = source.Config{}, nil
					reopened, err := state.Open(config)
					if err != nil {
						t.Fatal("partial State startup did not join original root", err)
					}
					if boundary == 3 {
						snapshot, err := reopened.Current()
						if err != nil {
							t.Fatal(err)
						}
						conflict, err := duty.ReadConflict(plan.ParticipationRoot, time.Now, snapshot.SourceNodeID, sha256.Sum256([]byte(snapshot.SourceFamily)))
						if err != nil || conflict {
							t.Fatal("failed startup retained Source exposure after actual join", conflict, err)
						}
					}
					if err := reopened.Close(); err != nil {
						t.Fatal(err)
					}
				})
			}
		}
	}
}

type heldOriginalSourceState struct {
	sourceLifetime
	entered, release chan struct{}
}

type sourceStateCloseReceipt struct {
	sourceLifetime
	joined chan error
}

func (s sourceStateCloseReceipt) Close() error {
	err := s.sourceLifetime.Close()
	s.joined <- err
	return err
}

func TestSourceCancelInterruptsRealStateWhileOriginalCheckIsHeld(t *testing.T) {
	for _, unavailable := range []bool{false, true} {
		t.Run(map[bool]string{false: "joined", true: "completion-unavailable"}[unavailable], func(t *testing.T) {
			sourceHeldOriginalObservation(t, unavailable)
		})
	}
}

func sourceHeldOriginalObservation(t *testing.T, unavailable bool) {
	t.Helper()
	path, plan := sourceStateCustodyFixture(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var target atomic.Pointer[os.File]
	var armed, held atomic.Bool
	entered, release := make(chan struct{}), make(chan struct{})
	inputs, _, err := acquireObservedSourceInputs(ctx, path, func(phase string, f *os.File) {
		if phase == "check" && f == target.Load() && armed.Load() && held.CompareAndSwap(false, true) {
			close(entered)
			<-release
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	defer inputs.Close()
	config, err := networkStateConfig(plan.Network)
	if err != nil {
		t.Fatal(err)
	}
	config.LocalRoleStateRoot, config.PermitWork = plan.ParticipationRoot, inputs.Check
	if err := sourceServingInputs(&config, plan, inputs, inputs.Check); err != nil {
		t.Fatal(err)
	}
	owner, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	receipt := sourceStateCloseReceipt{owner, make(chan error, 1)}
	target.Store(inputs.files[2].file)
	armed.Store(true)
	_, deny := context.WithCancel(inputs.ctx)
	defer deny()
	joined := make(chan error, 1)
	notice := sourceNotice{make(chan struct{})}
	go func() { joined <- superviseSource(inputs.ctx, receipt, inputs.Check, deny, io.Discard, notice, nil) }()
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	<-entered
	cancel()
	select {
	case err := <-receipt.joined:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		close(release)
		t.Fatal("original cancellation waited for native Check before State interruption")
	}
	if unavailable {
		<-notice.observed
		select {
		case <-joined:
			t.Fatal("unavailable notice returned original custody before held use joined")
		default:
		}
	}
	for _, f := range inputs.files {
		if _, err := f.file.Stat(); err != nil {
			t.Fatal("composition released input before held original observation joined", err)
		}
	}
	close(release)
	err = <-joined
	if unavailable {
		var terminal *sourceTerminal
		if !errors.As(err, &terminal) || terminal.firstOwner != "completion" {
			t.Fatal("failed unavailable notice became success after actual use joined", err)
		}
	} else if err != nil {
		t.Fatal("ordinary joined cancellation became a new operational failure", err)
	}
}

func (s heldOriginalSourceState) Close() error {
	close(s.entered)
	<-s.release
	return s.sourceLifetime.Close()
}

func TestSourceOriginalCustodySurvivesHeldStateCloseAndLateInputFailure(t *testing.T) {
	path, plan := sourceStateCustodyFixture(t)
	inputs, _, err := acquireSourceInputs(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer inputs.Close()
	config, err := networkStateConfig(plan.Network)
	if err != nil {
		t.Fatal(err)
	}
	admission, deny := context.WithCancel(t.Context())
	defer deny()
	check := func() error {
		if err := inputs.Check(); err != nil {
			return err
		}
		return admission.Err()
	}
	config.LocalRoleStateRoot, config.PermitWork = plan.ParticipationRoot, check
	if err := sourceServingInputs(&config, plan, inputs, check); err != nil {
		t.Fatal(err)
	}
	owner, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	snapshot, err := owner.Current()
	if err != nil {
		t.Fatal(err)
	}
	held := heldOriginalSourceState{owner, make(chan struct{}), make(chan struct{})}
	defer func() {
		select {
		case <-held.release:
		default:
			close(held.release)
		}
	}()
	output := errors.New("original readiness output failure")
	joined := make(chan error, 1)
	go func() {
		joined <- superviseSource(t.Context(), held, check, deny, failedSourceOutput{output}, io.Discard, inputs.Close)
	}()
	<-held.entered
	for _, f := range inputs.files {
		if _, err := f.file.Stat(); err != nil {
			t.Fatal("original input released while State Close remains held", err)
		}
	}
	independent := config
	independent.Source, independent.PermitWork = source.Config{}, nil
	if replacement, err := state.Open(independent); err == nil {
		_ = replacement.Close()
		t.Fatal("original State root reopened before actual join")
	}
	conflict, err := duty.ReadConflict(plan.ParticipationRoot, time.Now, snapshot.SourceNodeID, sha256.Sum256([]byte(snapshot.SourceFamily)))
	if err != nil || !conflict {
		t.Fatal("original Source guard disappeared before actual join", err)
	}
	// A real independently failed original descriptor arrives after output
	// failure and synchronous denial, while State still retains its borrowers.
	if err := inputs.files[2].file.Close(); err != nil {
		t.Fatal(err)
	}
	close(held.release)
	if err := <-joined; !errors.Is(err, output) || !errors.Is(err, os.ErrClosed) {
		t.Fatal("late input observation erased original output failure", err)
	}
	first := inputs.Close()
	if !errors.Is(first, os.ErrClosed) || inputs.Close() != first {
		t.Fatal("independent input Close failed outcome changed", first)
	}
	reopened, err := state.Open(independent)
	if err != nil {
		t.Fatal("original State root not released after actual join", err)
	}
	if err := reopened.Close(); err != nil {
		t.Fatal(err)
	}
	conflict, err = duty.ReadConflict(plan.ParticipationRoot, time.Now, snapshot.SourceNodeID, sha256.Sum256([]byte(snapshot.SourceFamily)))
	if err != nil || conflict {
		t.Fatal("original Source guard not released after actual join", err)
	}
}

func TestSourceUnavailableInputCloseRetainsOriginalFailureThroughActualJoin(t *testing.T) {
	path, plan := sourceStateCustodyFixture(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var selected atomic.Pointer[os.File]
	var held atomic.Bool
	entered, release := make(chan struct{}), make(chan struct{})
	inputs, _, err := acquireObservedSourceInputs(ctx, path, func(phase string, f *os.File) {
		if phase == "close" && f == selected.Load() && held.CompareAndSwap(false, true) {
			close(entered)
			<-release
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	defer inputs.Close()
	config, err := networkStateConfig(plan.Network)
	if err != nil {
		t.Fatal(err)
	}
	config.LocalRoleStateRoot, config.PermitWork = plan.ParticipationRoot, inputs.Check
	if err := sourceServingInputs(&config, plan, inputs, inputs.Check); err != nil {
		t.Fatal(err)
	}
	owner, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	selected.Store(inputs.files[2].file)
	ready := make(chan struct{})
	output := sourceFailureWriter(func(raw []byte) (int, error) { close(ready); return len(raw), nil })
	notice := sourceNotice{make(chan struct{})}
	joined := make(chan error, 1)
	_, deny := context.WithCancel(inputs.ctx)
	defer deny()
	go func() { joined <- superviseSource(inputs.ctx, owner, inputs.Check, deny, output, notice, inputs.Close) }()
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	<-ready
	cancel()
	<-entered
	<-notice.observed
	if _, err := selected.Load().Stat(); err != nil {
		t.Fatal("original key descriptor released before held input Close joined", err)
	}
	select {
	case <-joined:
		t.Fatal("input Close notice substituted for actual original join")
	default:
	}
	if err := selected.Load().Close(); err != nil {
		t.Fatal(err)
	}
	close(release)
	err = <-joined
	var terminal *sourceTerminal
	if !errors.Is(err, os.ErrClosed) || !errors.As(err, &terminal) || terminal.firstOwner != "completion" {
		t.Fatal("late independent input Close erased failed unavailable notice", err)
	}
	first := inputs.Close()
	if !errors.Is(first, os.ErrClosed) || inputs.Close() != first {
		t.Fatal("copied/repeated physical input Close outcomes disagree", first)
	}
}

// This test proves actual State/serving/guard cleanup on failed readiness
// output. It supplies no successful native process measurement or admission.
func TestSourceFailedReadyOutputJoinsRealStateAndServingGuards(t *testing.T) {
	f := newNetworkAdmissionFixture(t)
	bundle, err := networkfixture.BuildClosed(f.spec)
	if err != nil {
		t.Fatal(err)
	}
	clock, stopClock := joinProcessClock(t)
	defer stopClock()
	var key [32]byte
	copy(key[:], f.spec.Authority.Public().(ed25519.PublicKey))
	plan := networkAuthorityPlan{Root: filepath.Join(t.TempDir(), "state"), NetworkID: f.spec.NetworkID, Authorities: [][32]byte{key}, Threshold: 1, ProfileAuthority: key, ClockObservationFile: clock}
	config, err := networkStateConfig(&plan)
	if err != nil {
		t.Fatal(err)
	}
	seed, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	_, acceptErr := seed.Accept(t.Context(), bundle.Epoch.Raw, bundle.Epoch.Inputs, bundle.Epoch.Materials)
	if err := errors.Join(acceptErr, seed.Close()); err != nil {
		t.Fatal(err)
	}
	server, _, _, root, _, pin := sourceNativeTLS(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	config.LocalRoleStateRoot = filepath.Join(t.TempDir(), "participation")
	admissionCtx, deny := context.WithCancel(t.Context())
	defer deny()
	config.PermitWork = admissionCtx.Err
	config.Source = source.Config{ServeAddress: address, ServeCertificate: server, ServeClientRootPEM: root, ServeClientKeyDigests: [][32]byte{pin}}
	owner, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := owner.Close(); err != nil {
			t.Error(err)
		}
	})
	snapshot, err := owner.Current()
	if err != nil {
		t.Fatal(err)
	}
	family := sha256.Sum256([]byte(snapshot.SourceFamily))
	assertConflict := func(want bool) {
		t.Helper()
		got, err := duty.ReadConflict(config.LocalRoleStateRoot, time.Now, snapshot.SourceNodeID, family)
		if err != nil || got != want {
			t.Fatal("original Source conflict differs", got, want, err)
		}
	}
	assertConflict(true)
	outputFailure := errors.New("failed actual command readiness output")
	if err := superviseSource(t.Context(), owner, admissionCtx.Err, deny, failedSourceOutput{outputFailure}, io.Discard, nil); !errors.Is(err, outputFailure) {
		t.Fatal("output failure lost", err)
	}
	assertConflict(false)
	config.Source = source.Config{}
	config.PermitWork = nil
	reopened, err := state.Open(config)
	if err != nil {
		t.Fatal("original State root not physically released", err)
	}
	if err := reopened.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestSourceCanceledCallerRefusesBeforePlanAccess(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if code := run(ctx, []string{"source", "serve", filepath.Join(t.TempDir(), "absent-plan")}, io.Discard, io.Discard); code != 2 {
		t.Fatal(code)
	}
}
