package blockedentry

import "testing"

func TestFinalPreparationFreezesEveryScheduledCell(t *testing.T) {
	order := finalCellOrder()
	if len(order) != 594 {
		t.Fatalf("final cell order count=%d want=594", len(order))
	}
	if order[0] != "profile/C0/00" || order[109] != "profile/C6/19" ||
		order[110] != "capacity/h3-s5-b1-v1/0" || order[len(order)-1] !=
		"hostile/G9-ledger-leakage/pipeline-contamination-certificate/4" {
		t.Fatalf("final cell order boundaries changed: first=%s profile-end=%s capacity=%s last=%s",
			order[0], order[109], order[110], order[len(order)-1])
	}
	seen := make(map[string]bool, len(order))
	for _, identity := range order {
		if seen[identity] {
			t.Fatalf("duplicate final cell identity %s", identity)
		}
		seen[identity] = true
	}
}

func TestExactFinalSpecUsesAcceptedProfiles(t *testing.T) {
	config := Config{LinuxImage: "ubuntu:24.04", ImageSHA256: "image", Kernel: "kernel"}
	value := exactFinalSpec("commit", "source", config, "client", "server", nil)
	if value.ReferenceBridge.ID != "h3-s5-b1-v1" || value.ReferenceBridge.VCPU != 2 ||
		value.ReferenceBridge.MemoryMiB != 2_048 || value.ReferenceBridge.HelperSockets != 32 ||
		value.StrongerBridge.ID != "h3-s5-b1-v1-strong" || value.StrongerBridge.VCPU != 8 ||
		value.StrongerBridge.MemoryMiB != 8_192 || value.StrongerBridge.HelperSockets != 128 ||
		value.Collector.VCPU != 16 || value.Collector.MemoryMiB != 32_768 ||
		value.Network.BaseRTTMillis != 80 || value.Network.LossPPM != 1_000 ||
		value.Clocks.AttemptMillis != 64_000 || value.Clocks.ContactMillis != 15_000 {
		t.Fatalf("prepared final profile differs from R-037: %+v", value)
	}
}
