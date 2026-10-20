//go:build linux

package main

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"io"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network/duty"
	"github.com/dianabuilds/ardents-network/internal/successor/network/source"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

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
	if err := superviseSource(t.Context(), owner, admissionCtx.Err, deny, failedSourceOutput{outputFailure}, io.Discard); !errors.Is(err, outputFailure) {
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
