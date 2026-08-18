//go:build live

package network_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFinalRunnerDispatchesMaintainedCellWorkers(t *testing.T) {
	cases := map[string]string{
		"profile/C1/00":                                   "TestBlockedEntryCommandsAcrossNamespaces",
		"profile/C4/04":                                   "TestBlockedEntryNegativeCommandsAcrossNamespaces",
		"capacity/h3-s5-b1-v1/0":                          "TestBlockedEntryFinalReferenceAndStrongCapacity",
		"sustained/endpoint-to-publisher/run-0":           "TestBlockedEntryFinalSustainedEvidence",
		"pressure/P0":                                     "TestBlockedEntryFinalProjectedAdmissionAndChurn",
		"pressure/P2":                                     "TestBlockedEntryReturnsFromExactRecoverableSocketPressure",
		"pressure/P3":                                     "TestBlockedEntryDrainsAtExactEmergencySocketPressure",
		"recovery/0":                                      "TestBlockedEntryRecoveryParentCommandsAcrossNamespaces",
		"hostile/G8-lifecycle/cancellation/0":             "TestBlockedEntryRecoveryParentCommandsAcrossNamespaces",
		"hostile/G1-invite/malformed/0":                   "TestBlockedEntryFinalHostileInviteValidation",
		"hostile/G2-domain-collision/responder/0":         "TestBlockedEntryFinalHostileDomainCollision",
		"hostile/G3-replay-replacement/active-reimport/0": "TestBlockedEntryFinalHostileReplay",
		"hostile/G3-replay-replacement/retired-replay/0":  "TestBlockedEntryFinalHostileReplay",
		"hostile/G3-replay-replacement/same-generation-different-bytes/0": "TestBlockedEntryFinalHostileReplay",
		"hostile/G3-replay-replacement/wrong-replacement-id/0":            "TestBlockedEntryFinalHostileReplay",
		"hostile/G3-replay-replacement/skipped-generation/0":              "TestBlockedEntryFinalHostileReplay",
		"hostile/G3-replay-replacement/third-generation/0":                "TestBlockedEntryFinalHostileReplay",
		"hostile/G3-replay-replacement/full-set/0":                        "TestBlockedEntryFinalHostileReplay",
		"hostile/G3-replay-replacement/cross-slot-replacement/0":          "TestBlockedEntryFinalHostileReplay",
		"hostile/G4-restart/after-regime-publication/0":                   "TestBlockedEntryFinalHostileRestart",
		"hostile/G4-restart/after-exposure-0/0":                           "TestBlockedEntryFinalHostileRestart",
		"hostile/G5-adapter-fault/accept-then-stall/0":                    "TestBlockedEntryNegativeCommandsAcrossNamespaces",
		"hostile/G8-lifecycle/endpoint-restart/0":                         "TestBlockedEntryFinalHostileRestart",
		"hostile/G6-substitution/network/0":                               "TestBlockedEntryFinalHostileInviteValidation",
		"hostile/G6-substitution/route-profile/0":                         "TestBlockedEntryFinalHostileInviteValidation",
		"hostile/G9-ledger-leakage/unknown-invite-field/0":                "TestBlockedEntryFinalHostileInviteValidation",
	}
	for cell, want := range cases {
		if got := finalWorkerTest(cell); got != want {
			t.Fatalf("worker for %s=%q want %q", cell, got, want)
		}
	}
	if got := finalWorkerTest("hostile/G4-restart/after-import/0"); got != "" {
		t.Fatalf("unimplemented later hostile group was silently dispatched to %q", got)
	}
}

func TestRecoveryWorkerSelectsOnlyItsTwoObservedCellFamilies(t *testing.T) {
	for _, test := range []struct {
		cell, wantCell, wantTerminal string
		selected                     bool
	}{
		{"recovery/2", "recovery/2", "abrupt connection loss", true},
		{"hostile/G8-lifecycle/cancellation/2", "hostile/G8-lifecycle/cancellation/2", "bridge-deadline-exceeded", true},
		{"hostile/G8-lifecycle/endpoint-restart/2", "", "", false},
	} {
		t.Run(test.cell, func(t *testing.T) {
			t.Setenv("ARDENTS_FINAL_CELL", test.cell)
			cell, terminal, selected := selectedRecoveryFinalCell(2)
			if cell != test.wantCell || terminal != test.wantTerminal || selected != test.selected {
				t.Fatalf("selection=(%q,%q,%t), want=(%q,%q,%t)",
					cell, terminal, selected, test.wantCell, test.wantTerminal, test.selected)
			}
		})
	}
}

func TestFinalArchiveCopyRejectsOverflow(t *testing.T) {
	var output bytes.Buffer
	written, overflow, err := copyFinalArchive(&output, strings.NewReader("123456"), 5)
	if err != nil || !overflow || written != 6 {
		t.Fatalf("archive copy=(%d,%v,%v)", written, overflow, err)
	}
}

func TestFinalRunnerSupplyLockBindsSchedule(t *testing.T) {
	value := finalSupplyFixture()
	lock := finalRunnerSupplyLock{Schema: "ardents-h3-s5-supply-lock-v1",
		GoBuilderImageID: value.GoBuilderImageID, GoBuilderVersion: value.GoBuilderVersion,
		GoArchiveSHA256: "708effb774be8237570d0add163225abbdfaf4fca28b2611df167beba4feef89",
		GoRecipeSHA256:  value.ProductReceipt.GoRecipeSHA256,
		GoModuleSHA256:  value.ProductReceipt.GoModuleSHA256,
		ToolImageID:     value.ToolImageID, ToolLockSHA256: value.ToolReceipt.ToolLockSHA256,
		CarrierSHA256: value.ToolReceipt.CarrierSHA256}
	raw, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	raw = append(raw, '\n')
	digest := sha256.Sum256(raw)
	value.SupplyLock = finalRunnerArtifact{Path: "runtime/supply.lock.json",
		SHA256: hex.EncodeToString(digest[:]), Bytes: int64(len(raw))}
	if err := validateFinalRunnerSupplyLock(raw, value); err != nil {
		t.Fatal(err)
	}
	value.ToolImageID = value.ProductImageID
	if err := validateFinalRunnerSupplyLock(raw, value); err == nil {
		t.Fatal("runner accepted a tool image outside the repository lock")
	}
}

func TestFinalRunnerObservationPreservesWorkerEvidence(t *testing.T) {
	seed := strings.Repeat("a", 64)
	plan := finalRunnerPlan{Schema: "ardents-h3-blocked-cell-plan-v1", CellID: "profile/C1/00", Seed: seed}
	worker := finalWorkerResult{CellID: plan.CellID, Terminal: "success", EvidenceComplete: true, StartedOffsetMillis: 4,
		TerminalOffsetMillis: 8, CleanupOffsetMillis: 11, ObserverSets: 1,
		Observers: fixtureFinalRunnerObservers(), Residuals: fixtureFinalRunnerResiduals()}
	if !validFinalRunnerPlan(plan) {
		t.Fatal("valid final plan rejected")
	}
	schedule := finalRunnerSchedule{CellOrder: []string{plan.CellID}, Seeds: []string{seed}}
	if !matchesFinalRunnerSchedule(schedule, 0, plan) || matchesFinalRunnerSchedule(schedule, 1, plan) {
		t.Fatal("runner did not enforce the exact schedule ordinal")
	}
	observed := finalObservationFromWorker(plan, worker)
	if observed.CellID != plan.CellID || observed.Seed != seed || observed.ObservedTerminal != "success" ||
		!observed.ProductStarted || !observed.FaultInjected || observed.FaultOwner != "none" ||
		observed.StartedOffsetMillis != 4 || observed.TerminalOffsetMillis != 8 ||
		observed.CleanupOffsetMillis != 11 || observed.AdapterCleanupMillis != 3 ||
		len(observed.Observers) != 9 || len(observed.Residuals) != 10 {
		t.Fatalf("runner observation=%+v", observed)
	}
	plan.Seed = strings.Repeat("A", 64)
	if validFinalRunnerPlan(plan) {
		t.Fatal("non-canonical seed accepted")
	}
	incomplete := worker
	incomplete.EvidenceComplete = false
	if validFinalWorkerResult(incomplete) {
		t.Fatal("worker without retained evidence was accepted")
	}
}

func TestFinalWorkerEvidenceRequiresRetainedObserversAndPostCleanupRoot(t *testing.T) {
	parent := t.TempDir()
	root, err := os.MkdirTemp(parent, "fixture-")
	if err != nil {
		t.Fatal(err)
	}
	writeFinalObserverRoot(t, root, []string{"endpoint"})
	observers, rawObservers := collectFinalWorkerObservers("profile/C1/00", []string{root})
	if len(observers) != 9 {
		t.Fatalf("retained observers=%d", len(observers))
	}
	value := []finalWorkerResult{{CellID: "profile/C1/00", TerminalOffsetMillis: 4,
		Observers: observers, RawObservers: rawObservers, ObserverSets: 1}}
	if err := releaseFinalWorkerRoot(parent); err == nil {
		t.Fatal("live worker root was accepted before cleanup")
	}
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	if err := releaseFinalWorkerRoot(parent); err != nil {
		t.Fatal(err)
	}
	completeFinalWorkerEvidence(value, 3, 4, 5)
	if !value[0].EvidenceComplete ||
		len(value[0].Residuals) != 10 || value[0].StartedOffsetMillis != 3 ||
		value[0].TerminalOffsetMillis != 4 || value[0].CleanupOffsetMillis != 5 {
		t.Fatalf("post-cleanup evidence=%+v", value[0])
	}
}

func TestFinalWorkerRootCleanupIsOwnershipScoped(t *testing.T) {
	secret := t.TempDir()
	t.Setenv("ARDENTS_BLOCKED_SECRET_ROOT", secret)
	root, err := prepareFinalWorkerRoot(strings.Repeat("a", 24))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "residual"), []byte("test"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := cleanupFinalWorkerRoot(root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("owned worker root remained: %v", err)
	}
	if err := cleanupFinalWorkerRoot(filepath.Join(secret, "outside")); err == nil {
		t.Fatal("cleanup accepted an unowned path")
	}
}

func TestFinalWorkerEvidenceRejectsMissingCapacityUnitAndLaterBatch(t *testing.T) {
	reference := filepath.Join(t.TempDir(), "reference")
	writeFinalObserverRoot(t, reference, []string{"capacity-00", "capacity-01", "capacity-02", "capacity-03"})
	observers, _ := collectFinalWorkerObservers("capacity/h3-s5-b1-v1/0", []string{reference})
	if len(observers) != 9 {
		t.Fatal("complete four-unit capacity observers were rejected")
	}
	exact := filepath.Join(reference, "sync", "capacity-03")
	substitute := filepath.Join(reference, "sync", "capacity-copy")
	if err := os.Rename(exact, substitute); err != nil {
		t.Fatal(err)
	}
	if observers, _ := collectFinalWorkerObservers("capacity/h3-s5-b1-v1/0", []string{reference}); observers != nil {
		t.Fatal("capacity cell accepted a substituted Endpoint identity")
	}
	if err := os.Rename(substitute, exact); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(reference, "sync", "capacity-03", "result.json")); err != nil {
		t.Fatal(err)
	}
	if observers, _ := collectFinalWorkerObservers("capacity/h3-s5-b1-v1/0", []string{reference}); observers != nil {
		t.Fatal("capacity cell accepted a missing Endpoint observer")
	}
	first, second := filepath.Join(t.TempDir(), "batch-0"), filepath.Join(t.TempDir(), "batch-1")
	for _, root := range []string{first, second} {
		writeFinalObserverRoot(t, root, []string{"capacity-00", "capacity-01", "capacity-02", "capacity-03"})
	}
	if err := os.Remove(filepath.Join(second, "sync", "bridge", "result.json")); err != nil {
		t.Fatal(err)
	}
	if observers, _ := collectFinalWorkerObservers("pressure/P4", []string{first, second}); observers != nil {
		t.Fatal("P4 accepted observer loss in a later batch")
	}
	boundaryRoot := filepath.Join(t.TempDir(), "boundary")
	writeFinalObserverRoot(t, boundaryRoot, []string{"endpoint"})
	dnsPath := filepath.Join(boundaryRoot, "sync", "bridge", "result.json")
	var dns finalDNSObservation
	raw, err := os.ReadFile(dnsPath)
	if err != nil || json.Unmarshal(raw, &dns) != nil {
		t.Fatal("read boundary-control fixture")
	}
	delete(dns.BoundaryControls, "B-to-Initiator")
	raw, _ = json.Marshal(dns)
	if err := os.WriteFile(dnsPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if observers, _ := collectFinalWorkerObservers("profile/C1/00", []string{boundaryRoot}); observers != nil {
		t.Fatal("worker accepted a missing boundary-specific DNS control")
	}
}

func writeFinalObserverRoot(t *testing.T, root string, endpoints []string) {
	t.Helper()
	flows := map[string]int64{"E-to-B-front": 1, "B-to-Initiator": 1,
		"Responder-to-Publisher": 1, "Initiator-to-Introduction": 1,
		"Introduction-to-Rendezvous": 1, "Rendezvous-to-Responder": 1}
	roles := append(append([]string(nil), endpoints...), "bridge", "publisher", "initiator", "introduction",
		"rendezvous", "responder")
	for _, role := range roles {
		directory := filepath.Join(root, "sync", role)
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
		phaseRole := role
		if strings.HasPrefix(role, "capacity-") {
			phaseRole = "endpoint"
		}
		pathRaw, _ := json.Marshal(finalPathObservation{Phase: "s5.3-" + phaseRole, Counts: flows,
			DynamicBindings: map[string]finalTarget{"front-to-WebTunnel-server": {Address: "127.0.0.1", Port: 1}},
			Packets:         1, Passed: true})
		controls := make(map[string]finalDNSControl, len(flows)+1)
		for flow := range flows {
			controls[flow] = finalDNSControl{IPv4UDP: 2, IPv6UDP: 2, IPv4TCP: 2, IfIndex: 1,
				Token: strings.Repeat("a", 32)}
		}
		controls["front-to-WebTunnel-server"] = finalDNSControl{
			IPv4UDP: 2, IPv6UDP: 2, IPv4TCP: 2, IfIndex: 1, Token: strings.Repeat("a", 32),
		}
		count := int64(len(controls))
		dnsRaw, _ := json.Marshal(finalDNSObservation{Controls: 6 * count,
			IPv4UDPControls: 2 * count, IPv6UDPControls: 2 * count, IPv4TCPControls: 2 * count,
			BoundaryControls: controls})
		if err := os.WriteFile(filepath.Join(directory, "path-result.json"), pathRaw, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(directory, "result.json"), dnsRaw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestFinalRunnerSupplyIdentityRequiresDistinctContentAddresses(t *testing.T) {
	value := finalSupplyFixture()
	if !validFinalRunnerSupplyIdentity(value) {
		t.Fatal("valid frozen supply identity rejected")
	}
	value.ToolImageID = value.ProductImageID
	if validFinalRunnerSupplyIdentity(value) {
		t.Fatal("one image was accepted for distinct product and observer responsibilities")
	}
	value.ToolImageID = "sha256:" + strings.Repeat("d", 64)
	value.GoBuilderImageID = value.ToolImageID
	if validFinalRunnerSupplyIdentity(value) {
		t.Fatal("observer tooling image was accepted as the final Go builder")
	}
}

func finalSupplyFixture() finalRunnerSchedule {
	raw := json.RawMessage(`{}`)
	return finalRunnerSchedule{Schema: "ardents-h3-s5-final-spec-v1",
		RepositoryCommit: strings.Repeat("a", 40), SourceSHA256: strings.Repeat("b", 64),
		LinuxImage: finalRunnerLinuxImage, ImageSHA256: strings.TrimPrefix(finalRunnerLinuxImage, "ubuntu@sha256:"),
		ProductImageID: "sha256:" + strings.Repeat("c", 64), ToolImageID: "sha256:" + strings.Repeat("d", 64),
		GoBuilderImageID: "sha256:" + strings.Repeat("e", 64),
		GoBuilderVersion: "go version go1.26.6 linux/amd64",
		SupplyLock: finalRunnerArtifact{Path: "runtime/supply.lock.json",
			SHA256: strings.Repeat("0", 64), Bytes: 1},
		RuntimeCompose: finalRunnerArtifact{Path: "runtime/blocked-entry.compose.yaml", SHA256: strings.Repeat("1", 64), Bytes: 1},
		ProductReceipt: finalRunnerProductReceipt{SourceSHA256: strings.Repeat("b", 64),
			GoArchiveSHA256: "708effb774be8237570d0add163225abbdfaf4fca28b2611df167beba4feef89",
			GoRecipeSHA256:  strings.Repeat("f", 64),
			GoModuleSHA256:  strings.Repeat("e", 64),
			RouteSHA256:     strings.Repeat("1", 64), BridgeSHA256: strings.Repeat("2", 64),
			ServiceSHA256: strings.Repeat("3", 64), StreamSHA256: strings.Repeat("4", 64),
			PublishSHA256: strings.Repeat("5", 64), NetworkSHA256: strings.Repeat("6", 64),
			AdapterSHA256: strings.Repeat("7", 64)},
		ToolReceipt: finalRunnerToolReceipt{BaseDigest: strings.TrimPrefix(finalRunnerLinuxImage, "ubuntu@"),
			ToolLockSHA256: strings.Repeat("8", 64), SourceSHA256: strings.Repeat("9", 64),
			CarrierSHA256: strings.Repeat("a", 64)},
		Kernel: "6.8.0", ClientSHA256: strings.Repeat("e", 64), ServerSHA256: strings.Repeat("f", 64),
		Endpoint: raw, ReferenceBridge: raw, StrongerBridge: raw, Collector: raw, Network: raw, Clocks: raw,
		Configurations: raw}
}

func TestFinalWorkerCaptureDrainsButRejectsOversizedOutput(t *testing.T) {
	var capture finalWorkerCapture
	value := make([]byte, maximumFinalWorkerStream+1)
	written, err := capture.Write(value)
	if err != nil || written != len(value) || !capture.overflow || capture.buffer.Len() != maximumFinalWorkerStream {
		t.Fatalf("bounded capture=(written=%d overflow=%v bytes=%d err=%v)",
			written, capture.overflow, capture.buffer.Len(), err)
	}
}

func TestFinalWorkerEnvironmentReplacesFrozenInputs(t *testing.T) {
	t.Setenv("ARDENTS_FINAL_CELL", "stale")
	environment := finalWorkerEnvironment(map[string]string{"ARDENTS_FINAL_CELL": "profile/C1/00"})
	count := 0
	for _, value := range environment {
		if strings.HasPrefix(value, "ARDENTS_FINAL_CELL=") {
			count++
			if value != "ARDENTS_FINAL_CELL=profile/C1/00" {
				t.Fatalf("worker retained mutable cell input %q", value)
			}
		}
	}
	if count != 1 {
		t.Fatalf("worker cell environment count=%d", count)
	}
}

func TestFinalSupplyCommandRejectsOversizedOutput(t *testing.T) {
	t.Setenv("ARDENTS_FINAL_COMMAND_FIXTURE", "oversized")
	if _, err := finalSupplyOutput(os.Args[0], "-test.run", "^TestFinalCommandFixture$"); err == nil {
		t.Fatal("oversized final supply output was accepted")
	}
}

func TestFinalBoundedProcessStopsHungCommand(t *testing.T) {
	t.Setenv("ARDENTS_FINAL_COMMAND_FIXTURE", "hang")
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	command := exec.CommandContext(ctx, os.Args[0], "-test.run", "^TestFinalCommandFixture$")
	command.Env = os.Environ()
	started := time.Now()
	_, _, err := runFinalBoundedProcess(command, 1<<20)
	if err == nil || time.Since(started) > 7*time.Second {
		t.Fatalf("hung final command result=%v elapsed=%s", err, time.Since(started))
	}
}

func TestFinalWorkerTerminalUsesParentReceiptClock(t *testing.T) {
	t.Setenv("ARDENTS_FINAL_COMMAND_FIXTURE", "terminal-delay")
	command := exec.CommandContext(context.Background(), os.Args[0], "-test.run", "^TestFinalCommandFixture$")
	command.Env = os.Environ()
	origin := time.Now()
	_, _, receipt, err := runFinalBoundedWorkerProcess(command, 1<<20, origin, "profile/C1/00")
	elapsed := time.Since(origin)
	if err != nil || receipt.At < 0 || receipt.At >= elapsed-100*time.Millisecond ||
		receipt.CellID != "profile/C1/00" || receipt.Terminal != "success" {
		t.Fatalf("terminal receipt=%+v elapsed=%s err=%v", receipt, elapsed, err)
	}
}

func TestFinalWorkerTerminalRejectsDuplicateAndMismatchedMarkers(t *testing.T) {
	for _, mode := range []string{"terminal-duplicate", "terminal-wrong-cell", "result-before-terminal",
		"result-unknown", "result-duplicate-key", "result-noncanonical", "result-malformed-extra"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("ARDENTS_FINAL_COMMAND_FIXTURE", mode)
			command := exec.CommandContext(context.Background(), os.Args[0], "-test.run", "^TestFinalCommandFixture$")
			command.Env = os.Environ()
			if _, _, _, err := runFinalBoundedWorkerProcess(command, 1<<20, time.Now(), "profile/C1/00"); err == nil {
				t.Fatal("invalid terminal marker was accepted")
			}
		})
	}
}

func TestFinalCommandFixture(t *testing.T) {
	switch os.Getenv("ARDENTS_FINAL_COMMAND_FIXTURE") {
	case "oversized":
		_, _ = os.Stdout.Write(make([]byte, (1<<20)+1))
	case "hang":
		time.Sleep(time.Hour)
	case "terminal-delay":
		writeFinalCommandTerminal("profile/C1/00")
		time.Sleep(200 * time.Millisecond)
		writeFinalCommandResult("")
	case "terminal-duplicate":
		for range 2 {
			writeFinalCommandTerminal("profile/C1/00")
		}
		writeFinalCommandResult("")
	case "terminal-wrong-cell":
		writeFinalCommandTerminal("profile/C1/01")
		writeFinalCommandResult("")
	case "result-before-terminal":
		writeFinalCommandResult("")
		writeFinalCommandTerminal("profile/C1/00")
	case "result-unknown", "result-duplicate-key", "result-noncanonical":
		writeFinalCommandTerminal("profile/C1/00")
		writeFinalCommandResult(os.Getenv("ARDENTS_FINAL_COMMAND_FIXTURE"))
	case "result-malformed-extra":
		writeFinalCommandTerminal("profile/C1/00")
		writeFinalCommandResult("")
		_, _ = os.Stdout.WriteString("{\"schema\":\"ardents-h3-final-worker-cell-v1\"\n")
	}
}

func writeFinalCommandTerminal(cell string) {
	value := struct {
		Schema   string `json:"schema"`
		CellID   string `json:"cell_id"`
		Terminal string `json:"terminal"`
	}{Schema: "ardents-h3-final-worker-terminal-v1", CellID: cell, Terminal: "success"}
	raw, _ := json.Marshal(value)
	_, _ = os.Stdout.Write(append(raw, '\n'))
}

func writeFinalCommandResult(mutation string) {
	raw, _ := json.Marshal(finalWorkerResult{Schema: "ardents-h3-final-worker-cell-v1",
		CellID: "profile/C1/00", Terminal: "success"})
	switch mutation {
	case "result-unknown":
		raw = append(raw[:len(raw)-1], []byte(",\"unknown\":true}")...)
	case "result-duplicate-key":
		raw = append([]byte("{\"schema\":\"ardents-h3-final-worker-cell-v1\","), raw[1:]...)
	case "result-noncanonical":
		raw = append([]byte(" "), raw...)
	}
	_, _ = os.Stdout.Write(append(raw, '\n'))
}

func fixtureFinalRunnerObservers() []finalRunnerObserver {
	boundaries := []string{"endpoint-adapter", "tls-front", "webtunnel-server", "bridge-next-leg",
		"publisher-endpoint", "ordinary-initiator", "ordinary-introduction", "ordinary-rendezvous",
		"ordinary-responder"}
	result := make([]finalRunnerObserver, 0, len(boundaries))
	for _, boundary := range boundaries {
		result = append(result, finalRunnerObserver{Boundary: boundary, IPv4UDPControl: true,
			IPv6UDPControl: true, IPv4TCPControl: true, Attribution: "exact", ForbiddenOwner: "none",
			ObservationCompleted: true})
	}
	return result
}

func fixtureFinalRunnerResiduals() []finalRunnerResidual {
	kinds := []string{"process", "listener", "socket", "namespace", "mount", "file", "queue", "timer",
		"cgroup", "publishable-secret"}
	result := make([]finalRunnerResidual, 0, len(kinds))
	for _, kind := range kinds {
		result = append(result, finalRunnerResidual{Kind: kind, Owner: "none"})
	}
	return result
}
