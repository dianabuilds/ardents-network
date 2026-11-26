//go:build linux && node_process_native

package node_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/enrollment"
	"github.com/dianabuilds/ardents-network/internal/network/duty"
	"github.com/dianabuilds/ardents-network/internal/network/epoch"
	"github.com/dianabuilds/ardents-network/internal/network/source"
	"github.com/dianabuilds/ardents-network/internal/network/source/servingdeclaration"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/platform/systemd"
	"github.com/dianabuilds/ardents-network/internal/processresource"
	"github.com/dianabuilds/ardents-network/internal/release"
	"github.com/dianabuilds/ardents-network/internal/release/infrastructure"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
	networkinputs "github.com/dianabuilds/ardents-network/tests/networkfixture"
	releasefixture "github.com/dianabuilds/ardents-network/tests/releasefixture"
	"github.com/theupdateframework/go-tuf/v2/metadata"

	"github.com/dianabuilds/ardents-network/tests/e2e/node/processfixture"
	sourcefixture "github.com/dianabuilds/ardents-network/tests/e2e/source"
)

// End-to-end retained Source lifecycle (#604 stage 1.2): prepare-state domain
// composition → explicit first pin → serve-retained → joined completion →
// relaunch → forward program change → crash refusal. Every serving phase runs
// the genuine pinned command under the exact native process envelope; State,
// participation, Release and transport owners are real. Preparation composes
// the same owners the offline prepare-state command uses (its fixed-account
// gate has separate network_signing_native coverage). The first pin runs the
// genuine enrolled executable through the development authenticate-initial
// command, because executable provenance binds /proc/self/exe; the platform
// initialize-trust/check fixed-path composition is installed evidence
// (stage 1.5).
//
// Honest limits: no installed unit, no loaded-manager platform check pre-start
// and no repair-retained manager qualification here — those are the separate
// native installed evidence gate (stage 1.5) before transitional Source
// orchestration retires. The forward artifact is distinct exact signed bytes;
// executing a replaced program binary belongs to that installed sequence. One
// fixture operator, not independent participants.

func launchRetainedSource(t *testing.T, binary, plan string) *processfixture.NativeProcess {
	t.Helper()
	unit := fmt.Sprintf("ardents-source-retained-%d-%d.service", os.Getpid(), time.Now().UnixNano())
	return processfixture.Launch(t, unit, "retained Source original diagnostic", "original retained Source stop unavailable",
		"--property=CPUQuota=100%", "--property=CPUQuotaPeriodSec=100ms", "--property=MemoryMax=1G", "--property=MemoryHigh=768M", "--property=TasksMax=256", "--property=LimitNOFILE=1024", "--property=RuntimeMaxSec=120s",
		"--setenv=GOMAXPROCS=1", "--setenv=GOMEMLIMIT=512MiB", "--", binary, "source", "serve-retained", plan)
}

type retainedSourceTerminal struct {
	Phase, Outcome string
	First          string `json:"first_owner"`
}

// joinRetainedSource optionally signals the original main process, then joins
// the actual command and decodes its terminal report. An empty signal waits for
// a self-refusing invocation. Killed processes may leave no report at all.
func joinRetainedSource(t *testing.T, p *processfixture.NativeProcess, signal string) (int, retainedSourceTerminal, []byte) {
	t.Helper()
	if signal != "" {
		if err := exec.Command("systemctl", "kill", "--kill-whom=main", "--signal="+signal, p.Unit).Run(); err != nil {
			t.Fatal("original retained Source signal", signal, err)
		}
	}
	// Wait closes the original StdoutPipe; drain the actual output to EOF
	// (process exit) before reaping, for both signaled and self-refusing runs.
	stdout, readErr := io.ReadAll(p.Stdout)
	err := p.Command.Wait()
	p.Joined = true
	if readErr != nil {
		t.Fatal("original retained Source stdout", readErr)
	}
	code := 0
	var exited *exec.ExitError
	if err != nil {
		if !errors.As(err, &exited) {
			t.Fatal("original retained Source wait", err)
		}
		code = exited.ExitCode()
	}
	var terminal retainedSourceTerminal
	if raw := bytes.TrimSpace([]byte(p.Diagnostic.String())); len(raw) > 0 {
		if json.Unmarshal(raw, &terminal) != nil {
			t.Fatal("original retained Source terminal report", string(raw))
		}
	}
	return code, terminal, stdout
}

func TestNativeSourceRetainedLifecycle(t *testing.T) {
	binary := processfixture.PinnedCommand(t)
	binaryBytes, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	spec, _, _ := networkinputs.PrepareNetworkGenesis(t, t.TempDir())
	bundle, err := networkfixture.BuildClosed(spec)
	if err != nil {
		t.Fatal(err)
	}
	clock, closeClock := networkinputs.StartObservedClock(t)
	defer closeClock()

	parent := t.TempDir()
	stateRoot := filepath.Join(parent, "state")
	participationRoot := filepath.Join(parent, "participation")
	for _, root := range []string{stateRoot, participationRoot} {
		if err := os.Mkdir(root, 0700); err != nil {
			t.Fatal(err)
		}
	}
	var key [32]byte
	copy(key[:], spec.Authority.Public().(ed25519.PublicKey))
	networkPlan := state.AuthorityDeclaration{Root: stateRoot, NetworkID: spec.NetworkID, Authorities: [][32]byte{key}, Threshold: 1, ProfileAuthority: key, ClockObservationFile: clock}
	config, err := networkPlan.Config()
	if err != nil {
		t.Fatal(err)
	}

	server, client, _, rootPEM, serverPin, clientPin := sourcefixture.NativeTLS(t)
	certPath, keyPath := processfixture.WriteNativeCredential(t, server)
	rootPath := filepath.Join(parent, "client-ca.pem")
	if err := os.WriteFile(rootPath, rootPEM, 0600); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	plan := servingdeclaration.ServingDeclaration{Network: (*servingdeclaration.ServingNetwork)(&networkPlan), ProcessProfile: processresource.SourceProfile,
		ParticipationRoot: participationRoot, Address: address, Certificate: certPath, PrivateKey: keyPath, ClientRoot: rootPath, ClientPins: [][32]byte{clientPin}}
	raw, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(parent, "source-plan.json")
	if err := os.WriteFile(planPath, raw, 0600); err != nil {
		t.Fatal(err)
	}
	authorities := map[[32]byte]ed25519.PublicKey{sha256.Sum256(key[:]): spec.Authority.Public().(ed25519.PublicKey)}
	clientConfig := source.Config{Sources: [2]source.Source{
		{Address: address, ServerName: "source-native.test", Identity: [32]byte{71}, Family: "source-one", EndpointHandle: "source-one", RootPEM: rootPEM, LeafKeyDigest: serverPin},
		{Address: "127.0.0.1:9", ServerName: "unused.test", Identity: [32]byte{72}, Family: "source-two", EndpointHandle: "source-two", RootPEM: rootPEM, LeafKeyDigest: [32]byte{73}},
	}, ClientCertificate: client, VerificationClock: time.Now}
	fetch := func(message source.Message) (source.Message, error) {
		t.Helper()
		c, _, err := source.New(clientConfig, authorities)
		if err != nil {
			return source.Message{}, err
		}
		ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
		defer cancel()
		return c.Fetch(ctx, 0, message)
	}
	healthyFetch := func(phase string) {
		t.Helper()
		for _, operation := range []string{"latest", "by-digest"} {
			request := source.Message{Operation: operation, NetworkDigest: source.NetworkDigest(networkPlan.NetworkID)}
			if operation == "by-digest" {
				request.ObjectDigest = bundle.Epoch.Digest
			}
			response, err := fetch(request)
			if err != nil || response.Status != "ok" || response.ObjectDigest != bundle.Epoch.Digest {
				t.Fatal(phase, "genuine authenticated exchange", operation, response.Status, err)
			}
			decoded, err := source.DecodeBundle(response.Payload)
			if err != nil || !bytes.Equal(decoded.Epoch, bundle.Epoch.Raw) || len(decoded.Materials) != 1 || !bytes.Equal(decoded.Materials[0], bundle.Epoch.Materials[0]) {
				t.Fatal(phase, "canonical signed byte oracle differs", err)
			}
			verified, err := epoch.Verify(epoch.Policy{NetworkID: networkPlan.NetworkID, Authorities: authorities, Threshold: 1, Profile: "ardents-route-v3", Now: time.Now()}, decoded.Epoch, decoded.Inputs, decoded.Materials, true)
			if err != nil || verified.Header.Digest != bundle.Epoch.Digest {
				t.Fatal(phase, "independent new Epoch verification", err)
			}
		}
	}
	waitListening := func(p *processfixture.NativeProcess, phase string) {
		t.Helper()
		var ready map[string]string
		if err := json.NewDecoder(p.Stdout).Decode(&ready); err != nil || ready["operation"] != "source.serve" || ready["phase"] != "listening" {
			t.Fatal(phase, "retained Source startup", ready, err)
		}
	}
	cleanStop := func(p *processfixture.NativeProcess, phase string) {
		t.Helper()
		code, terminal, _ := joinRetainedSource(t, p, "SIGTERM")
		if code != 130 || terminal.Phase != "joined" || terminal.Outcome != "canceled" {
			t.Fatal(phase, "original joined cancellation", code, terminal, p.Diagnostic.String())
		}
	}

	// Cold retained opening refuses missing history before any listening.
	p := launchRetainedSource(t, binary, planPath)
	code, terminal, stdout := joinRetainedSource(t, p, "")
	if code != 1 || terminal.Phase != "refused" || terminal.First != "state" || bytes.Contains(stdout, []byte("listening")) {
		t.Fatal("cold serve-retained adopted missing history", code, terminal, string(stdout))
	}

	// prepare-state domain composition: signed State acceptance through the
	// fresh owner plus empty participation initialization, once on vacant roots.
	owner, err := state.OpenFresh(config)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := owner.AcceptEpoch(t.Context(), bundle.Epoch.Raw, bundle.Epoch.Inputs, bundle.Epoch.Materials); err != nil {
		_ = owner.Close()
		t.Fatal(err)
	}
	snapshot, err := owner.Current()
	if err != nil {
		_ = owner.Close()
		t.Fatal(err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	if err := duty.Initialize(t.Context(), duty.Config{Root: participationRoot, Clock: time.Now}); err != nil {
		t.Fatal(err)
	}
	if repeated, err := state.OpenFresh(config); err == nil {
		_ = repeated.Close()
		t.Fatal("repeated preparation adopted a non-vacant root")
	}
	statePointer, err := os.ReadFile(filepath.Join(stateRoot, "current"))
	if err != nil {
		t.Fatal(err)
	}
	// The serving duty identity is the owner's own published serving exclusion.
	dutyIdentity, dutyFamily := snapshot.SourceNodeID, sha256.Sum256([]byte(snapshot.SourceFamily))
	producerPresent := func(want bool, phase string) {
		t.Helper()
		conflict, err := duty.ReadConflict(participationRoot, time.Now, dutyIdentity, dutyFamily)
		if err != nil || conflict != want {
			t.Fatal(phase, "serving producer presence", conflict, want, err)
		}
	}
	producerPresent(false, "prepared")

	// Explicit first pin: executable provenance requires the enrolled program
	// to be the actual running executable (/proc/self/exe), so the genuine
	// enrolled copy runs the development authenticate-initial command over
	// independent paths. The platform initialize-trust fixed-path composition
	// is installed evidence (stage 1.5); the runtime role pair binds below
	// through the retained candidate handoff.
	authority := releasefixture.NewReleaseAuthority(t, releasefixture.InfrastructureReference().Add(365*24*time.Hour))
	dirV1, pinV1, _ := releasefixture.InfrastructureDistribution(t, t.TempDir(), binaryBytes, authority, 1, nil, "source", []byte(systemd.PlatformUnitTemplate))
	releaseRoot := filepath.Join(parent, "release-trust")
	pinCommand := exec.CommandContext(t.Context(), filepath.Join(dirV1, infrastructure.ProgramName),
		"infrastructure", "authenticate-initial", dirV1, pinV1, releaseRoot, "h3-test", "new-release-test", releasefixture.InfrastructureReference().Format(time.RFC3339))
	if out, err := pinCommand.CombinedOutput(); err != nil || !bytes.Contains(out, []byte("authenticated-source-artifact")) || !bytes.Contains(out, []byte(`"program_outcome":"release-accepted"`)) {
		t.Fatal("explicit first pin", string(out), err)
	}

	// First retained launch: genuine producer Claim, listening and distribution,
	// then requested stop whose joined completion removes the producer.
	p = launchRetainedSource(t, binary, planPath)
	waitListening(p, "first launch")
	producerPresent(true, "first launch")
	healthyFetch("first launch")
	cleanStop(p, "first launch")
	producerPresent(false, "first completion")
	if reopened, err := state.Open(config); err != nil {
		t.Fatal("first completion did not release the State lease", err)
	} else if err := reopened.Close(); err != nil {
		t.Fatal(err)
	}

	// Relaunch is a new independently checked invocation over the same histories.
	p = launchRetainedSource(t, binary, planPath)
	waitListening(p, "relaunch")
	healthyFetch("relaunch")
	cleanStop(p, "relaunch")
	producerPresent(false, "relaunch completion")

	// Forward program change: strictly forward signed generation authenticates
	// against retained floors; rollback and mixed metadata refuse without
	// touching State or participation bytes.
	forwardBytes := append(bytes.Clone(binaryBytes), '\n')
	dirV2, _, _ := releasefixture.InfrastructureDistribution(t, t.TempDir(), forwardBytes, authority, 2, nil, "source", []byte(systemd.PlatformUnitTemplate))
	dirMixed, _, _ := releasefixture.InfrastructureDistribution(t, t.TempDir(), forwardBytes, authority, 3, func(targets map[string]*metadata.TargetFiles) {
		generation := targets["ardents/linux-amd64/infrastructure/source"]
		var fields map[string]any
		if err := json.Unmarshal(*generation.Custom, &fields); err != nil {
			t.Fatal(err)
		}
		fields["release_version"] = 2
		raw, err := json.Marshal(fields)
		if err != nil {
			t.Fatal(err)
		}
		custom := json.RawMessage(raw)
		generation.Custom = &custom
	}, "source", []byte(systemd.PlatformUnitTemplate))
	loadCandidate := func(dir string) (enrollment.InfrastructureCandidate, release.Inputs) {
		t.Helper()
		c, err := enrollment.ReadInfrastructureCandidate(t.Context(), dir)
		if err != nil {
			t.Fatal(err)
		}
		return c, releasefixture.InfrastructureCandidateInputs(t, c)
	}
	candidateV1, inputV1 := loadCandidate(dirV1)
	candidateV2, inputV2 := loadCandidate(dirV2)
	candidateMixed, inputMixed := loadCandidate(dirMixed)
	verifier, err := release.OpenRetained(releaseRoot)
	if err != nil {
		t.Fatal(err)
	}
	forward, err := infrastructure.AuthenticateRuntimeCandidate(t.Context(), verifier, candidateV2, inputV2)
	if err != nil {
		_ = verifier.Close()
		t.Fatal("forward program change", err)
	}
	if err := forward.Check(t.Context(), verifier); err != nil {
		_ = verifier.Close()
		t.Fatal("forward handoff", err)
	}
	floors, err := verifier.CurrentFloors(t.Context())
	if err != nil || floors.TargetsVersion != 2 {
		_ = verifier.Close()
		t.Fatal("forward floor", floors, err)
	}
	participationPointer, err := os.ReadFile(filepath.Join(participationRoot, "current"))
	if err != nil {
		_ = verifier.Close()
		t.Fatal(err)
	}
	for name, unsafe := range map[string]struct {
		candidate enrollment.InfrastructureCandidate
		input     release.Inputs
	}{"rollback": {candidateV1, inputV1}, "mixed": {candidateMixed, inputMixed}} {
		if _, err := infrastructure.AuthenticateRuntimeCandidate(t.Context(), verifier, unsafe.candidate, unsafe.input); err == nil {
			_ = verifier.Close()
			t.Fatal("unsafe program change accepted", name)
		}
	}
	if err := verifier.Close(); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(filepath.Join(stateRoot, "current"))
	if err != nil || !bytes.Equal(statePointer, after) {
		t.Fatal("program change rewrote State history", err)
	}
	after, err = os.ReadFile(filepath.Join(participationRoot, "current"))
	if err != nil || !bytes.Equal(participationPointer, after) {
		t.Fatal("Release refusal rewrote participation", err)
	}
	p = launchRetainedSource(t, binary, planPath)
	waitListening(p, "forward launch")
	healthyFetch("forward launch")
	cleanStop(p, "forward launch")
	producerPresent(false, "forward completion")

	// A killed invocation supplies no completion evidence: the producer stays
	// retained and the next retained opening refuses before listening.
	p = launchRetainedSource(t, binary, planPath)
	waitListening(p, "crash launch")
	producerPresent(true, "crash launch")
	code, terminal, _ = joinRetainedSource(t, p, "SIGKILL")
	if code == 0 || code == 130 {
		t.Fatal("killed Source reported joined completion", code, terminal)
	}
	producerPresent(true, "after crash")
	p = launchRetainedSource(t, binary, planPath)
	code, terminal, stdout = joinRetainedSource(t, p, "")
	if code != 1 || terminal.Phase != "refused" || terminal.First != "state" || bytes.Contains(stdout, []byte("listening")) {
		t.Fatal("retained producer adopted after crash", code, terminal, string(stdout))
	}
	after, err = os.ReadFile(filepath.Join(stateRoot, "current"))
	if err != nil || !bytes.Equal(statePointer, after) {
		t.Fatal("crash or refusal rewrote State history", err)
	}
}
