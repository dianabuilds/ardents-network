//go:build linux

package node

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
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
	snapshot, err := currentFacts(resolved)
	if err != nil {
		return nil, err
	}
	view, ok := config.CurrentClosedRoute()
	if !ok {
		return nil, errors.New("missing role State fixture")
	}
	input := introductionHeapInput{snapshot, view, config.ClosedIntroduction.Certificate.Certificate, config.IdentityKey, config.ClosedIntroduction.AdmissionRoot, config.LocalRoleStateRoot, output}
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
	for _, carrier := range []route.CarrierProfile{route.ClosedCarrierTCP, route.ClosedCarrierQUIC} {
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
			fixture := newPrivateRecipientNetworkFixtureWithStart(t, carrier, route.ClosedPurposeIntroduction, 3, func(config Config) (func() error, error) {
				var err error
				process, err = startIntroductionHeapProcess(t, config, output)
				if err != nil {
					return nil, err
				}
				return process.stop, nil
			}, 1)
			process.dump("startup")
			request := route.ClosedRegistrationRequest{Revision: 1, Expiry: time.Now().UTC().Add(40 * time.Second).Truncate(time.Second)}
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
			frame, err := route.ReadClosedLaneFrame(registration)
			if err != nil {
				t.Fatal(err)
			}
			if frame.Kind != 10 || frame.Lane == 0 {
				t.Fatal("missing actual pending capsule")
			}
			process.dump("pending")
			nonce, _, err := route.DecodeClosedIntroductionSubmission(frame.Body)
			if err != nil {
				t.Fatal(err)
			}
			body, err := route.EncodeClosedDescriptorResult(nonce, 0, nil)
			if err != nil {
				t.Fatal(err)
			}
			if err := route.WriteClosedLaneFrame(registration, route.ClosedLaneFrame{Kind: 11, Lane: frame.Lane, Body: body}); err != nil {
				t.Fatal(err)
			}
			closed, err := route.ReadClosedLaneFrame(registration)
			if err != nil || closed.Kind != 9 || closed.Lane != frame.Lane {
				t.Fatalf("delivery close: %v", err)
			}
			response, err := route.ReadClosedLaneFrame(submission)
			if err != nil || response.Kind != 11 || response.Lane != 0 {
				t.Fatalf("submission response: %v", err)
			}
			verdict, proof, err := route.DecodeClosedDescriptorResult(response.Body, sourceNonce)
			if err != nil || verdict != 0 || len(proof) != 0 {
				t.Fatalf("submission result: %v", err)
			}
			closeSubmission()
			withdrawal := route.ClosedRegistrationRequest{Slot: request.Slot, Revision: 1, Withdraw: true, Nonce: [32]byte{99}}
			if sendRegistrationFixture(t, registration, withdrawal) != 0 {
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
