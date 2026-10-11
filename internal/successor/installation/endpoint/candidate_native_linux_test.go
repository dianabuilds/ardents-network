//go:build installation_native

package endpoint

import (
	"context"
	"errors"
	"testing"
)

func TestInstallationNativeCandidateJoinRefusesMissingPhysicalCustody(t *testing.T) {
	t.Parallel() // Own private filesystem/Unix lifetime; no shared manager effects.
	reader, request, previous, files, selected := successorStagingFixture(t)
	stage, err := stageSuccessorGeneration(t.Context(), reader, request, previous, files, selected, 65534)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stage.close() })
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	first := errors.New("original failed start")
	owned := &successorPreparation{ctx: ctx, inspection: reader, stage: stage, candidate: &inspectedGeneration{}, terminal: first}
	// The empty candidate is deliberately unqualified: no process, scope,
	// manager effect, Release proof or successful join is supplied. The join
	// boundary must refuse before manager I/O while retaining actual custody.
	original := &installedCandidate{preparation: owned, startAttempted: true, terminal: first}
	owned.started = original
	for range 2 {
		if err := original.joinAttempt(context.WithoutCancel(ctx)); !errors.Is(err, ErrBinding) {
			t.Fatal("missing physical custody entered manager observation", err)
		}
		if !errors.Is(original.terminal, first) || !errors.Is(original.terminal, context.Canceled) {
			t.Fatal("failure-only join lost original failure or cancellation", original.terminal)
		}
		if original.joined || original.released || owned.started != original || owned.stage != stage || owned.inspection != reader {
			t.Fatal("missing physical join changed original owner or completion")
		}
		if reader.lease.writer == nil || reader.lease.root == nil {
			t.Fatal("missing physical join released writer lease")
		}
		if _, err := reader.lease.writer.Stat(); err != nil {
			t.Fatal("missing physical join closed actual writer", err)
		}
		if err := stage.observe(); err != nil {
			t.Fatal("missing physical join closed actual stage", err)
		}
	}
}

func TestInstallationNativeCandidateCloseRetainsUnjoinedWriterAndStage(t *testing.T) {
	t.Parallel() // Own private filesystem/Unix lifetime; no shared manager effects.
	reader, request, previous, files, selected := successorStagingFixture(t)
	stage, err := stageSuccessorGeneration(t.Context(), reader, request, previous, files, selected, 65534)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stage.close() })
	first := errors.New("original candidate failure")
	owned := &successorPreparation{ctx: t.Context(), inspection: reader, stage: stage, terminal: first}
	// This deliberately incomplete attempt supplies no successful manager,
	// process, scopes, Release pair or physical join. Its refusal must keep the
	// actual fixture's original file handles and writer lease intact.
	original := &installedCandidate{preparation: owned, terminal: first}
	owned.started = original
	for range 2 {
		if err := owned.close(); !errors.Is(err, first) || !errors.Is(err, ErrBinding) {
			t.Fatal("unjoined candidate became completed cleanup", err)
		}
		if original.released || owned.started != original || owned.stage != stage || owned.inspection != reader {
			t.Fatal("unjoined candidate lost its original owner")
		}
		if reader.lease.root == nil || reader.lease.writer == nil {
			t.Fatal("unjoined candidate released writer lease")
		}
		if _, err := reader.lease.writer.Stat(); err != nil {
			t.Fatal("unjoined candidate physically closed original writer", err)
		}
		if err := stage.observe(); err != nil {
			t.Fatal("unjoined candidate closed staged custody", err)
		}
	}
}
