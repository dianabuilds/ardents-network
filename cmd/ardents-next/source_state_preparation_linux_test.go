package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/duty"
	"github.com/dianabuilds/ardents-network/internal/network/source"
	"github.com/dianabuilds/ardents-network/internal/network/source/servingdeclaration"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/processresource"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

// These units exercise the finite domain composition as the actual test user.
// The command's separate fixed-system-account gate has its own refusal test.
// Genuine signed State and actual local files grant no manager/runtime claim.
func sourceInitialFixture(t *testing.T) (servingdeclaration.ServingDeclaration, string, string) {
	t.Helper()
	return sourceInitialFixtureAt(t, t.TempDir())
}

func sourceInitialFixtureAt(t *testing.T, root string) (servingdeclaration.ServingDeclaration, string, string) {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	authority := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{71}, 32))
	node := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{72}, 32))
	network := [32]byte{73}
	record, err := networkfixture.BuildRecord(networkfixture.RecordSpec{NetworkID: network, NodeID: [32]byte{74}, Generation: 1, ValidFrom: now.Add(-time.Hour), ValidUntil: now.Add(2 * time.Hour), Family: "fixture-family", Endpoint: "127.0.0.1:4101", Carrier: "ardents-carrier-tcp-tls-v2", Capability: 2, Capacity: 1, PrivateKey: node})
	if err != nil {
		t.Fatal(err)
	}
	epoch, err := networkfixture.BuildEpoch(networkfixture.EpochSpec{NetworkID: network, Number: 1, ValidFrom: now.Add(-time.Minute), ValidUntil: now.Add(time.Hour), Inputs: [][]byte{record.Raw}, Accepted: []networkfixture.Record{record}, AssignmentSeed: [32]byte{75}, Profile: "ardents-route-v3", Version: 3, Domains: []string{"initiator", "introduction", "rendezvous", "responder"}, Authorities: []ed25519.PrivateKey{authority}})
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := source.EncodeBundle(source.Bundle{Epoch: epoch.Raw, Inputs: epoch.Inputs, Materials: [][]byte{epoch.Materials[0]}})
	if err != nil {
		t.Fatal(err)
	}
	plan := servingdeclaration.ServingDeclaration{Network: &servingdeclaration.ServingNetwork{Root: filepath.Join(root, "state"), NetworkID: network, Authorities: [][32]byte{[32]byte(authority.Public().(ed25519.PublicKey))}, Threshold: 1, ProfileAuthority: [32]byte(authority.Public().(ed25519.PublicKey)), ClockObservationFile: filepath.Join(root, "clock")}, ProcessProfile: "ardents-source-process-v1", ParticipationRoot: filepath.Join(root, "participation"), Address: "127.0.0.1:4100", Certificate: "/unopened/source.crt", PrivateKey: "/unopened/source.key", ClientRoot: "/unopened/client-ca.pem", ClientPins: [][32]byte{{76}}}
	for _, path := range []string{plan.Network.Root, plan.ParticipationRoot} {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	planRaw, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	planPath, bundlePath := filepath.Join(root, "serve.json"), filepath.Join(root, "bundle.bin")
	for path, raw := range map[string][]byte{planPath: planRaw, bundlePath: bundle, plan.Network.ClockObservationFile: []byte("clock")} {
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	return plan, planPath, bundlePath
}

func sourceHistoryBytes(t *testing.T, root string) map[string]string {
	t.Helper()
	result := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		result[path] = string(raw)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestSourceInitialPreparationCreatesBothRetainedHistories(t *testing.T) {
	plan, planPath, bundlePath := sourceInitialFixture(t)
	if err := prepareSourceStateInputs(t.Context(), planPath, bundlePath); err != nil {
		t.Fatal(err)
	}
	stateBefore, rolesBefore := sourceHistoryBytes(t, plan.Network.Root), sourceHistoryBytes(t, plan.ParticipationRoot)
	if err := prepareSourceStateInputs(t.Context(), planPath, bundlePath); err == nil {
		t.Fatal("completed preparation repeated")
	}
	if !reflect.DeepEqual(stateBefore, sourceHistoryBytes(t, plan.Network.Root)) || !reflect.DeepEqual(rolesBefore, sourceHistoryBytes(t, plan.ParticipationRoot)) {
		t.Fatal("repeat changed retained state")
	}
	config, err := state.SourcePreparationConfig(plan)
	if err != nil {
		t.Fatal(err)
	}
	owner, err := state.OpenRetained(config)
	if err != nil {
		t.Fatal("prepared State cannot reopen", err)
	}
	view, err := owner.Current()
	if err != nil || view.Epoch != 1 {
		t.Fatal("signed State not retained", view, err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	roles, err := duty.Open(duty.Config{Root: plan.ParticipationRoot, Clock: time.Now})
	if err != nil {
		t.Fatal("prepared participation cannot reopen", err)
	}
	conflict, err := roles.Conflict([32]byte{74}, [32]byte{75})
	if err != nil || conflict {
		t.Fatal("preparation created participation authority", conflict, err)
	}
	if err := roles.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestSourceInitialPreparationRefusesPriorAndPartialRoots(t *testing.T) {
	for _, which := range []string{"state", "participation"} {
		t.Run(which, func(t *testing.T) {
			plan, planPath, bundlePath := sourceInitialFixture(t)
			root := plan.Network.Root
			if which == "participation" {
				root = plan.ParticipationRoot
			}
			if err := os.WriteFile(filepath.Join(root, "partial"), []byte("retained"), 0600); err != nil {
				t.Fatal(err)
			}
			beforeState, beforeRoles := sourceHistoryBytes(t, plan.Network.Root), sourceHistoryBytes(t, plan.ParticipationRoot)
			if err := prepareSourceStateInputs(t.Context(), planPath, bundlePath); err == nil {
				t.Fatal("partial history adopted")
			}
			if !reflect.DeepEqual(beforeState, sourceHistoryBytes(t, plan.Network.Root)) || !reflect.DeepEqual(beforeRoles, sourceHistoryBytes(t, plan.ParticipationRoot)) {
				t.Fatal("refusal changed original histories")
			}
		})
	}
}

func prepareObservedSourceState(ctx context.Context, planPath, bundlePath string, afterAccept func()) (result error) {
	custody := sourceStateCustody{files: newSourceInputs(ctx, nil), uid: uint32(os.Geteuid())}
	defer func() { result = errors.Join(result, custody.close(), ctx.Err()) }()
	plan, bundle, err := custody.read(planPath, bundlePath)
	if err != nil {
		return err
	}
	// Arm the intervention only after genuine acceptance publishes its pointer.
	// The ordinary preparation still reobserves the real input descriptors.
	var observed bool
	check := func() error {
		if !observed {
			if _, err := os.Stat(filepath.Join(plan.Network.Root, "current")); err == nil {
				observed = true
				afterAccept()
			} else if !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
		return custody.check()
	}
	return state.PrepareInitialSource(ctx, plan, bundle, check)
}

func TestSourceInitialPreparationRetainsAcceptedStateOnInterruption(t *testing.T) {
	for _, failure := range []string{"cancel", "competing-participation"} {
		t.Run(failure, func(t *testing.T) {
			plan, planPath, bundlePath := sourceInitialFixture(t)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			err := prepareObservedSourceState(ctx, planPath, bundlePath, func() {
				if failure == "cancel" {
					cancel()
				} else {
					if err := os.WriteFile(filepath.Join(plan.ParticipationRoot, "original-partial"), []byte("must remain"), 0600); err != nil {
						t.Fatal(err)
					}
				}
			})
			if err == nil || failure == "cancel" && !errors.Is(err, context.Canceled) {
				t.Fatal("interrupted preparation succeeded", err)
			}
			beforeState, beforeRoles := sourceHistoryBytes(t, plan.Network.Root), sourceHistoryBytes(t, plan.ParticipationRoot)
			if err := prepareSourceStateInputs(t.Context(), planPath, bundlePath); err == nil {
				t.Fatal("partial preparation silently continued")
			}
			if !reflect.DeepEqual(beforeState, sourceHistoryBytes(t, plan.Network.Root)) || !reflect.DeepEqual(beforeRoles, sourceHistoryBytes(t, plan.ParticipationRoot)) {
				t.Fatal("retry changed original partial preparation")
			}
			config, err := state.SourcePreparationConfig(plan)
			if err != nil {
				t.Fatal(err)
			}
			owner, err := state.OpenRetained(config)
			if err != nil {
				t.Fatal("accepted State erased", err)
			}
			view, err := owner.Current()
			if err != nil || view.Epoch != 1 {
				t.Fatal("accepted Epoch lost", view, err)
			}
			if err := owner.Close(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSourceInitialPreparationRejectsForgedEpochBeforeParticipation(t *testing.T) {
	plan, planPath, bundlePath := sourceInitialFixture(t)
	raw, err := os.ReadFile(bundlePath)
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := source.DecodeBundle(raw)
	if err != nil {
		t.Fatal(err)
	}
	bundle.Epoch[len(bundle.Epoch)-1] ^= 1
	raw, err = source.EncodeBundle(bundle)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bundlePath, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err := prepareSourceStateInputs(t.Context(), planPath, bundlePath); err == nil {
		t.Fatal("forged Epoch accepted")
	}
	if len(sourceHistoryBytes(t, plan.ParticipationRoot)) != 0 {
		t.Fatal("forged State initialized participation")
	}
}

func TestSourceInitialPreparationDoesNotRecreateLostParticipation(t *testing.T) {
	plan, planPath, bundlePath := sourceInitialFixture(t)
	if err := prepareSourceStateInputs(t.Context(), planPath, bundlePath); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(plan.ParticipationRoot)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if err := os.Remove(filepath.Join(plan.ParticipationRoot, entry.Name())); err != nil {
			t.Fatal(err)
		}
	}
	before := sourceHistoryBytes(t, plan.Network.Root)
	if err := prepareSourceStateInputs(t.Context(), planPath, bundlePath); err == nil {
		t.Fatal("lost history treated as first preparation")
	}
	if !reflect.DeepEqual(before, sourceHistoryBytes(t, plan.Network.Root)) || len(sourceHistoryBytes(t, plan.ParticipationRoot)) != 0 {
		t.Fatal("repeat repaired or changed retained histories")
	}
	if owner, err := duty.Open(duty.Config{Root: plan.ParticipationRoot, Clock: time.Now}); err == nil {
		_ = owner.Close()
		t.Fatal("retained opening initialized erased participation")
	}
	if len(sourceHistoryBytes(t, plan.ParticipationRoot)) != 0 {
		t.Fatal("retained refusal created history")
	}
}

func TestSourcePreparationConfigHasNoServingOrAcquisition(t *testing.T) {
	plan := servingdeclaration.ServingDeclaration{ProcessProfile: processresource.SourceProfile, ParticipationRoot: "/var/lib/source-participation",
		Network: &servingdeclaration.ServingNetwork{Root: "/var/lib/source-state", NetworkID: [32]byte{1}, Authorities: [][32]byte{{2}},
			Threshold: 1, ProfileAuthority: [32]byte{2}, ClockObservationFile: "/var/lib/source-clock"},
		Address: "127.0.0.1:4000", Certificate: "/unopened/certificate", PrivateKey: "/unopened/private-key", ClientRoot: "/unopened/ca", ClientPins: [][32]byte{{3}}}
	config, err := state.SourcePreparationConfig(plan)
	if err != nil {
		t.Fatal(err)
	}
	if config.Root != plan.Network.Root || config.Clock == nil || !config.Now.IsZero() || config.ClockObservationFile != plan.Network.ClockObservationFile || config.LocalRoleStateRoot != "" ||
		config.Source.ServeAddress != "" || config.Source.ServeCertificate.PrivateKey != nil || config.Source.ClientCertificate.PrivateKey != nil {
		t.Fatal("offline preparation acquired runtime configuration")
	}
	if _, details, err := source.New(config.Source, config.Authorities); err != nil || details.Configured || details.Serving {
		t.Fatal("offline configuration declares Source transport", details, err)
	}
	// Declaration projection supplies no accepted State or trusted time. State's
	// genuine verifier separately checks the pins, signatures, time and floors.
	for _, root := range []string{plan.Network.Root, plan.Network.Root + "/child", "/var/lib"} {
		changed := plan
		network := *plan.Network
		changed.Network = &network
		changed.ParticipationRoot = root
		if _, err := state.SourcePreparationConfig(changed); err == nil {
			t.Fatal("overlapping roots accepted", root)
		}
	}
}

func TestSourcePreparationRefusesOtherPrincipalBeforeInputEffects(t *testing.T) {
	if os.Geteuid() >= 100 && os.Geteuid() <= 999 {
		t.Fatal("ordinary test profile requires an unprivileged development user outside the fixed system-account range")
	}
	var out, diagnostic bytes.Buffer
	if code := run(t.Context(), []string{"source", "prepare-state", "/unopened-plan", "/unopened-bundle"}, &out, &diagnostic); code == 0 || out.Len() != 0 {
		t.Fatal("foreign principal reached preparation or receipt", code)
	}
}

func TestSourceInitialPreparationRefusesEarlierInputSubstitutionDuringBundleRead(t *testing.T) {
	for _, change := range []string{"file", "ancestor"} {
		t.Run(change, func(t *testing.T) {
			plan, originalPlan, bundlePath := sourceInitialFixture(t)
			raw, err := os.ReadFile(originalPlan)
			if err != nil {
				t.Fatal(err)
			}
			planDir := filepath.Join(filepath.Dir(originalPlan), "public-plan")
			if err := os.Mkdir(planDir, 0700); err != nil {
				t.Fatal(err)
			}
			planPath := filepath.Join(planDir, "serve.json")
			if err := os.Rename(originalPlan, planPath); err != nil {
				t.Fatal(err)
			}
			armed, replaced := false, false
			observe := func(phase string, file *os.File) {
				if !armed || replaced || phase != "check" || file.Name() != filepath.Base(bundlePath) {
					return
				}
				replaced = true
				target := planPath
				if change == "ancestor" {
					target = planDir
				}
				if err := os.Rename(target, target+".original"); err != nil {
					t.Fatal(err)
				}
				if change == "ancestor" {
					if err := os.Mkdir(planDir, 0700); err != nil {
						t.Fatal(err)
					}
				}
				if err := os.WriteFile(planPath, raw, 0600); err != nil {
					t.Fatal(err)
				}
			}
			custody := sourceStateCustody{files: newSourceInputs(t.Context(), observe), uid: uint32(os.Geteuid())}
			t.Cleanup(func() {
				if err := custody.close(); replaced && !errors.Is(err, errSourceInput) {
					t.Error("original custody failure lost at close", err)
				}
			})
			readPlan, bundle, err := custody.read(planPath, bundlePath)
			if err != nil {
				t.Fatal(err)
			}
			armed = true
			if err := state.PrepareInitialSource(t.Context(), readPlan, bundle, custody.check); !errors.Is(err, errSourceInput) || !replaced {
				t.Fatalf("earlier input substitution = %v, replaced = %v", err, replaced)
			}
			if len(sourceHistoryBytes(t, plan.Network.Root)) != 0 || len(sourceHistoryBytes(t, plan.ParticipationRoot)) != 0 {
				t.Fatal("refused inputs initialized history")
			}
		})
	}
}
