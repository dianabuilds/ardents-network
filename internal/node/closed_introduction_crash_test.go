//go:build linux

package node

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route"
)

// Only fixture credentials enter a parent-owned private temporary directory.
// The live recipient stays in the parent while SIGKILL removes the registration
// client's sockets without its TLS/Carrier/registration cleanup running.
// This exercises receiving admission and delivery, not Publisher authentication
// or installed Endpoint worker containment.
type introductionCrashClient struct {
	Profile      state.ClosedProfileView
	Carrier      route.CarrierProfile
	Endpoint     string
	Certificates [][]byte
	PrivateKey   ed25519.PrivateKey
	Receiver     route.ClosedRoleReceiver
	ServerKey    [32]byte
	Token        []byte
	Request      route.ClosedRegistrationRequest
}

func TestClosedIntroductionClientCrashStopsDelivery(t *testing.T) {
	if path := os.Getenv("ARDENTS_INTRODUCTION_CRASH_CLIENT"); path != "" {
		runIntroductionCrashClient(t, path)
		return
	}
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, carrier := range []route.CarrierProfile{route.ClosedCarrierTCP, route.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			fixture := newPrivateRecipientNetworkFixture(t, carrier, route.ClosedPurposeIntroduction, 3, 1)
			request := route.ClosedRegistrationRequest{Nonce: [32]byte{121}, Slot: [32]byte{122}, Revision: 1, Expiry: time.Now().UTC().Add(60 * time.Second).Truncate(time.Second)}
			input := introductionCrashClient{Profile: fixture.profile, Carrier: carrier, Endpoint: fixture.endpoint, Certificates: fixture.certificate.Certificate, PrivateKey: fixture.certificate.PrivateKey.(ed25519.PrivateKey), Receiver: fixture.receiver, ServerKey: fixture.serverKey, Token: fixture.tokens[0], Request: request}
			root := t.TempDir()
			if err := os.Chmod(root, 0700); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, "client.json")
			raw, err := json.Marshal(input)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, raw, 0600); err != nil {
				t.Fatal(err)
			}
			clear(raw)
			ctx, cancel := context.WithTimeout(t.Context(), 40*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, binary, "-test.run=^TestClosedIntroductionClientCrashStopsDelivery$", "-test.timeout=45s")
			command.Env = append(os.Environ(), "ARDENTS_INTRODUCTION_CRASH_CLIENT="+path)
			var output bytes.Buffer
			command.Stdout, command.Stderr = &output, &output
			if err := command.Start(); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() { done <- command.Wait() }()
			joined := false
			defer func() {
				if !joined {
					_ = command.Process.Kill()
					<-done
				}
			}()
			tick := time.NewTicker(10 * time.Millisecond)
			defer tick.Stop()
			for {
				if _, err := os.Stat(path + ".ready"); err == nil {
					break
				} else if !errors.Is(err, os.ErrNotExist) {
					t.Fatal(err)
				}
				select {
				case err := <-done:
					joined = true
					t.Fatalf("registration child ended: %v / %s", err, output.Bytes())
				case <-ctx.Done():
					t.Fatal("registration child deadline")
				case <-tick.C:
				}
			}
			if status, err := submitIntroductionCrashFixture(t, fixture, request, 0); err != nil || status != 0 {
				t.Fatalf("live delivery failed: %d / %v", status, err)
			}
			if err := command.Process.Kill(); err != nil {
				t.Fatal(err)
			}
			killed := <-done
			joined = true
			var exit *exec.ExitError
			if !errors.As(killed, &exit) {
				t.Fatalf("child not killed: %v", killed)
			}
			status, ok := exit.Sys().(syscall.WaitStatus)
			if !ok || !status.Signaled() || status.Signal() != syscall.SIGKILL {
				t.Fatalf("not SIGKILL: %v / %s", killed, output.Bytes())
			}
			// Admission must succeed before observing delivery failure. A QUIC peer
			// killed without CONNECTION_CLOSE may be detected only by bounded expiry;
			// an expired operation cannot be counted as successful delivery.
			if status, err := submitIntroductionCrashFixture(t, fixture, request, 1); err == nil && status == 0 {
				t.Fatal("dead registration acknowledged delivery")
			}
			request.Nonce[0]++
			_, closeOld, reclaim := registerIntroductionFixture(t, fixture, 1, request)
			closeOld()
			if reclaim != 1 {
				t.Fatal("dead owner's slot reclaimed with fresh admission")
			}
			fixture.restart()
			request.Nonce[0]++
			_, closeRestart, reclaim := registerIntroductionFixture(t, fixture, 2, request)
			closeRestart()
			if reclaim != 1 {
				t.Fatal("restart erased dead owner's slot floor")
			}
			request.Slot[0]++
			request.Nonce[0]++
			_, closeFresh, fresh := registerIntroductionFixture(t, fixture, 3, request)
			closeFresh()
			if fresh != 0 {
				t.Fatal("live recipient refused a fresh slot after crash")
			}
		})
	}
}

func runIntroductionCrashClient(t *testing.T, path string) {
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var input introductionCrashClient
	if err := json.Unmarshal(raw, &input); err != nil {
		t.Fatal(err)
	}
	clear(raw)
	fixture := &resolutionNetworkFixture{profile: input.Profile, carrier: input.Carrier, endpoint: input.Endpoint, certificate: tls.Certificate{Certificate: input.Certificates, PrivateKey: input.PrivateKey}, receiver: input.Receiver, serverKey: input.ServerKey, tokens: [][]byte{input.Token}}
	connection, closeCarrier, status := registerIntroductionFixture(t, fixture, 0, input.Request)
	defer closeCarrier()
	if status != 0 {
		t.Fatal("child registration refused")
	}
	if err := os.WriteFile(path+".ready", []byte("registered"), 0600); err != nil {
		t.Fatal(err)
	}
	var pending uint32
	for {
		frame, err := route.ReadClosedLaneFrame(connection)
		if err != nil {
			t.Fatal(err)
		}
		if frame.Kind == 9 {
			if pending == 0 || frame.Lane != pending || len(frame.Body) != 1 || frame.Body[0] != 0 {
				t.Fatal("unexpected delivery close")
			}
			pending = 0
			continue
		}
		if frame.Kind != 10 || frame.Lane == 0 || frame.Lane%2 != 0 || pending != 0 {
			t.Fatal("unexpected delivery frame")
		}
		nonce, capsule, err := route.DecodeClosedIntroductionSubmission(frame.Body)
		if err != nil || capsule.Slot != input.Request.Slot {
			t.Fatal("unexpected delivery capsule")
		}
		pending = frame.Lane
		response, err := route.EncodeClosedDescriptorResult(nonce, 0, nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := route.WriteClosedLaneFrame(connection, route.ClosedLaneFrame{Kind: 11, Lane: frame.Lane, Body: response}); err != nil {
			t.Fatal(err)
		}
	}
}

func submitIntroductionCrashFixture(t *testing.T, fixture *resolutionNetworkFixture, request route.ClosedRegistrationRequest, index int) (uint8, error) {
	t.Helper()
	submitter := *fixture
	submitter.receiver.ExpectedPurpose = route.ClosedPurposeSubmission
	ctx, cancel := context.WithTimeout(t.Context(), 12*time.Second)
	defer cancel()
	connection, closeCarrier, err := submitter.openTerminal(ctx, fixture.supplementary[1][index], 1)
	if err != nil {
		t.Fatalf("independent submission admission failed: %v", err)
	}
	defer closeCarrier()
	end := time.Now().UTC().Add(5 * time.Second).Truncate(time.Second)
	if err := connection.SetDeadline(end.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	nonce := [32]byte{byte(130 + index)}
	capsule := route.ClosedIntroductionCapsule{Slot: request.Slot, Revision: request.Revision, Expiry: end, DeliveryNonce: [32]byte{byte(140 + index)}, Encapsulation: [32]byte{131}, Ciphertext: make([]byte, 360)}
	raw, err := route.EncodeClosedIntroductionSubmission(nonce, capsule)
	if err != nil {
		t.Fatal(err)
	}
	if err := route.WriteClosedLaneFrame(connection, route.ClosedLaneFrame{Kind: 10, Body: raw}); err != nil {
		t.Fatal(err)
	}
	frame, err := route.ReadClosedLaneFrame(connection)
	if err != nil {
		return 1, err
	}
	if frame.Kind != 11 || frame.Lane != 0 {
		t.Fatal("unexpected submission result")
	}
	result, proof, err := route.DecodeClosedDescriptorResult(frame.Body, nonce)
	if err != nil || len(proof) != 0 {
		t.Fatalf("malformed submission result: %v", err)
	}
	return result, nil
}
