package blockedverify

import (
	"fmt"
	"reflect"
	"strings"
)

const (
	acceptedFinalLinuxImage = "ubuntu@sha256:7b202b0e2e0028c6250f5fcf41d04df492d145a1654c6995a6553f0c1f6f1960"
	acceptedFinalImageHash  = "7b202b0e2e0028c6250f5fcf41d04df492d145a1654c6995a6553f0c1f6f1960"
	acceptedFinalClientHash = "de581c8dd36193bb4168aee840406294af406bf8187817c10ac2bcd9464fd120"
	acceptedFinalServerHash = "5fe32f8ab736ed54fc66027775761084e68f0e1ec9b5fea7c3417c6617255336"
)

var acceptedFinalConfigurationHashes = map[string]string{
	"configuration/topology.json":  "ad8019e0855a637ec0aa0a376aac590e87c34d4c56fc8a305e044e4e874133a6",
	"configuration/cgroups.json":   "577cdf546310d74a68f9b8efc46de5986b9be1bead0cb26949a9f271ffab4c2d",
	"configuration/network.json":   "ffba48ee8fcd1be3bfee8339a0719a601684629eafb1a262ef2ec00ec1372b5d",
	"configuration/workloads.json": "07d0f2a48adc7481ad4edb7977ddaadeed54618de3d394d072c94f3b867e23de",
	"configuration/observers.json": "30529fdc6774d484e8d4a9aef16e6ea1080ff632e3d835ed878d4404eb5b4c19",
}

func verifyHostClass(name string, got finalHostClass) string {
	common := func(id string, vcpu uint16, memory uint32, down, up uint16) finalHostClass {
		return finalHostClass{ID: id, OperatingSystem: "ubuntu-lts", Architecture: "x86-64",
			StorageClass: "ssd", Dedicated: true, VCPU: vcpu, MemoryMiB: memory,
			LinkDownMbit: down, LinkUpMbit: up}
	}
	var want finalHostClass
	switch name {
	case "endpoint":
		want = common("endpoint-reference", 4, 8_192, 100, 20)
		want.CPUMeanCores, want.CPUP95Cores, want.MemoryP95MiB = .5, 1, 512
	case "reference bridge":
		want = common("h3-s5-b1-v1", 2, 2_048, 100, 100)
		want.CPUMaxCores, want.CPUMeanCores, want.CPUP95Cores = 1.6, 1.12, 1.28
		want.MemoryMaxMiB, want.MemoryP95MiB = 1_280, 896
		want.HelperRSSP95MiB, want.HelperFDs, want.HelperSockets, want.MinimumReservePC = 128, 64, 32, 20
	case "stronger bridge":
		want = common("h3-s5-b1-v1-strong", 8, 8_192, 400, 400)
		want.CPUMaxCores, want.CPUMeanCores, want.CPUP95Cores = 6.4, 4.48, 5.12
		want.MemoryMaxMiB, want.MemoryP95MiB = 5_120, 3_584
		want.HelperRSSP95MiB, want.HelperFDs, want.HelperSockets, want.MinimumReservePC = 512, 256, 128, 20
	case "collector":
		want = common("h3-s5-collector-v1", 16, 32_768, 1_000, 1_000)
	}
	if !reflect.DeepEqual(got, want) {
		return "final " + name + " class differs from R-037"
	}
	return ""
}

func verifyFinalConfigurations(values []artifactCommitment) string {
	if len(values) != len(requiredFinalConfigurations) {
		return "final campaign configuration commitments are incomplete"
	}
	for index, path := range requiredFinalConfigurations {
		value := values[index]
		if value.Path != path || !isHexDigest(value.SHA256, 32) || value.Bytes <= 0 {
			return "final campaign configuration commitments are invalid or reordered"
		}
		if expected := acceptedFinalConfigurationHashes[path]; expected != "" && value.SHA256 != expected {
			return "final public campaign configuration differs from its accepted template"
		}
	}
	return ""
}

func verifyFinalHosts(values []finalObservedHost) []string {
	if len(values) == 0 {
		return []string{"final campaign host allocation is missing"}
	}
	seen := make(map[string]bool, len(values))
	observed := make(map[string]finalObservedAllocation)
	processNamespaces, networkNamespaces := make(map[string]bool), make(map[string]bool)
	for _, value := range values {
		var hostCPU uint16
		var hostMemory uint32
		if value.ID == "" || seen[value.ID] || !value.DedicatedThreads || !value.CgroupV2 ||
			value.SwapEvents != 0 || value.AllocatedVCPU > value.LogicalCPUs ||
			value.AllocatedMemoryMiB > value.MemoryMiB {
			return []string{"final campaign host allocation is ambiguous or overcommitted"}
		}
		seen[value.ID] = true
		for _, allocation := range value.Allocations {
			if allocation.ID == "" || observed[allocation.ID].ID != "" || allocation.ProcessNamespace == "" ||
				allocation.NetworkNamespace == "" || processNamespaces[allocation.ProcessNamespace] ||
				networkNamespaces[allocation.NetworkNamespace] ||
				strings.HasPrefix(allocation.ProcessNamespace, "allocation:") ||
				strings.HasPrefix(allocation.NetworkNamespace, "allocation:") {
				return []string{"final campaign role allocation or namespace identity is ambiguous"}
			}
			observed[allocation.ID] = allocation
			processNamespaces[allocation.ProcessNamespace] = true
			networkNamespaces[allocation.NetworkNamespace] = true
			hostCPU += allocation.VCPU
			hostMemory += allocation.MemoryMiB
		}
		if hostCPU != value.AllocatedVCPU || hostMemory != value.AllocatedMemoryMiB {
			return []string{"final campaign host allocation does not reconcile to its roles"}
		}
		if !validFinalRuntimeHost(value) {
			return []string{"final campaign runtime host attestation is missing or invalid"}
		}
	}
	wanted := expectedFinalAllocations()
	if len(observed) != len(wanted) {
		return []string{"final campaign exact role allocation set is incomplete"}
	}
	for id, expected := range wanted {
		got := observed[id]
		if got.ID != id || got.Class != expected.Class || got.VCPU != expected.VCPU ||
			got.MemoryMiB != expected.MemoryMiB {
			return []string{"final campaign role allocation differs from the frozen stronger topology"}
		}
	}
	return nil
}

func expectedFinalAllocations() map[string]finalObservedAllocation {
	result := make(map[string]finalObservedAllocation, 24)
	for index := range 16 {
		id := fmt.Sprintf("endpoint-%02d", index)
		result[id] = finalObservedAllocation{ID: id, Class: "endpoint-reference", VCPU: 4, MemoryMiB: 8_192}
	}
	result["publisher"] = finalObservedAllocation{ID: "publisher", Class: "publisher-reference", VCPU: 4, MemoryMiB: 8_192}
	result["bridge"] = finalObservedAllocation{ID: "bridge", Class: "h3-s5-b1-v1-strong", VCPU: 8, MemoryMiB: 8_192}
	result["harness"] = finalObservedAllocation{ID: "harness", Class: "collector", VCPU: 16, MemoryMiB: 32_768}
	for _, id := range []string{"ordinary-entry", "initiator", "introduction", "rendezvous", "responder"} {
		result[id] = finalObservedAllocation{ID: id, Class: "route-reference", VCPU: 2, MemoryMiB: 2_048}
	}
	return result
}

func verifyFinalProfiles(values []finalProfileResult) ([]string, []string) {
	want := []finalProfileResult{{"C0", "success", 20, 20}, {"C1", "success", 20, 20},
		{"C2", "success", 20, 20}, {"C3", "bridge-attempt-exhausted", 5, 5},
		{"C4", "bridge-attempt-exhausted", 5, 5}, {"C5", "probe-contained", 20, 20},
		{"C6", "limitation-recorded", 20, 20}}
	if len(values) != len(want) {
		return []string{"C0-C6 result set is incomplete"}, nil
	}
	for index, expected := range want {
		got := values[index]
		if got.ID != expected.ID || got.Terminal != expected.Terminal || got.Attempts != expected.Attempts {
			return []string{"C0-C6 result identity or sample floor is invalid"}, nil
		}
		if got.Successful != expected.Successful {
			return nil, []string{"C0-C6 mandatory attempt failed"}
		}
	}
	return nil, nil
}

func verifyFinalRecovery(value finalRecovery) ([]string, []string) {
	if value.Attempts != 5 {
		return []string{"recovery-parent sample floor is incomplete"}, nil
	}
	if value.ConnectionLoss != 5 || value.LaterStarts != 0 || value.Residuals != 0 ||
		!value.AttemptIdentityStable || !value.DeadlineStable {
		return nil, []string{"recovery-parent clipping gate failed"}
	}
	return nil, nil
}
