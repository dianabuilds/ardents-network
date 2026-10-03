//go:build linux

package node

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	introductioncapsule "github.com/dianabuilds/ardents-network/internal/route/capsule"
	routecarrier "github.com/dianabuilds/ardents-network/internal/route/carrier"
	"github.com/dianabuilds/ardents-network/internal/route/terminal"
)

type introductionHeapProcess struct {
	dump   func(string)
	stop   func() error
	output string
}

func startIntroductionHeapProcess(t *testing.T, config Config, output string) (*introductionHeapProcess, error) {
	resolved, err := resolveConfig(config)
	if err != nil {
		return nil, err
	}
	snapshot, err := currentFacts(resolved.Current)
	if err != nil {
		return nil, err
	}
	view, err := config.CurrentClosedRoute()
	if err != nil {
		return nil, errors.New("missing role State fixture")
	}
	input := introductionHeapInput{snapshot, view, config.ClosedIntroduction.Certificate.Certificate, config.IdentityKey, config.HostingRoot, config.ClosedIntroduction.AdmissionRoot, config.LocalRoleStateRoot, output}
	raw, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	inputPath := filepath.Join(t.TempDir(), "receiver.json")
	if err := os.WriteFile(inputPath, raw, 0600); err != nil {
		return nil, err
	}
	clear(raw)
	binary, err := os.Executable()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	command := exec.CommandContext(ctx, binary, "-test.run=^TestClosedIntroductionProcessHeapObservation$", "-test.timeout=55s")
	// Do not copy arbitrary parent credentials or observer canaries into the role.
	command.Env = []string{"PATH=" + os.Getenv("PATH"), "ARDENTS_ROLE_HEAP_CHILD=" + inputPath}
	stdin, err := command.StdinPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		cancel()
		return nil, err
	}
	joined := false
	t.Cleanup(func() {
		if !joined {
			_ = command.Process.Kill()
			command.Wait()
		}
		cancel()
	})
	reader := bufio.NewReader(stdout)
	expect := func(want string) {
		t.Helper()
		line, err := reader.ReadString('\n')
		if err != nil || strings.TrimSpace(line) != want {
			t.Fatalf("role controller expected %q: %q / %v", want, line, err)
		}
	}
	expect("role-ready")
	process := &introductionHeapProcess{output: output}
	process.dump = func(phase string) {
		t.Helper()
		if _, err := fmt.Fprintln(stdin, phase); err != nil {
			t.Fatal(err)
		}
		expect("role-dump " + phase)
	}
	process.stop = func() error {
		if joined {
			return nil
		}
		if _, err := fmt.Fprintln(stdin, "stop"); err != nil {
			return err
		}
		_ = stdin.Close()
		// Drain test harness output before Wait closes StdoutPipe.
		_, readErr := io.Copy(io.Discard, reader)
		result := command.Wait()
		joined = true
		cancel()
		if result != nil {
			return fmt.Errorf("role child: %w; %s", result, stderr.String())
		}
		return readErr
	}
	return process, nil
}

// Heap capture observes an isolated receiving process, not all role fixtures
// co-located in a test runner. Captures pause the role and cannot qualify timing.
// Raw heap production is not a complete field-join analysis or a full P3 pass.
func TestClosedIntroductionProcessHeapObservation(t *testing.T) {
	if path := os.Getenv("ARDENTS_ROLE_HEAP_CHILD"); path != "" {
		runIntroductionHeapChild(t, path)
		return
	}
	for _, carrier := range []routecarrier.CarrierProfile{routecarrier.ClosedCarrierTCP, routecarrier.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			output := t.TempDir()
			if root := os.Getenv("ARDENTS_ROLE_HEAP_OBSERVATIONS"); root != "" {
				if !filepath.IsAbs(root) {
					t.Fatal("heap output must be absolute")
				}
				var err error
				output, err = os.MkdirTemp(root, string(carrier)+"-")
				if err != nil {
					t.Fatal(err)
				}
			}
			var process *introductionHeapProcess
			fixture := newPrivateRecipientNetworkFixtureWithStart(t, carrier, ardp.PurposeIntroduction, 3, func(config Config) (func() error, error) {
				var err error
				process, err = startIntroductionHeapProcess(t, config, output)
				if err != nil {
					return nil, err
				}
				return process.stop, nil
			}, 1)
			process.dump("startup")
			request := terminal.RegistrationRequest{Revision: 1, Expiry: time.Now().UTC().Add(40 * time.Second).Truncate(time.Second)}
			if _, err := rand.Read(request.Slot[:]); err != nil {
				t.Fatal(err)
			}
			if _, err := rand.Read(request.Nonce[:]); err != nil {
				t.Fatal(err)
			}
			registration, closeRegistration, status := registerIntroductionFixture(t, fixture, 0, request)
			defer closeRegistration()
			if status != 0 {
				t.Fatal("heap role registration refused")
			}
			process.dump("registered")
			submission, closeSubmission, sourceNonce, privateFields, ciphertext := startHeapSubmission(t, fixture, request)
			defer closeSubmission()
			frame, err := ardp.ReadFrame(registration)
			if err != nil {
				t.Fatal(err)
			}
			if frame.Kind != 10 || frame.Lane == 0 {
				t.Fatal("missing actual pending capsule")
			}
			process.dump("pending")
			nonce, _, err := introductioncapsule.DecodeSubmission(frame.Body)
			if err != nil {
				t.Fatal(err)
			}
			body, err := terminal.EncodeDescriptorResult(nonce, 0, nil)
			if err != nil {
				t.Fatal(err)
			}
			if err := ardp.WriteFrame(registration, ardp.Frame{Kind: 11, Lane: frame.Lane, Body: body}); err != nil {
				t.Fatal(err)
			}
			closed, err := ardp.ReadFrame(registration)
			if err != nil || closed.Kind != 9 || closed.Lane != frame.Lane {
				t.Fatalf("delivery close: %v", err)
			}
			response, err := ardp.ReadFrame(submission)
			if err != nil || response.Kind != 11 || response.Lane != 0 {
				t.Fatalf("submission response: %v", err)
			}
			verdict, proof, err := terminal.DecodeDescriptorResult(response.Body, sourceNonce)
			if err != nil || verdict != 0 || len(proof) != 0 {
				t.Fatalf("submission result: %v", err)
			}
			finishRecipientTerminal(t, fixture, submission, 0)
			closeSubmission()
			withdrawal := terminal.RegistrationRequest{Slot: request.Slot, Revision: 1, Withdraw: true, Nonce: [32]byte{99}}
			if sendRegistrationFixture(t, fixture, registration, withdrawal) != 0 {
				t.Fatal("heap role withdrawal refused")
			}
			closeRegistration()
			process.dump("withdrawn")
			if err := process.stop(); err != nil {
				t.Fatal(err)
			}
			for _, phase := range []string{"startup", "registered", "pending", "withdrawn"} {
				raw, err := os.ReadFile(filepath.Join(output, phase+".heap"))
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.HasPrefix(raw, []byte("go1.7 heap dump\n")) {
					t.Fatal("unexpected heap dump format")
				}
				for _, private := range privateFields {
					if bytes.Contains(raw, private[:]) {
						t.Fatalf("recipient-only plaintext appeared in receiving heap at %s", phase)
					}
				}
				if phase == "pending" && !bytes.Contains(raw, ciphertext) {
					t.Fatal("pending heap missed received encrypted capsule positive control")
				}
				contains := bytes.Contains(raw, request.Slot[:])
				if phase == "startup" && contains || phase != "startup" && !contains {
					t.Fatalf("heap phase %s failed receiving slot positive control", phase)
				}
			}
			t.Log("isolated real Node process heaps captured at startup/registration/pending/withdrawal; pause-influenced, incomplete P3 analysis")
		})
	}
}

type introductionHeapInput struct {
	Snapshot                        state.NodeDuty
	View                            state.ClosedRouteView
	Certificates                    [][]byte
	PrivateKey                      ed25519.PrivateKey
	HostingRoot                     string
	AdmissionRoot, RoleRoot, Output string
}

// The child receives only its own role key and public fixture State, not the
// parent's issuance authority, client keys, pending blind state or canaries.
// Run is the production Node lifecycle; public State acceptance is a fixture.
func runIntroductionHeapChild(t *testing.T, path string) {
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var input introductionHeapInput
	if err := json.Unmarshal(raw, &input); err != nil {
		t.Fatal(err)
	}
	clear(raw)
	events := make(chan Event, 32)
	config := Config{HostingRoot: input.HostingRoot, NetworkID: input.Snapshot.NetworkID, NodeID: input.Snapshot.NodeID, IdentityKey: input.PrivateKey,
		Current:              func() (state.NodeDuty, error) { return input.Snapshot, nil },
		CurrentClosedProfile: func() (state.ClosedProfileView, bool) { return input.View.Profile, true },
		CurrentClosedRoute:   func() (state.ClosedRouteView, error) { return input.View, nil },
		ClosedIntroduction:   ClosedIntroductionProfile{AdmissionRoot: input.AdmissionRoot, Certificate: tls.Certificate{Certificate: input.Certificates, PrivateKey: input.PrivateKey}, ConnectionLimit: 8, DrainTimeout: time.Second},
		PollInterval:         10 * time.Millisecond, Quarantine: time.Millisecond, LocalRoleStateRoot: input.RoleRoot, CheckPlacement: func() error { return nil },
		Emit: func(_ context.Context, event Event) error { events <- event; return nil }}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := Run(ctx, config); done <- err }()
	waitForStateEvent(t, events, "READY")
	eventsDone := make(chan struct{})
	eventsStop := make(chan struct{})
	joined := false
	go func() {
		defer close(eventsDone)
		for {
			select {
			case <-events:
			case <-eventsStop:
				return
			}
		}
	}()
	defer func() {
		cancel()
		if !joined {
			select {
			case err := <-done:
				if err != nil {
					t.Error(err)
				}
			case <-time.After(testLifecycleWait):
				t.Error("heap role failed to join after controller exit")
			}
		}
		close(eventsStop)
		<-eventsDone
	}()
	fmt.Println("role-ready")
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		phase := scanner.Text()
		if phase == "stop" {
			cancel()
			select {
			case err := <-done:
				joined = true
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(testLifecycleWait):
				t.Fatal("heap role failed to join")
			}
			return
		}
		switch phase {
		case "startup", "registered", "pending", "withdrawn":
		default:
			t.Fatal("unknown heap phase")
		}
		path := filepath.Join(input.Output, phase+".heap")
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			t.Fatal(err)
		}
		// A regular file is required: WriteHeapDump stops every goroutine. Never
		// send the dump to a pipe serviced by this same process.
		debug.WriteHeapDump(file.Fd())
		if err := errors.Join(file.Sync(), file.Close()); err != nil {
			t.Fatal(err)
		}
		fmt.Println("role-dump " + phase)
	}
	t.Fatal("heap controller ended before joined stop")
}

// The parent alone owns recipient key and plaintext canaries. Real HPKE seals
// them before the actual admitted Submission enters the receiving process.
// Publication/Endpoint job ownership is a fixture, not a full Service journey.
func startHeapSubmission(t *testing.T, fixture *resolutionNetworkFixture, request terminal.RegistrationRequest) (net.Conn, func(), [32]byte, [2][32]byte, []byte) {
	t.Helper()
	submitter := *fixture
	submitter.receiver.ExpectedPurpose = ardp.PurposeSubmission
	connection, closeCarrier, err := submitter.openTerminal(t.Context(), fixture.supplementary[1][0], 1)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(closeCarrier)
	end := time.Now().UTC().Add(9 * time.Second).Truncate(time.Second)
	if err := connection.SetDeadline(end); err != nil {
		t.Fatal(err)
	}
	var private [2][32]byte
	var nonce, delivery [32]byte
	for _, value := range [][]byte{private[0][:], private[1][:], nonce[:], delivery[:]} {
		if _, err := rand.Read(value); err != nil {
			t.Fatal(err)
		}
	}
	key, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	plaintext := introductioncapsule.Plaintext{Network: fixture.profile.NetworkID, ProfileDigest: fixture.profile.Digest,
		Target: private[0], JoinSecret: private[1], PublicationDigest: [32]byte{201}, Revision: request.Revision,
		RendezvousNode: [32]byte{202}, RendezvousDutyGeneration: 1, HandshakeContext: [32]byte{203}, ConnectionNonce: [32]byte{204},
		AttachmentGeneration: 1, Deadline: end, InitiatorBinding: [32]byte{205}, WorkSafetyNotAfter: end.Unix(), WorkSafetyMaximum: end.Unix(), NoNewRecoveryAfter: end.Unix()}
	envelope := introductioncapsule.Capsule{Slot: request.Slot, Revision: request.Revision, Expiry: end, DeliveryNonce: delivery}
	sealed, _, err := introductioncapsule.Seal(envelope, [32]byte(key.PublicKey().Bytes()), plaintext)
	if err != nil {
		t.Fatal(err)
	}
	operation, err := introductioncapsule.EncodeSubmission(nonce, sealed)
	if err != nil {
		t.Fatal(err)
	}
	if err := ardp.WriteFrame(connection, ardp.Frame{Kind: 10, Body: operation}); err != nil {
		t.Fatal(err)
	}
	return connection, closeCarrier, nonce, private, sealed.Ciphertext
}
