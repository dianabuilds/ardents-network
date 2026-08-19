//go:build live

package network_test

import (
	"fmt"
	"testing"
)

func TestFinalCampaignHostAllocationIsExactAndFailClosed(t *testing.T) {
	hosts := fixtureFinalHostAllocation()
	if !validFinalRunnerHostAllocation(hosts) {
		t.Fatalf("hosts=%+v", hosts)
	}
	hosts[0].DedicatedThreads = false
	if validFinalRunnerHostAllocation(hosts) {
		t.Fatal("non-dedicated stand was accepted")
	}
	hosts = fixtureFinalHostAllocation()
	hosts[0].Runtime = &finalRunnerRuntimeHost{Schema: "preauthored"}
	if validFinalRunnerHostAllocation(hosts) {
		t.Fatal("runner schedule accepted operator-authored runtime evidence")
	}
}

func fixtureFinalHostAllocation() []finalRunnerObservedHost {
	host := finalRunnerObservedHost{ID: "sha256:stand", LogicalCPUs: finalHostVCPU,
		MemoryMiB: finalHostMemoryMiB, AllocatedVCPU: finalHostVCPU,
		AllocatedMemoryMiB: finalHostMemoryMiB, DedicatedThreads: true, CgroupV2: true}
	appendAllocation := func(id, class string, vcpu uint16, memory uint32) {
		host.Allocations = append(host.Allocations, finalRunnerObservedAllocation{ID: id, Class: class,
			ProcessNamespace: "cgroup:/ardents/" + id, NetworkNamespace: "netns:" + id,
			VCPU: vcpu, MemoryMiB: memory})
	}
	for index := range 16 {
		appendAllocation(fmt.Sprintf("endpoint-%02d", index), "endpoint-reference", 4, 8_192)
	}
	appendAllocation("publisher", "publisher-reference", 4, 8_192)
	appendAllocation("bridge", "h3-s5-b1-v1-strong", 8, 8_192)
	appendAllocation("harness", "collector", 16, 32_768)
	for _, role := range []string{"ordinary-entry", "initiator", "introduction", "rendezvous", "responder"} {
		appendAllocation(role, "route-reference", 2, 2_048)
	}
	return []finalRunnerObservedHost{host}
}
