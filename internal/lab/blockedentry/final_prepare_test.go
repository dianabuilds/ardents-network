package blockedentry

import (
	"fmt"
	"strings"
	"testing"
)

func TestFinalPreparationFreezesEveryScheduledCell(t *testing.T) {
	order := finalCellOrder()
	if len(order) != 564 {
		t.Fatalf("final candidate cell order count=%d want=564", len(order))
	}
	if order[0] != "profile/C0/00" || order[109] != "profile/C6/19" ||
		order[110] != "capacity/h3-s5-b1-v1/0" || order[len(order)-1] !=
		"hostile/G9-ledger-leakage/candidate-leak-certificate/4" {
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

func TestFinalPreparationSeparatesEvidenceIntegrityCampaigns(t *testing.T) {
	campaigns := finalMutationCampaigns()
	if len(campaigns) != 6 {
		t.Fatalf("evidence-integrity campaigns=%d want=6", len(campaigns))
	}
	seen := make(map[string]bool)
	for _, campaign := range campaigns {
		if campaign.ID == "" || campaign.ExpectedVerdict != "invalid" || len(campaign.CellOrder) != 5 {
			t.Fatalf("invalid evidence-integrity campaign: %+v", campaign)
		}
		for _, cell := range campaign.CellOrder {
			if seen[cell] || finalCellIndex(cell) >= 0 {
				t.Fatalf("mutation cell is duplicated or present in candidate order: %s", cell)
			}
			seen[cell] = true
		}
	}
	if len(seen) != 30 {
		t.Fatalf("evidence-integrity episodes=%d want=30", len(seen))
	}
}

func TestFinalSuiteOrderRejectsSeedReuseAcrossCampaigns(t *testing.T) {
	value := finalSpec{CellOrder: finalCellOrder(), MutationCampaigns: finalMutationCampaigns()}
	seed := 1
	for range value.CellOrder {
		value.Seeds = append(value.Seeds, fmt.Sprintf("%064x", seed))
		seed++
	}
	for index := range value.MutationCampaigns {
		for range value.MutationCampaigns[index].CellOrder {
			value.MutationCampaigns[index].Seeds = append(value.MutationCampaigns[index].Seeds,
				fmt.Sprintf("%064x", seed))
			seed++
		}
	}
	if !validFinalSuiteOrder(value) {
		t.Fatal("valid final suite order rejected")
	}
	value.MutationCampaigns[0].Seeds[0] = value.Seeds[0]
	if validFinalSuiteOrder(value) {
		t.Fatal("cross-campaign seed reuse accepted")
	}
}

func TestExactFinalSpecUsesAcceptedProfiles(t *testing.T) {
	config := Config{LinuxImage: "ubuntu:24.04", ImageSHA256: "image", Kernel: "kernel",
		ProductImageID:   "sha256:" + strings.Repeat("a", 64),
		ToolImageID:      "sha256:" + strings.Repeat("b", 64),
		GoBuilderImageID: "sha256:" + strings.Repeat("d", 64)}
	compose := artifactCommitment{Path: finalRuntimeComposePath, SHA256: strings.Repeat("c", 64), Bytes: 10}
	lock := artifactCommitment{Path: finalSupplyLockPath, SHA256: strings.Repeat("e", 64), Bytes: 10}
	product := finalProductReceipt{SourceSHA256: "source"}
	tool := finalToolReceipt{BaseDigest: finalImageHash}
	value := exactFinalSpec("commit", "source", config, "client", "server", nil, compose, lock, product, tool, nil)
	if value.ReferenceBridge.ID != "h3-s5-b1-v1" || value.ReferenceBridge.VCPU != 2 ||
		value.ReferenceBridge.MemoryMiB != 2_048 || value.ReferenceBridge.HelperSockets != 32 ||
		value.StrongerBridge.ID != "h3-s5-b1-v1-strong" || value.StrongerBridge.VCPU != 8 ||
		value.StrongerBridge.MemoryMiB != 8_192 || value.StrongerBridge.HelperSockets != 128 ||
		value.Collector.VCPU != 16 || value.Collector.MemoryMiB != 32_768 ||
		value.Network.BaseRTTMillis != 80 || value.Network.LossPPM != 1_000 ||
		value.Clocks.AttemptMillis != 64_000 || value.Clocks.ContactMillis != 15_000 ||
		value.ProductImageID != config.ProductImageID || value.ToolImageID != config.ToolImageID ||
		value.GoBuilderImageID != config.GoBuilderImageID ||
		value.GoBuilderVersion != finalGoBuilderVersion ||
		value.SupplyLock != lock || value.RuntimeCompose != compose ||
		value.ProductReceipt != product || value.ToolReceipt != tool {
		t.Fatalf("prepared final profile differs from R-037: %+v", value)
	}
}

func TestFinalImageReceiptParsersBindEveryExecutableAndToolInput(t *testing.T) {
	hash := strings.Repeat("a", 64)
	productOutput := hash + "\n" + hash + "\n" + hash + "\n" + hash + "\n"
	for _, path := range []string{"/usr/local/bin/ardents-route", "/usr/local/bin/ardents-bridge",
		"/usr/local/bin/ardents-service", "/usr/local/bin/ardents-stream-app",
		"/usr/local/bin/ardents-publish-app", "/usr/local/bin/network-live.test",
		"/usr/local/bin/camouflage.test"} {
		productOutput += hash + "  " + path + "\n"
	}
	product, err := parseProductReceipt(productOutput, hash, hash, hash, hash)
	if err != nil || !validProductReceipt(product, hash) {
		t.Fatalf("product receipt=%+v err=%v", product, err)
	}
	toolOutput := hash + "  /usr/share/ardents/carrier-lab-tools.lock\n" +
		hash + "  /usr/local/bin/carrier-lab\n" + hash + "\n"
	tool, err := parseToolReceipt(toolOutput, hash, hash)
	if err != nil || !validToolReceipt(tool) {
		t.Fatalf("tool receipt=%+v err=%v", tool, err)
	}
}

func TestFinalManifestRejectsRunnerOutsideProductReceipt(t *testing.T) {
	spec := &finalSpec{ProductReceipt: finalProductReceipt{NetworkSHA256: strings.Repeat("a", 64)}}
	if err := validateFinalRunnerBinding(strings.Repeat("b", 64), spec); err == nil ||
		!strings.Contains(err.Error(), "archive-built product receipt") {
		t.Fatalf("runner substitution error=%v", err)
	}
}
