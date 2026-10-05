//go:build linux

package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

// The actual compiled holder must retain a failed physical Route close through
// later console commands and EOF. A refused reply alone cannot prove this: an
// absent handle also refuses, but would otherwise erase the terminal exit cause.
func TestRouteCompiledHolderRetainsFailedPrefixClose(t *testing.T) {
	binary := compiledCommand(t)
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	f, sockets, certificates := newJoinRouteFixture(t, transport.ClosedCarrierTCP)
	receivers, receiverBudgets := startJoinRouteReceivers(t, f, sockets, certificates)
	bundle, err := networkfixture.BuildClosed(f.spec)
	if err != nil {
		t.Fatal(err)
	}
	clockFile, stopClock := joinProcessClock(t)
	defer stopClock()
	var authorityKey [32]byte
	copy(authorityKey[:], f.spec.Authority.Public().(ed25519.PublicKey))
	networkPlan := networkAuthorityPlan{Root: filepath.Join(t.TempDir(), "network"), NetworkID: f.spec.NetworkID, Authorities: [][32]byte{authorityKey}, Threshold: 1, ProfileAuthority: authorityKey, ClockObservationFile: clockFile}
	config, err := networkStateConfig(&networkPlan)
	if err != nil {
		t.Fatal(err)
	}
	networkOwner, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	_, err = networkOwner.Accept(ctx, bundle.Epoch.Raw, bundle.Epoch.Inputs, bundle.Epoch.Materials)
	if err == nil {
		_, err = networkOwner.AcceptClosedProfile(bundle.Profile)
	}
	if err := errors.Join(err, networkOwner.Close()); err != nil {
		t.Fatal("independent signed State initialization", err)
	}
	root := t.TempDir()
	plan := routePrefixPlan{EntryRoot: filepath.Join(root, "entry"), InteriorRoot: filepath.Join(root, "interior"), HostingRoot: routeProcessBudget(t), Domain: 1, Deadline: time.Now().Add(45 * time.Second).UTC().Truncate(time.Second), Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
	process := startJoinCommandProcess(t, ctx, binary, "admission", "holder", "--config", hostingConfig(t, map[string]any{"root": t.TempDir(), "network": networkPlan, "role": admission.AllocationUser, "route": plan}))
	defer func() {
		if !process.waited {
			process.interrupt(t)
			_ = process.wait()
		}
	}()
	joinCommandStock(t, f, process, admission.AllocationUser)
	if reply, err := process.exchange(holderCommand{Operation: "join-prefix-open"}); err != nil || reply.Outcome != "completed" {
		t.Fatal("genuine compiled Source prefix", reply.Outcome, err)
	}
	choice, err := process.exchange(holderCommand{Operation: "rendezvous"})
	if err != nil || choice.Outcome != "completed" || choice.Recipient.Generation != 9 || (choice.Recipient.Node != [32]byte{20} && choice.Recipient.Node != [32]byte{21}) {
		t.Fatal("retained current Rendezvous", choice.Outcome, err)
	}

	// Stop both eligible authenticated Entry peers and join their real Carrier
	// trees. The subsequent real JOIN attempt observes the lost original prefix;
	// it supplies a causal completion barrier without sleeping for the reader.
	for _, receiver := range receivers[:2] {
		if err := receiver.Close(); err != nil {
			t.Fatal("joined Entry peer shutdown", err)
		}
	}
	intent := routeJoinIntent{Node: choice.Recipient.Node, Generation: choice.Recipient.Generation, Deadline: minRouteDeadline(choice.Recipient.NotAfter, plan.Deadline), SetupDeadline: time.Now().Add(8 * time.Second).UTC().Truncate(time.Second)}
	if _, err := rand.Read(intent.Secret[:]); err != nil {
		t.Fatal(err)
	}
	if _, err := rand.Read(intent.Context[:]); err != nil {
		t.Fatal(err)
	}
	defer func() { clear(intent.Secret[:]); clear(intent.Context[:]) }()
	if reply, err := process.exchange(holderCommand{Operation: "join-open", Join: intent}); err != nil || reply.Outcome != "refused" {
		t.Fatal("JOIN through physically retired original prefix", reply.Outcome, err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		if reply, err := process.exchange(holderCommand{Operation: "prefix-close"}); err != nil || reply.Outcome != "refused" {
			t.Errorf("failed prefix close outcome changed on attempt %d: outcome=%s error=%v", attempt+1, reply.Outcome, err)
		}
	}
	if reply, err := process.exchange(holderCommand{Operation: "join-prefix-open"}); err != nil || reply.Outcome != "refused" {
		t.Error("failed prefix was replaced", reply.Outcome, err)
	}
	if err := process.input.Close(); err != nil {
		t.Fatal(err)
	}
	exitErr := process.wait()
	var exit *exec.ExitError
	if !errors.As(exitErr, &exit) || exit.ExitCode() != 1 {
		t.Errorf("EOF erased failed prefix close: wanted exit 1, got %v; diagnostic=%s", exitErr, process.diagnostic.String())
	}
	for _, receiver := range receivers {
		if err := receiver.Close(); err != nil {
			t.Error("receiver cleanup", err)
		}
	}
	for _, budget := range receiverBudgets {
		observation, err := budget.Observe(ctx)
		if err != nil || observation.ReservedBytes != 0 {
			t.Error("receiver retained physical reservation after join", observation.ReservedBytes, err)
		}
	}
	budget, err := hosting.Open(plan.HostingRoot)
	if err != nil {
		t.Fatal("holder retained Hosting root after process join", err)
	}
	observation, observeErr := budget.Observe(ctx)
	closeErr := budget.Close()
	if observeErr != nil || closeErr != nil || observation.ReservedBytes != 0 {
		t.Error("holder retained physical reservation after process join", observation.ReservedBytes, observeErr, closeErr)
	}
}
