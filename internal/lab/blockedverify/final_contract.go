package blockedverify

import (
	"fmt"
	"reflect"
	"strings"
)

var requiredFinalConfigurations = []string{
	"configuration/topology.json",
	"configuration/cgroups.json",
	"configuration/network.json",
	"configuration/workloads.json",
	"configuration/invites.sha256",
	"configuration/route-credentials.sha256",
	"configuration/observers.json",
}

func verifyFinalCampaign(spec *finalSpec, summary *finalSummary) ([]string, []string) {
	if spec == nil || summary == nil {
		return []string{"final campaign specification or summary is missing"}, nil
	}
	invalid := verifyFinalSpec(*spec)
	if len(invalid) > 0 {
		return invalid, nil
	}
	if summary.Schema != "ardents-h3-s5-final-summary-v1" || summary.ImageHash != spec.ImageSHA256 ||
		summary.ClientHash != spec.ClientSHA256 || summary.ServerHash != spec.ServerSHA256 {
		invalid = append(invalid, "final summary identity or supply binding is invalid")
	}
	invalid = append(invalid, verifyFinalCells(*spec, summary.Cells)...)
	if !reflect.DeepEqual(summary.Profiles, profileResultsFromCells(summary.Cells)) {
		invalid = append(invalid, "final profile aggregate does not reproduce the observed cells")
	}
	invalid = append(invalid, verifyFinalHosts(summary.Hosts)...)
	profileInvalid, profileFailures := verifyFinalProfiles(summary.Profiles)
	capacityInvalid, capacityFailures := verifyFinalCapacity(summary.Capacity)
	sustainedInvalid, sustainedFailures := verifyFinalSustained(summary.Sustained)
	pressureInvalid, pressureFailures := verifyFinalPressure(summary.Pressure)
	recoveryInvalid, recoveryFailures := verifyFinalRecovery(summary.Recovery)
	invalid = append(invalid, profileInvalid...)
	invalid = append(invalid, capacityInvalid...)
	invalid = append(invalid, sustainedInvalid...)
	invalid = append(invalid, pressureInvalid...)
	invalid = append(invalid, recoveryInvalid...)
	failures := append(profileFailures, capacityFailures...)
	failures = append(failures, sustainedFailures...)
	failures = append(failures, pressureFailures...)
	failures = append(failures, recoveryFailures...)
	return unique(invalid), unique(failures)
}

func profileResultsFromCells(cells []finalCellObservation) []finalProfileResult {
	ids := []string{"C0", "C1", "C2", "C3", "C4", "C5", "C6"}
	terminals := []string{"success", "success", "success", "bridge-attempt-exhausted",
		"bridge-attempt-exhausted", "probe-contained", "limitation-recorded"}
	result := make([]finalProfileResult, 0, len(ids))
	for index, id := range ids {
		value := finalProfileResult{ID: id, Terminal: terminals[index]}
		for _, cell := range cells {
			if strings.HasPrefix(cell.ID, "profile/"+id+"/") {
				value.Attempts++
				if cell.Terminal == terminals[index] {
					value.Successful++
				}
			}
		}
		result = append(result, value)
	}
	return result
}

func verifyFinalCells(spec finalSpec, observed []finalCellObservation) []string {
	if len(observed) != len(spec.CellOrder) {
		return []string{"final cell observation inventory is incomplete"}
	}
	var reasons []string
	var previousCleanup uint64
	for index, cell := range observed {
		if cell.ID != spec.CellOrder[index] || cell.Seed != spec.Seeds[index] {
			reasons = append(reasons, "final cell identity, order, or seed binding is invalid")
			break
		}
		if expected, fixed := expectedFinalTerminal(cell.ID); fixed && cell.Terminal != expected {
			reasons = append(reasons, "final cell terminal differs from its frozen schedule")
			break
		}
		if cell.TerminalOffsetMillis < cell.StartedOffsetMillis ||
			cell.CleanupOffsetMillis < cell.TerminalOffsetMillis ||
			cell.CleanupOffsetMillis-cell.TerminalOffsetMillis > uint64(spec.Clocks.CellCleanupMillis) ||
			index > 0 && cell.StartedOffsetMillis < previousCleanup {
			reasons = append(reasons, "final cell monotonic interval or cleanup bound is invalid")
			break
		}
		previousCleanup = cell.CleanupOffsetMillis
	}
	return reasons
}

func expectedFinalTerminal(identity string) (string, bool) {
	for profile, terminal := range map[string]string{"C0": "success", "C1": "success", "C2": "success",
		"C3": "bridge-attempt-exhausted", "C4": "bridge-attempt-exhausted", "C5": "probe-contained",
		"C6": "limitation-recorded"} {
		if strings.HasPrefix(identity, "profile/"+profile+"/") {
			return terminal, true
		}
	}
	if strings.HasPrefix(identity, "capacity/") || strings.Contains(identity, "/direct-") ||
		strings.Contains(identity, "/run-") {
		return "complete", true
	}
	for pressure, terminal := range map[string]string{"P0": "normal", "P1": "normal", "P2": "normal",
		"P3": "drain", "P4": "normal"} {
		if identity == "pressure/"+pressure {
			return terminal, true
		}
	}
	if strings.HasPrefix(identity, "recovery/") {
		return "abrupt connection loss", true
	}
	return "", false
}

func verifyHostileCellBindings(events []event, cells []finalCellObservation) []string {
	if len(cells) < len(events) {
		return []string{"hostile events are not covered by final cell observations"}
	}
	offset := len(cells) - len(events)
	for index, observed := range events {
		cell := cells[offset+index]
		if cell.ID != "hostile/"+observed.ID || cell.StartedOffsetMillis != observed.StartedOffsetMillis ||
			cell.TerminalOffsetMillis != observed.TerminalOffsetMillis ||
			cell.CleanupOffsetMillis != observed.CleanupOffsetMillis {
			return []string{"hostile event evidence differs from its final cell observation"}
		}
	}
	return nil
}

func verifyFinalSpec(value finalSpec) []string {
	var reasons []string
	if value.Schema != "ardents-h3-s5-final-spec-v1" || !isHexDigest(value.RepositoryCommit, 20) ||
		!isHexDigest(value.SourceSHA256, 32) || value.LinuxImage != acceptedFinalLinuxImage ||
		value.ImageSHA256 != acceptedFinalImageHash || value.Kernel == "" ||
		!isImageID(value.ProductImageID) || !isImageID(value.ToolImageID) ||
		!isImageID(value.GoBuilderImageID) || value.ProductImageID == value.ToolImageID ||
		value.ProductImageID == value.GoBuilderImageID || value.ToolImageID == value.GoBuilderImageID ||
		value.GoBuilderVersion != "go version go1.26.6 linux/amd64" ||
		value.SupplyLock.Path != "runtime/supply.lock.json" ||
		!isHexDigest(value.SupplyLock.SHA256, 32) || value.SupplyLock.Bytes < 1 ||
		value.RuntimeCompose.Path != "runtime/blocked-entry.compose.yaml" ||
		!isHexDigest(value.RuntimeCompose.SHA256, 32) || value.RuntimeCompose.Bytes < 1 ||
		!validFinalProductReceipt(value.ProductReceipt, value.SourceSHA256) ||
		!validFinalToolReceipt(value.ToolReceipt) ||
		value.ClientSHA256 != acceptedFinalClientHash || value.ServerSHA256 != acceptedFinalServerHash {
		reasons = append(reasons, "final campaign source or supply identity is incomplete")
	}
	for name, got := range map[string]finalHostClass{"endpoint": value.Endpoint,
		"reference bridge": value.ReferenceBridge, "stronger bridge": value.StrongerBridge,
		"collector": value.Collector} {
		if reason := verifyHostClass(name, got); reason != "" {
			reasons = append(reasons, reason)
		}
	}
	wantedNetwork := finalNetwork{BaseRTTMillis: 80, LossPPM: 1_000, JitterP95Millis: 10}
	if value.Network != wantedNetwork {
		reasons = append(reasons, "final campaign network envelope differs from R-037")
	}
	wantedClocks := finalClocks{OrdinaryBlockedMillis: 3_000, TransitionMillis: 2_000,
		AttemptMillis: 64_000, ContactMillis: 15_000, StartupMillis: 5_000,
		InterContactMillis: 1_000, AdapterCleanupMillis: 6_000, CellCleanupMillis: 15_000}
	if value.Clocks != wantedClocks {
		reasons = append(reasons, "final campaign clocks differ from R-037")
	}
	if !reflect.DeepEqual(value.CellOrder, requiredFinalCellOrder()) {
		reasons = append(reasons, "final campaign cell order is incomplete or reordered")
	}
	if len(value.Seeds) != len(value.CellOrder) {
		reasons = append(reasons, "final campaign seeds do not cover every scheduled cell")
	} else {
		seen := make(map[string]bool, len(value.Seeds))
		for _, seed := range value.Seeds {
			if !isHexDigest(seed, 32) || seen[seed] {
				reasons = append(reasons, "final campaign seed is invalid or reused")
				break
			}
			seen[seed] = true
		}
	}
	if reason := verifyFinalConfigurations(value.Configurations); reason != "" {
		reasons = append(reasons, reason)
	}
	return reasons
}

func isImageID(value string) bool {
	return strings.HasPrefix(value, "sha256:") && isHexDigest(strings.TrimPrefix(value, "sha256:"), 32)
}

func validFinalProductReceipt(value finalProductReceipt, source string) bool {
	return value.SourceSHA256 == source && isHexDigest(value.GoArchiveSHA256, 32) &&
		isHexDigest(value.GoRecipeSHA256, 32) && isHexDigest(value.GoModuleSHA256, 32) &&
		isHexDigest(value.RouteSHA256, 32) &&
		isHexDigest(value.BridgeSHA256, 32) && isHexDigest(value.ServiceSHA256, 32) &&
		isHexDigest(value.StreamSHA256, 32) && isHexDigest(value.PublishSHA256, 32) &&
		isHexDigest(value.NetworkSHA256, 32) && isHexDigest(value.AdapterSHA256, 32)
}

func validFinalToolReceipt(value finalToolReceipt) bool {
	return value.BaseDigest == acceptedFinalImageHash && isHexDigest(value.ToolLockSHA256, 32) &&
		isHexDigest(value.SourceSHA256, 32) && isHexDigest(value.CarrierSHA256, 32)
}

func requiredFinalCellOrder() []string {
	var result []string
	floors := []struct {
		id    string
		count int
	}{{"C0", 20}, {"C1", 20}, {"C2", 20}, {"C3", 5}, {"C4", 5}, {"C5", 20}, {"C6", 20}}
	for _, profile := range floors {
		for episode := range profile.count {
			result = append(result, fmt.Sprintf("profile/%s/%02d", profile.id, episode))
		}
	}
	for _, profile := range []string{"h3-s5-b1-v1", "h3-s5-b1-v1-strong"} {
		for batch := range 5 {
			result = append(result, fmt.Sprintf("capacity/%s/%d", profile, batch))
		}
	}
	for _, direction := range []string{"endpoint-to-publisher", "publisher-to-endpoint"} {
		result = append(result, "sustained/"+direction+"/direct-before")
		for run := range 5 {
			result = append(result, fmt.Sprintf("sustained/%s/run-%d", direction, run))
		}
		result = append(result, "sustained/"+direction+"/direct-after")
	}
	for cell := range 5 {
		result = append(result, fmt.Sprintf("pressure/P%d", cell))
	}
	for episode := range 5 {
		result = append(result, fmt.Sprintf("recovery/%d", episode))
	}
	for _, identity := range expectedEventSequence() {
		result = append(result, "hostile/"+identity.id)
	}
	return result
}
