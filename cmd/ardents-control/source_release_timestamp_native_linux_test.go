//go:build linux && network_signing_native

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
	"github.com/theupdateframework/go-tuf/v2/metadata"
)

type sourceTimestampNativeInput struct {
	command, planPath, keys, parent, distribution string
	plan                                          sourceReleasePlan
	program, descriptor                           []byte
}

// The caller supplies its actual canonically signed Source distribution and
// original private fixture keys. No installed Source or successful authority
// substitute is constructed. Expiry below uses an explicit verifier reference
// time; actual installed wall-clock expiry remains a separate cohort control.
func exerciseSourceTimestampRefresh(t *testing.T, input sourceTimestampNativeInput) {
	t.Helper()
	write := func(plan sourceReleasePlan) {
		raw, err := json.Marshal(plan)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(input.planPath, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	defer write(input.plan)
	plan := input.plan
	plan.Schema = "ardents-source-timestamp-release-plan-v1"
	plan.Expires = plan.Reference.Add(10 * time.Minute)
	write(plan)
	short := filepath.Join(input.parent, "timestamp-short")
	child := exec.CommandContext(t.Context(), input.command, "source", "refresh-release-timestamp", input.planPath, input.keys, short, filepath.Join(input.parent, "timestamp-short-history"), input.distribution)
	if output, err := child.CombinedOutput(); err != nil || !bytes.Contains(output, []byte(`"operation":"source.refresh-release-timestamp"`)) {
		t.Fatal("canonical timestamp refresh", string(output), err)
	}
	read := func(directory, name string) []byte {
		raw, err := os.ReadFile(filepath.Join(directory, name))
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	// The next refresh reads a genuine complete public distribution, including
	// the inventory from the actual preparation owner, not a copied initial pin.
	distribution := filepath.Join(input.parent, "timestamp-short-distribution")
	if err := os.Mkdir(distribution, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{infrastructure.ProgramName, infrastructure.DescriptorName, infrastructure.TemplateName, "root.json", "snapshot.json", "targets.json", "timestamp.json"} {
		directory := input.distribution
		if name == "timestamp.json" {
			directory = short
		}
		if err := os.WriteFile(filepath.Join(distribution, name), read(directory, name), 0600); err != nil {
			t.Fatal(err)
		}
	}
	inventory := filepath.Join(input.parent, "timestamp-short-inventory")
	if _, err := prepareSourceArtifact(t.Context(), []string{"prepare-inventory", distribution, inventory}, nil); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(distribution, "infrastructure-enrollment.json"), read(inventory, "infrastructure-enrollment.json"), 0600); err != nil {
		t.Fatal(err)
	}
	long := filepath.Join(input.parent, "timestamp-long")
	plan.Expires = plan.Reference.Add(30 * time.Minute)
	write(plan)
	if _, err := signSourceRelease(t.Context(), []string{input.planPath, input.keys, long, filepath.Join(input.parent, "timestamp-long-history"), distribution}, sourceReleaseTimestamp, nil); err != nil {
		t.Fatal("timestamp renewal of unchanged actual Source", err)
	}
	for _, directory := range []string{short, long} {
		for _, name := range []string{"root.json", "snapshot.json", "targets.json"} {
			if !bytes.Equal(read(directory, name), read(input.distribution, name)) {
				t.Fatal("timestamp refresh changed another role", directory, name)
			}
		}
	}
	shortTimestamp, err := metadata.Timestamp().FromBytes(read(short, "timestamp.json"))
	if err != nil || shortTimestamp.Signed.Version != 2 || !shortTimestamp.Signed.Expires.Equal(input.plan.Reference.Add(10*time.Minute)) || shortTimestamp.Signed.Meta["snapshot.json"].Version != 1 {
		t.Fatal("short timestamp version/expiry/reference", err)
	}
	longTimestamp, err := metadata.Timestamp().FromBytes(read(long, "timestamp.json"))
	if err != nil || longTimestamp.Signed.Version != 3 || !longTimestamp.Signed.Expires.Equal(plan.Expires) || longTimestamp.Signed.Meta["snapshot.json"].Version != 1 {
		t.Fatal("renewed timestamp version/expiry/reference", err)
	}
	verifier, err := release.Open(filepath.Join(input.parent, "timestamp-causal-verifier"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := verifier.Close(); err != nil {
			t.Error(err)
		}
	}()
	evaluate := func(directory string, at time.Time, want release.Outcome) {
		files, err := release.StaticMetadataFiles(read(directory, "root.json"), read(directory, "timestamp.json"), read(directory, "snapshot.json"), read(directory, "targets.json"))
		if err != nil {
			t.Fatal(err)
		}
		for i, target := range []string{infrastructure.ProgramTarget, infrastructure.GenerationTarget} {
			artifact := [][]byte{input.program, input.descriptor}[i]
			decision := verifier.Evaluate(t.Context(), release.Inputs{RootBytes: read(directory, "root.json"), Files: files, TargetPath: target, Artifact: artifact,
				Local: release.LocalEnvironment{Platform: "linux-amd64", Architecture: "amd64", Environment: "alpha", Network: plan.Network, RefTime: at}})
			_, authorized := decision.Authorization()
			if want == release.OutcomeReleaseExpired {
				if decision.Outcome != want || authorized {
					t.Fatal("timestamp alone did not cause expiry", target, decision.Outcome, decision.Err())
				}
			} else if !authorized {
				t.Fatal("unchanged Source lost authorization", target, decision.Outcome, decision.Err())
			}
		}
	}
	evaluate(short, input.plan.Reference, "")
	expiredAt := shortTimestamp.Signed.Expires.Add(time.Second)
	evaluate(short, expiredAt, release.OutcomeReleaseExpired)
	evaluate(long, expiredAt, "")
	// A timestamp cannot extend any unchanged parent role's authority horizon.
	plan.Expires = input.plan.Expires.Add(time.Second)
	write(plan)
	output, history := filepath.Join(input.parent, "timestamp-over-parent"), filepath.Join(input.parent, "timestamp-over-parent-history")
	if raw, err := signSourceRelease(t.Context(), []string{input.planPath, input.keys, output, history, input.distribution}, sourceReleaseTimestamp, nil); err == nil || raw != nil {
		t.Fatal("timestamp extended unchanged parent expiry", err)
	}
	for _, path := range []string{output, history} {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("over-parent expiry dependent effect", path, err)
		}
	}
	exerciseSourceReleaseCoherence(t, input)
	exerciseSourceReleaseExpiryObserver(t, input)
}
