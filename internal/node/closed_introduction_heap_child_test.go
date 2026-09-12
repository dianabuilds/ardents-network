//go:build linux

package node

import (
	"bufio"
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
)

type introductionHeapInput struct {
	Snapshot                        dutyFacts
	View                            state.ClosedRouteView
	Certificates                    [][]byte
	PrivateKey                      ed25519.PrivateKey
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
	config := Config{NetworkID: input.Snapshot.NetworkID, NodeID: input.Snapshot.NodeID, IdentityKey: input.PrivateKey,
		Current:              func() (DutyView, error) { return input.Snapshot, nil },
		CurrentClosedProfile: func() (state.ClosedProfileView, bool) { return input.View.Profile, true },
		CurrentClosedRoute:   func() (state.ClosedRouteView, bool) { return input.View, true },
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
