//go:build linux && amd64 && network_signing_native

package infrastructure

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Inspect the actual canonical ELF and closed declaration without invoking it.
// Format-only receipt/journal pins below are not authority or bootstrap consent.
func TestSourceRefusalObserverProfileInspectionNative(t *testing.T) {
	program := os.Getenv("ARDENTS_OBSERVER_INSPECT_PROGRAM")
	if !sourcePath(program) {
		t.Fatal("actual offline canonical program required")
	}
	raw, err := os.ReadFile(program)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "source")
	zero := strings.Repeat("0", 64)
	profile := sourceRefusalObserverProfile{Schema: "source-recovery-refusal-observer-v1", ProgramSHA256: sourcePhysicalSHA(raw), PublicReceiptSHA256: zero,
		Arguments: []string{"installation", "recover-source", filepath.Dir(program), "/testtmp/history", "alpha", "ardents-source-qualification", "/testtmp/request", root + ".source-initial-journal"}, JournalSHA256: zero, JournalMode: "0600"}
	load := func(t *testing.T, body []byte) (*sourceRefusalObserverProfile, error) {
		t.Helper()
		path := filepath.Join(t.TempDir(), "refusal.json")
		if err := os.WriteFile(path, body, 0600); err != nil {
			t.Fatal(err)
		}
		return readSourceRefusalObserverProfile(path, sourcePhysicalSHA(body), program, root)
	}
	encode := func(p sourceRefusalObserverProfile) []byte {
		t.Helper()
		body, err := json.Marshal(p)
		if err != nil {
			t.Fatal(err)
		}
		return append(body, '\n')
	}
	for _, journal := range []string{root + ".source-initial-journal", root + ".source-replacement-" + strings.Repeat("1", 64) + "-journal"} {
		for _, external := range [][2]bool{{false, false}, {true, false}, {false, true}, {true, true}} {
			p := profile
			p.Arguments = append([]string(nil), profile.Arguments...)
			p.Arguments[7], p.ExternalJournalLease, p.ExternalReleaseLease = journal, external[0], external[1]
			if external[0] {
				p.JournalMode = "0640"
			}
			admitted, err := load(t, encode(p))
			if err != nil {
				t.Fatal("closed initial/replacement refusal declaration", err)
			}
			result, err := observeSourceProcess(program, root, "", "refusal-closed", nil, true, 2, nil, admitted)
			if err != nil || !result.InspectionOnly || result.Entry == 0 || result.RefusalAdmissionEntry == 0 || result.RefusalAdmissionObserved || result.RootPID != 0 || result.RootExit != -1 || result.InstalledExecution || result.RefusalOwnershipReleasedBeforeExit {
				t.Fatal("offline refusal entry inspection executed command or claimed cleanup", result, err)
			}
			admitted.ProgramSHA256 = zero
			if _, err := observeSourceProcess(program, root, "", "refusal-closed", nil, true, 2, nil, admitted); err == nil {
				t.Fatal("refusal observer accepted a different program")
			}
		}
	}
	for attempt := uint32(0); attempt < uint32(sourceAttemptLimit); attempt++ {
		p := profile
		p.Schema = "source-startup-recovery-refusal-observer-v1"
		p.Arguments = append([]string(nil), profile.Arguments...)
		p.Arguments[7] = sourceAttemptNumber(attempt).journal(root)
		p.StartupPending = &sourcePendingGuardProfile{Attempt: attempt, GID: 500}
		admitted, err := load(t, encode(p))
		if err != nil {
			t.Fatal("exact startup attempt refusal profile", attempt, err)
		}
		result, err := observeSourceProcess(program, root, "", "refusal-closed", nil, true, 2, nil, admitted)
		if err != nil || !result.InspectionOnly || result.RootPID != 0 || result.InstalledExecution || result.RefusalStartupPendingObserved || result.RefusalStartupPendingDevice != 0 || result.RefusalStartupPendingInode != 0 || result.RefusalStartupPendingSHA256 != "" {
			t.Fatal("offline startup refusal inspection executed or claimed actual observation", attempt, result, err)
		}
		for name, change := range map[string]func(*sourceRefusalObserverProfile){
			"base-purpose":    func(p *sourceRefusalObserverProfile) { p.Schema = "source-recovery-refusal-observer-v1" },
			"missing-purpose": func(p *sourceRefusalObserverProfile) { p.StartupPending = nil },
			"foreign-attempt": func(p *sourceRefusalObserverProfile) {
				p.StartupPending.Attempt = (attempt + 1) % uint32(sourceAttemptLimit)
			},
			"out-of-bound-attempt": func(p *sourceRefusalObserverProfile) { p.StartupPending.Attempt = uint32(sourceAttemptLimit) },
			"root-group":           func(p *sourceRefusalObserverProfile) { p.StartupPending.GID = 0 },
			"out-of-bound-group":   func(p *sourceRefusalObserverProfile) { p.StartupPending.GID = 1000 },
			"journal-held":         func(p *sourceRefusalObserverProfile) { p.ExternalJournalLease = true },
			"release-held":         func(p *sourceRefusalObserverProfile) { p.ExternalReleaseLease = true },
			"missing-cut":          func(p *sourceRefusalObserverProfile) { p.MissingOriginalCut = true },
			"recovery-hold":        func(p *sourceRefusalObserverProfile) { p.RecoveryHold = &sourceRecoveryHoldProfile{} },
			"changed-mode":         func(p *sourceRefusalObserverProfile) { p.JournalMode = "0640" },
		} {
			changed, pending := p, *p.StartupPending
			changed.StartupPending = &pending
			change(&changed)
			if _, err := load(t, encode(changed)); err == nil {
				t.Fatal("mixed startup refusal purpose admitted", attempt, name)
			}
		}
	}
	for attempt := uint32(0); attempt < uint32(sourceAttemptLimit); attempt++ {
		p := profile
		p.Schema = "source-retirement-recovery-refusal-observer-v1"
		p.Arguments = append([]string(nil), profile.Arguments...)
		p.Arguments[7] = filepath.Join(sourceAttemptNumber(attempt).directory(root), "source-retirement-journal")
		p.RetirementPending = &sourcePendingGuardProfile{Attempt: attempt, GID: 500}
		admitted, err := load(t, encode(p))
		if err != nil {
			t.Fatal("exact retirement attempt refusal profile", attempt, err)
		}
		result, err := observeSourceProcess(program, root, "", "refusal-closed", nil, true, 2, nil, admitted)
		if err != nil || !result.InspectionOnly || result.RootPID != 0 || result.InstalledExecution || result.RefusalRetirementPendingObserved || result.RefusalRetirementPendingDevice != 0 || result.RefusalRetirementPendingInode != 0 || result.RefusalRetirementPendingSHA256 != "" {
			t.Fatal("offline retirement refusal inspection executed or claimed actual observation", attempt, result, err)
		}
		for name, change := range map[string]func(*sourceRefusalObserverProfile){
			"startup-purpose": func(p *sourceRefusalObserverProfile) {
				p.StartupPending = &sourcePendingGuardProfile{Attempt: attempt, GID: 500}
			},
			"base-purpose":    func(p *sourceRefusalObserverProfile) { p.Schema = "source-recovery-refusal-observer-v1" },
			"missing-purpose": func(p *sourceRefusalObserverProfile) { p.RetirementPending = nil },
			"foreign-attempt": func(p *sourceRefusalObserverProfile) {
				p.RetirementPending.Attempt = (attempt + 1) % uint32(sourceAttemptLimit)
			},
			"out-of-bound-attempt": func(p *sourceRefusalObserverProfile) { p.RetirementPending.Attempt = uint32(sourceAttemptLimit) },
			"root-group":           func(p *sourceRefusalObserverProfile) { p.RetirementPending.GID = 0 },
			"out-of-bound-group":   func(p *sourceRefusalObserverProfile) { p.RetirementPending.GID = 1000 },
			"journal-held":         func(p *sourceRefusalObserverProfile) { p.ExternalJournalLease = true },
			"release-held":         func(p *sourceRefusalObserverProfile) { p.ExternalReleaseLease = true },
			"missing-cut":          func(p *sourceRefusalObserverProfile) { p.MissingOriginalCut = true },
			"recovery-hold":        func(p *sourceRefusalObserverProfile) { p.RecoveryHold = &sourceRecoveryHoldProfile{} },
			"changed-mode":         func(p *sourceRefusalObserverProfile) { p.JournalMode = "0640" },
		} {
			changed, pending := p, *p.RetirementPending
			changed.RetirementPending = &pending
			change(&changed)
			if _, err := load(t, encode(changed)); err == nil {
				t.Fatal("mixed retirement refusal purpose admitted", attempt, name)
			}
		}
	}
	if _, err := os.Lstat(root); !os.IsNotExist(err) {
		t.Fatal("offline refusal inspection touched Source root", err)
	}
	encoded := encode(profile)
	for name, body := range map[string][]byte{
		"other-purpose":        bytes.Replace(encoded, []byte("source-recovery-refusal-observer-v1"), []byte("source-original-cut-observer-v1"), 1),
		"start":                bytes.Replace(encoded, []byte(`"recover-source"`), []byte(`"start-source"`), 1),
		"missing-journal-pin":  bytes.Replace(encoded, []byte(`"journal_sha256":"`+zero+`"`), []byte(`"journal_sha256":""`), 1),
		"unknown-journal-mode": bytes.Replace(encoded, []byte(`"journal_mode":"0600"`), []byte(`"journal_mode":"0666"`), 1),
		"foreign-journal":      bytes.Replace(encoded, []byte(".source-initial-journal"), []byte(".foreign-journal"), 1),
		"unknown":              bytes.Replace(encoded, []byte(`"schema":`), []byte(`"unexpected":true,"schema":`), 1),
		"duplicate":            bytes.Replace(encoded, []byte(`"schema":`), []byte(`"schema":"source-recovery-refusal-observer-v1","schema":`), 1),
		"trailing":             append(bytes.Clone(encoded), []byte("{}\n")...),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := load(t, body); err == nil {
				t.Fatal("ambiguous or out-of-scope refusal profile accepted")
			}
		})
	}
	for _, index := range []int{2, 3, 4, 5, 6, 7} {
		p := profile
		p.Arguments = append([]string(nil), profile.Arguments...)
		p.Arguments[index] = zero
		if index == 5 {
			p.Arguments[index] = "/misplaced-network"
		}
		if _, err := load(t, encode(p)); err == nil {
			t.Fatal("misplaced canonical refusal argv accepted", index)
		}
	}
}

func TestSourceRecoveryCancellationProfileInspectionNative(t *testing.T) {
	program := os.Getenv("ARDENTS_OBSERVER_INSPECT_PROGRAM")
	if !sourcePath(program) {
		t.Fatal("actual offline canonical ELF required")
	}
	raw, err := os.ReadFile(program)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "source")
	zero, generation := strings.Repeat("0", 64), strings.Repeat("1", 64)
	for _, operation := range []string{"initial", "replacement"} {
		t.Run(operation, func(t *testing.T) {
			journal, count := root+".source-initial-journal", 70
			if operation == "replacement" {
				journal, count = root+".source-replacement-"+generation+"-journal", 13
			}
			p := sourceRefusalObserverProfile{Schema: "source-recovery-cancellation-observer-v1", ProgramSHA256: sourcePhysicalSHA(raw), PublicReceiptSHA256: zero,
				Arguments: []string{"installation", "recover-source", filepath.Dir(program), "/testtmp/history", "alpha", "ardents-source-qualification", "/testtmp/request", journal}, JournalSHA256: zero, JournalMode: "0600",
				RecoveryHold: &sourceRecoveryHoldProfile{Phase: "file-ready", Path: filepath.Join(root, "generations", generation, ProgramName), BodySHA256: sourcePhysicalSHA(raw), Generation: generation, BindingSHA256: zero, PrefixRecords: count}}
			load := func(p sourceRefusalObserverProfile) (*sourceRefusalObserverProfile, error) {
				t.Helper()
				body, err := json.Marshal(p)
				if err != nil {
					t.Fatal(err)
				}
				body = append(body, '\n')
				path := filepath.Join(t.TempDir(), "cancellation.json")
				if err := os.WriteFile(path, body, 0600); err != nil {
					t.Fatal(err)
				}
				return readSourceRefusalObserverProfile(path, sourcePhysicalSHA(body), program, root)
			}
			admitted, err := load(p)
			if err != nil {
				t.Fatal(err)
			}
			result, err := observeSourceProcess(program, root, "", "refusal-closed", nil, true, 2, nil, admitted)
			if err != nil || !result.InspectionOnly || result.RecoveryHoldEntry == 0 || result.RecoveryHoldEntry == result.Entry || result.RecoveryHoldEntry == result.RefusalAdmissionEntry || result.RootPID != 0 || result.InstalledExecution || result.RecoveryHeldBeforeRecord || result.RefusalOwnershipReleasedBeforeExit {
				t.Fatal("offline canonical inspection executed a command or asserted an observation", result, err)
			}
			mutations := map[string]func(*sourceRefusalObserverProfile){
				"refusal-purpose":       func(p *sourceRefusalObserverProfile) { p.Schema = "source-recovery-refusal-observer-v1" },
				"missing-hold":          func(p *sourceRefusalObserverProfile) { p.RecoveryHold = nil },
				"foreign-phase":         func(p *sourceRefusalObserverProfile) { p.RecoveryHold.Phase = "file-born" },
				"foreign-path":          func(p *sourceRefusalObserverProfile) { p.RecoveryHold.Path = "/testtmp/foreign" },
				"nested-path":           func(p *sourceRefusalObserverProfile) { p.RecoveryHold.Path += "/child" },
				"foreign-body":          func(p *sourceRefusalObserverProfile) { p.RecoveryHold.BodySHA256 = zero },
				"foreign-generation":    func(p *sourceRefusalObserverProfile) { p.RecoveryHold.Generation = zero },
				"missing-binding":       func(p *sourceRefusalObserverProfile) { p.RecoveryHold.BindingSHA256 = "" },
				"other-frontier":        func(p *sourceRefusalObserverProfile) { p.RecoveryHold.PrefixRecords++ },
				"foreign-journal-lease": func(p *sourceRefusalObserverProfile) { p.ExternalJournalLease = true },
				"foreign-release-lease": func(p *sourceRefusalObserverProfile) { p.ExternalReleaseLease = true },
				"damaged-access":        func(p *sourceRefusalObserverProfile) { p.JournalMode = "0640" },
			}
			for name, mutate := range mutations {
				t.Run(name, func(t *testing.T) {
					changed, hold := p, *p.RecoveryHold
					changed.RecoveryHold = &hold
					mutate(&changed)
					if _, err := load(changed); err == nil {
						t.Fatal("out-of-purpose cancellation declaration accepted")
					}
				})
			}
		})
	}
	if _, err := os.Lstat(root); !os.IsNotExist(err) {
		t.Fatal("offline cancellation inspection touched the Source root", err)
	}
}

func TestSourceMissingCutProfileInspectionNative(t *testing.T) {
	program := os.Getenv("ARDENTS_OBSERVER_INSPECT_PROGRAM")
	if !sourcePath(program) {
		t.Fatal("actual offline canonical ELF required")
	}
	programRaw, err := os.ReadFile(program)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "uncreated-source")
	zero, generation := strings.Repeat("0", 64), strings.Repeat("1", 64)
	for _, journal := range []string{root + ".source-initial-journal", root + ".source-replacement-" + generation + "-journal"} {
		profile := sourceRefusalObserverProfile{Schema: "source-recovery-missing-cut-observer-v1", ProgramSHA256: sourcePhysicalSHA(programRaw), PublicReceiptSHA256: zero,
			Arguments: []string{"installation", "recover-source", filepath.Dir(program), "/testtmp/history", "alpha", "one-network", "/testtmp/request", journal}, JournalSHA256: zero, JournalMode: "0600", MissingOriginalCut: true}
		load := func(p sourceRefusalObserverProfile) (*sourceRefusalObserverProfile, error) {
			t.Helper()
			raw, err := json.Marshal(p)
			if err != nil {
				t.Fatal(err)
			}
			raw = append(raw, '\n')
			path := filepath.Join(t.TempDir(), "missing-cut.json")
			if err := os.WriteFile(path, raw, 0600); err != nil {
				t.Fatal(err)
			}
			return readSourceRefusalObserverProfile(path, sourcePhysicalSHA(raw), program, root)
		}
		admitted, err := load(profile)
		if err != nil {
			t.Fatal(err)
		}
		result, err := observeSourceProcess(program, root, "", "refusal-closed", nil, true, 2, nil, admitted)
		if err != nil || !result.InspectionOnly || result.RootPID != 0 || result.InstalledExecution || result.RefusalMissingCutObserved || result.RefusalMissingCutParentDevice != 0 || result.RefusalMissingCutParentInode != 0 || result.Entry == 0 || result.RefusalAdmissionEntry == 0 {
			t.Fatal("offline missing-cut inspection executed command or claimed kernel observation", result, err)
		}
		for name, change := range map[string]func(*sourceRefusalObserverProfile){
			"base-purpose":    func(p *sourceRefusalObserverProfile) { p.Schema = "source-recovery-refusal-observer-v1" },
			"cancel-purpose":  func(p *sourceRefusalObserverProfile) { p.Schema = "source-recovery-cancellation-observer-v1" },
			"missing-purpose": func(p *sourceRefusalObserverProfile) { p.MissingOriginalCut = false },
			"journal-held":    func(p *sourceRefusalObserverProfile) { p.ExternalJournalLease = true },
			"release-held":    func(p *sourceRefusalObserverProfile) { p.ExternalReleaseLease = true },
			"changed-mode":    func(p *sourceRefusalObserverProfile) { p.JournalMode = "0640" },
			"recovery-hold":   func(p *sourceRefusalObserverProfile) { p.RecoveryHold = &sourceRecoveryHoldProfile{} },
		} {
			t.Run(name, func(t *testing.T) {
				changed := profile
				change(&changed)
				if _, err := load(changed); err == nil {
					t.Fatal("mixed original-cut read purpose admitted")
				}
			})
		}
	}
	if _, err := os.Lstat(root); !os.IsNotExist(err) {
		t.Fatal("offline missing-cut inspection touched Source root", err)
	}
}
