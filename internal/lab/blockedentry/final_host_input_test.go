package blockedentry

import (
	"fmt"
	"path/filepath"
	"testing"
)

func TestFinalHostAllocationMustBeFrozenAndNonSynthetic(t *testing.T) {
	root := t.TempDir()
	record := finalHostAllocationRecord{Schema: "ardents-h3-final-host-allocation-v1",
		Hosts: fixtureFinalHostAllocation()}
	path := filepath.Join(root, "host.json")
	if err := writeJSON(path, record); err != nil {
		t.Fatal(err)
	}
	hosts, err := loadFinalHostAllocation(path)
	if err != nil || len(hosts) != 1 {
		t.Fatalf("hosts=%+v err=%v", hosts, err)
	}
	record.Hosts[0].Allocations[0].ProcessNamespace = "allocation:endpoint-00:process"
	if err := writeJSON(filepath.Join(root, "synthetic.json"), record); err != nil {
		t.Fatal(err)
	}
	if _, err := loadFinalHostAllocation(filepath.Join(root, "synthetic.json")); err == nil {
		t.Fatal("runner-synthesized namespace identity was accepted")
	}
	record = finalHostAllocationRecord{Schema: "ardents-h3-final-host-allocation-v1",
		Hosts: fixtureFinalHostAllocation()}
	record.Hosts[0].Runtime = &finalRuntimeHost{Schema: "operator-authored-runtime"}
	if err := writeJSON(filepath.Join(root, "preauthored-runtime.json"), record); err != nil {
		t.Fatal(err)
	}
	if _, err := loadFinalHostAllocation(filepath.Join(root, "preauthored-runtime.json")); err == nil {
		t.Fatal("reservation input was allowed to author runtime evidence")
	}
}

func fixtureFinalHostAllocation() []finalObservedHost {
	host := finalObservedHost{ID: "sha256:stand", LogicalCPUs: 102, MemoryMiB: 190_464,
		AllocatedVCPU: 102, AllocatedMemoryMiB: 190_464, DedicatedThreads: true, CgroupV2: true}
	appendAllocation := func(id, class string, vcpu uint16, memory uint32) {
		host.Allocations = append(host.Allocations, finalObservedAllocation{ID: id, Class: class,
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
	return []finalObservedHost{host}
}
