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
	"strings"
	"testing"
	"time"
)

func TestFinalRunnerDispatchesMaintainedCellWorkers(t *testing.T) {
	cases := map[string]string{
		"profile/C1/00":                         "TestBlockedEntryCommandsAcrossNamespaces",
		"profile/C4/04":                         "TestBlockedEntryNegativeCommandsAcrossNamespaces",
		"capacity/h3-s5-b1-v1/0":                "TestBlockedEntryFinalReferenceAndStrongCapacity",
		"sustained/endpoint-to-publisher/run-0": "TestBlockedEntryFinalSustainedEvidence",
		"pressure/P0":                           "TestBlockedEntryFinalProjectedAdmissionAndChurn",
		"pressure/P2":                           "TestBlockedEntryReturnsFromExactRecoverableSocketPressure",
		"pressure/P3":                           "TestBlockedEntryDrainsAtExactEmergencySocketPressure",
		"recovery/0":                            "TestBlockedEntryRecoveryParentCommandsAcrossNamespaces",
	}
	for cell, want := range cases {
		if got := finalWorkerTest(cell); got != want {
			t.Fatalf("worker for %s=%q want %q", cell, got, want)
		}
	}
	if got := finalWorkerTest("hostile/G1-invite/malformed/0"); got != "" {
		t.Fatalf("unimplemented hostile worker was silently dispatched to %q", got)
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
		TerminalOffsetMillis: 8, CleanupOffsetMillis: 11,
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

func TestFinalCommandFixture(t *testing.T) {
	switch os.Getenv("ARDENTS_FINAL_COMMAND_FIXTURE") {
	case "oversized":
		_, _ = os.Stdout.Write(make([]byte, (1<<20)+1))
	case "hang":
		time.Sleep(time.Hour)
	}
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
