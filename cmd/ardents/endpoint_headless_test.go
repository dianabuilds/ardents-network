package main

import (
	"bytes"
	"crypto/ed25519"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route"
)

func TestHeadlessRuntimeRefusesRetiredV1BeforeEffects(t *testing.T) {
	baseline := retiredHeadlessV1Plan(t)
	mixed := baseline
	mixedRoot := filepath.Dir(baseline.NetworkStateRoot)
	mixed.TextTokenRoot = filepath.Join(mixedRoot, "mixed-tokens")
	mixed.ReaderPermission = headlessPermissionPlan{RequestPath: filepath.Join(mixedRoot, "mixed-request"),
		ResponsePath: filepath.Join(mixedRoot, "mixed-response"), Maxima: [3]uint32{1, 1, 1}}

	for _, test := range []struct {
		name string
		plan headlessRuntimePlan
	}{{name: "previously valid", plan: baseline}, {name: "mixed v1 and v2", plan: mixed}} {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			err := runHeadlessRuntime(t.Context(), writeHeadlessTextPlan(t, test.plan), &output)
			if !errors.Is(err, errHeadlessRuntimeV1Retired) {
				t.Fatalf("retired v1 outcome = %v", err)
			}
			if output.Len() != 0 {
				t.Fatalf("retired v1 wrote output: %q", output.Bytes())
			}
			assertHeadlessPlanPathsAbsent(t, test.plan)
		})
	}
}

func TestHeadlessRuntimeV2ReachesSelectedRuntimeRefusalWithoutV1Fallback(t *testing.T) {
	valid := headlessTextPlanFixture(t)
	var output bytes.Buffer
	want := "text runtime plan requires the qualified Linux installation"
	if runtime.GOOS == "linux" {
		want = "text runtime requires a pollable local event output"
	}
	if err := runHeadlessRuntime(t.Context(), writeHeadlessTextPlan(t, valid), &output); err == nil || err.Error() != want {
		t.Fatalf("valid v2 dispatch outcome = %v, want %q", err, want)
	}
	if output.Len() != 0 {
		t.Fatalf("v2 refusal wrote output: %q", output.Bytes())
	}
	assertHeadlessPlanPathsAbsent(t, valid)

	broken := valid
	broken.ReaderPermission.ResponsePath = ""
	if err := runHeadlessRuntime(t.Context(), writeHeadlessTextPlan(t, broken), &output); err == nil || err.Error() != "text runtime paths must be distinct, absolute and canonical" {
		t.Fatalf("broken v2 decoder outcome = %v", err)
	}
	assertHeadlessPlanPathsAbsent(t, broken)
}

func retiredHeadlessV1Plan(t *testing.T) headlessRuntimePlan {
	t.Helper()
	plan := headlessTextPlanFixture(t)
	plan.Schema = "ardents-headless-runtime-v1"
	plan.NetworkProfile = route.Profile
	plan.ClosedProfileAuthority = ""
	plan.TextTokenRoot = ""
	plan.ReaderPermission = headlessPermissionPlan{}
	plan.PublisherPermission = headlessPermissionPlan{}
	plan.TransitAcquisitionRoot = filepath.Join(filepath.Dir(plan.NetworkStateRoot), "transit")
	plan.BytesEachDirection = 4096
	return plan
}

func assertHeadlessPlanPathsAbsent(t *testing.T, plan headlessRuntimePlan) {
	t.Helper()
	for _, path := range []string{plan.NetworkStateRoot, plan.EntryStateRoot, plan.TransitAcquisitionRoot,
		plan.ApplicationSocket, plan.AdministrationSocket, plan.PublicationRoot, plan.LocalRoleStateRoot, plan.ServiceInstanceRoot,
		plan.TimeConfidenceFile, plan.NetworkSourcePlan, plan.AlphaCorpusStateRoot, plan.TextTokenRoot,
		plan.ReaderPermission.RequestPath, plan.ReaderPermission.ResponsePath,
		plan.PublisherPermission.RequestPath, plan.PublisherPermission.ResponsePath} {
		if path == "" {
			continue
		}
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Errorf("retired v1 created %q: %v", path, err)
		}
	}
}

func TestHeadlessRuntimeSourcePlanMustShareItsStateOwners(t *testing.T) {
	directory := t.TempDir()
	sharedRoles, sharedClock := filepath.Join(directory, "roles"), filepath.Join(directory, "clock")
	public := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{9}, ed25519.SeedSize)).Public().(ed25519.PublicKey)
	identity := [32]byte{81}
	plan := decodedHeadlessRuntimePlan{NetworkID: [32]byte{80}, NetworkAuthorities: map[[32]byte]ed25519.PublicKey{identity: public},
		headlessRuntimePlan: headlessRuntimePlan{NetworkThreshold: 1, LocalRoleStateRoot: sharedRoles, TimeConfidenceFile: sharedClock}}
	matching := state.Config{NetworkID: plan.NetworkID, Authorities: map[[32]byte]ed25519.PublicKey{identity: public}, Threshold: 1,
		LocalRoleStateRoot: sharedRoles, ClockObservationFile: sharedClock, AutomaticRefreshInterval: time.Second}
	if !matchesHeadlessSourcePlan(plan, matching) {
		t.Fatal("matching source plan was rejected")
	}
	matching.LocalRoleStateRoot = filepath.Join(directory, "other-roles")
	if matchesHeadlessSourcePlan(plan, matching) {
		t.Fatal("source plan with a different local duty owner was accepted")
	}
	matching.LocalRoleStateRoot, matching.ClockObservationFile = sharedRoles, filepath.Join(directory, "other-clock")
	if matchesHeadlessSourcePlan(plan, matching) {
		t.Fatal("source plan with a different clock owner was accepted")
	}
	matching.ClockObservationFile, matching.AutomaticRefreshInterval = sharedClock, 0
	if matchesHeadlessSourcePlan(plan, matching) {
		t.Fatal("source plan without automatic refresh was accepted")
	}
}
