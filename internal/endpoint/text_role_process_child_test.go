//go:build linux

package endpoint

import (
	"bufio"
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/node"
)

type textRoleProcessInput struct {
	Snapshot                                     state.Snapshot
	View                                         state.ClosedRouteView
	Key                                          ed25519.PrivateKey
	Certificates                                 [][]byte
	Role, Root, AdmissionRoot, StateRoot, Output string
	Limit                                        uint16
	Drain                                        time.Duration
}

func textRoleProcessConfig(input textRoleProcessInput) node.Config {
	config := node.Config{NetworkID: input.Snapshot.NetworkID, NodeID: input.Snapshot.NodeID, IdentityKey: input.Key,
		Current:              func() (node.DutyView, error) { return textNetworkDutyFixture{snapshot: input.Snapshot}, nil },
		CurrentClosedProfile: func() (state.ClosedProfileView, bool) { return input.View.Profile, true },
		CurrentClosedRoute:   func() (state.ClosedRouteView, bool) { return input.View, true },
		LocalRoleStateRoot:   input.StateRoot, PollInterval: 20 * time.Millisecond, CheckPlacement: func() error { return nil }}
	certificate := tls.Certificate{Certificate: input.Certificates, PrivateKey: input.Key}
	switch input.Role {
	case "issuer":
		config.ClosedIssuer = node.ClosedIssuerProfile{Root: input.Root, AdmissionRoot: input.AdmissionRoot, Certificate: certificate, ConnectionLimit: input.Limit, DrainTimeout: input.Drain}
	case "resolution":
		config.ClosedResolution = node.ClosedResolutionProfile{Root: input.Root, AdmissionRoot: input.AdmissionRoot, Certificate: certificate, ConnectionLimit: input.Limit, DrainTimeout: input.Drain}
	case "introduction":
		config.ClosedIntroduction = node.ClosedIntroductionProfile{AdmissionRoot: input.AdmissionRoot, Certificate: certificate, ConnectionLimit: input.Limit, DrainTimeout: input.Drain}
	case "join":
		config.ClosedDataJoin = node.ClosedDataJoinProfile{AdmissionRoot: input.AdmissionRoot, Certificate: certificate, ConnectionLimit: input.Limit, DrainTimeout: input.Drain}
	case "forwarding":
		config.ClosedForwarding = node.ClosedForwardingProfile{Root: input.Root, Certificate: certificate, ConnectionLimit: input.Limit, DrainTimeout: input.Drain}
	}
	return config
}

// Public State/placement are explicit fixtures; all receiving duties use Run.
// These heaps contain role secrets, stay local and cannot qualify latency.
func runTextRoleObservationChild(t *testing.T, path string) {
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var input textRoleProcessInput
	if err := json.Unmarshal(raw, &input); err != nil {
		t.Fatal(err)
	}
	clear(raw)
	config := textRoleProcessConfig(input)
	ready := make(chan struct{}, 1)
	config.Emit = func(_ context.Context, event node.Event) error {
		if event.State == "READY" {
			select {
			case ready <- struct{}{}:
			default:
			}
		}
		return nil
	}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { _, err := node.Run(ctx, config); done <- err }()
	joined := false
	defer func() {
		cancel()
		if !joined {
			select {
			case err := <-done:
				if err != nil {
					t.Error(err)
				}
			case <-time.After(5 * time.Second):
				t.Error("role did not join")
			}
		}
	}()
	select {
	case <-ready:
	case err := <-done:
		joined = true
		t.Fatalf("role before READY: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("role READY timeout")
	}
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
			case <-time.After(5 * time.Second):
				t.Fatal("role stop timeout")
			}
			if err := captureTextRoleDurableState(input, path, "stopped"); err != nil {
				t.Fatal(err)
			}
			fmt.Println("role-stopped")
			return
		}
		if strings.HasPrefix(phase, "state ") {
			phase = strings.TrimPrefix(phase, "state ")
			if err := captureTextRoleDurableState(input, path, phase); err != nil {
				t.Fatal(err)
			}
			fmt.Println("role-state " + phase)
			continue
		}
		switch phase {
		case "startup", "published", "withdrawn":
		default:
			t.Fatal("invalid capture phase")
		}
		file, err := os.OpenFile(filepath.Join(input.Output, phase+".heap"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			t.Fatal(err)
		}
		debug.WriteHeapDump(file.Fd())
		syncErr := file.Sync()
		closeErr := file.Close()
		if syncErr != nil || closeErr != nil {
			t.Fatalf("heap write: %v / %v", syncErr, closeErr)
		}
		fmt.Println("role-dump " + phase)
	}
	t.Fatal("controller ended before stop")
}
