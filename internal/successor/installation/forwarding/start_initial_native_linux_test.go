//go:build linux && network_signing_native

package forwarding

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

func TestForwardingStartedNativeRefusesMissingStartHistory(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	path := nativeCompletedBinding(t)
	completed := nativeCompleted(t, ctx)
	b := completed.original.binding
	if err := completed.CheckHistoryPath(b.History); err != nil {
		t.Fatal(err)
	}
	if err := completed.Close(); err != nil {
		t.Fatal(err)
	}
	v, err := release.OpenRetained(b.History)
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	if err := Stop(ctx, v, path, b.History); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("root stop accepted an initial transcript without a start", err)
	}
	if err := Restart(ctx, v, infrastructure.ForwardingAuthorization{}, path, os.Getenv("ARDENTS_FORWARDING_INITIAL_DISTRIBUTION"), b.History); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("restart accepted missing initial start history", err)
	}
	oldGC := debug.SetGCPercent(-1)
	defer debug.SetGCPercent(oldGC)
	before := nativeInputFDCount(t)
	for range 3 {
		h, err := ReadStarted(ctx, v, path, b.History)
		if !errors.Is(err, os.ErrNotExist) || h.original != nil {
			_ = h.Close()
			t.Fatal("initial transcript impersonated a published start", err)
		}
		if err := h.Close(); err != nil {
			t.Fatal("empty refused handle acquired cleanup", err)
		}
		if err := Stop(ctx, v, path, b.History); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("root stop lost missing-start refusal", err)
		}
		if err := Restart(ctx, v, infrastructure.ForwardingAuthorization{}, path, os.Getenv("ARDENTS_FORWARDING_INITIAL_DISTRIBUTION"), b.History); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("restart lost missing-start refusal", err)
		}
	}
	if after := nativeInputFDCount(t); after != before {
		t.Fatal("failed independent start readers leaked original descriptors", before, after)
	}
	probeCompletedLock(t, b.Request.InstallationRoot+".forwarding-initial-journal", true)
	if err := v.Close(); err != nil {
		t.Fatal(err)
	}
	distribution := os.Getenv("ARDENTS_FORWARDING_INITIAL_DISTRIBUTION")
	if !validPath(distribution) {
		t.Fatal("invalid environment: genuine canonical distribution required")
	}
	command := exec.CommandContext(ctx, filepath.Join(distribution, programName), "forwarding", "inspect-start-history", b.History, path)
	output, err := command.CombinedOutput()
	if err == nil || strings.Contains(string(output), "forwarding-start-history-verified") {
		t.Fatal("canonical reader fabricated completed start", err, string(output))
	}
	command = exec.CommandContext(ctx, filepath.Join(distribution, programName), "forwarding", "stop", b.History, path)
	output, err = command.CombinedOutput()
	if err == nil || strings.Contains(string(output), "forwarding-original-stop-completed") {
		t.Fatal("canonical stop accepted missing start history", err, string(output))
	}
	command = exec.CommandContext(ctx, filepath.Join(distribution, programName), "forwarding", "restart", distribution, b.History, "alpha", "controlled-initial-component", path)
	output, err = command.CombinedOutput()
	if err == nil || strings.Contains(string(output), "forwarding-same-generation-restart-completed") {
		t.Fatal("canonical restart accepted missing start history", err, string(output))
	}
	for _, reserved := range append(b.Request.reservedSiblingPaths()[2:], attemptNumber(0).directory(b.Request.InstallationRoot)) {
		if _, err := os.Lstat(reserved); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("read-only refusal created future start history", reserved, err)
		}
	}
	probeCompletedLock(t, b.Request.InstallationRoot+".forwarding-initial-journal", true)
}

func TestForwardingStartInitialNativeRefusesOrdinaryRootExecutable(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	path := nativeCompletedBinding(t)
	distribution := os.Getenv("ARDENTS_FORWARDING_INITIAL_DISTRIBUTION")
	if !validPath(distribution) {
		t.Fatal("invalid environment: exact genuine distribution required")
	}
	completed, err := ReadCompleted(ctx, path)
	if err != nil {
		t.Fatal("genuine completed initial prerequisite", err)
	}
	defer completed.Close()
	b := completed.original.binding
	if err := completed.IndependentPaths(distribution, b.History); err != nil {
		t.Fatal(err)
	}
	candidate, err := enrollment.ReadInfrastructureCandidate(ctx, distribution)
	if err != nil {
		t.Fatal(err)
	}
	get := func(name string) []byte { raw, _ := candidate.File(name); return raw }
	metadata, err := release.StaticMetadataFiles(get("root.json"), get("timestamp.json"), get("snapshot.json"), get("targets.json"))
	if err != nil {
		t.Fatal(err)
	}
	v, err := release.OpenRetained(b.History)
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	a, err := infrastructure.AuthenticateForwardingCandidate(ctx, v, candidate, release.Inputs{
		RootBytes: get("root.json"), Artifact: get(infrastructure.ProgramName), TargetPath: infrastructure.ProgramTarget, Files: metadata,
		Local: release.LocalEnvironment{Platform: "linux-amd64", Architecture: "amd64", Environment: "alpha", Network: "controlled-initial-component", RefTime: time.Now().UTC()},
	})
	if err != nil {
		t.Fatal("genuine fresh coherent Release pair prerequisite", err)
	}
	if err := completed.CheckAuthorization(v, a); err != nil {
		t.Fatal("independent original native/authorization match prerequisite", err)
	}
	if err := completed.Close(); err != nil {
		t.Fatal(err)
	}
	// Root independently reopens its own original completed files and fresh
	// authorization. This ordinary test executable must fail at its actual mapped
	// program before native manager, completion, journal or Start effects.
	err = StartInitial(ctx, v, a, path, distribution, b.History)
	if !errors.Is(err, errControllerProgram) || !errors.Is(err, ErrBinding) {
		t.Fatal("wrong executable did not reach exact root controller admission", err)
	}
	for _, reserved := range append(b.Request.reservedSiblingPaths()[2:], attemptNumber(0).directory(b.Request.InstallationRoot)) {
		if _, err := os.Lstat(reserved); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("refused root start had a native birth effect", reserved, err)
		}
	}
	for _, root := range []string{b.Request.StateRoot, b.Request.ParticipationRoot, b.Request.SpendRoot, b.Request.HostingRoot} {
		entries, err := os.ReadDir(root)
		if err != nil || len(entries) != 0 {
			t.Fatal("root refusal initialized semantic history", err)
		}
	}
	probeCompletedLock(t, b.Request.InstallationRoot+".forwarding-initial-journal", true)
	if err := v.Close(); err != nil {
		t.Fatal("original verifier join", err)
	}
}
