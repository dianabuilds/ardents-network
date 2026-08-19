package blockedentry

import (
	"errors"
	"os"
	"path/filepath"
)

func freezePreparationConfigurations(sourceRoot, outputRoot string) ([]artifactCommitment, error) {
	result := make([]artifactCommitment, 0, len(finalConfigurationPaths))
	for _, relative := range finalConfigurationPaths {
		source := filepath.Join(sourceRoot, filepath.FromSlash(relative))
		if filepath.Ext(source) == ".sha256" {
			if err := validateCommitmentInventory(source); err != nil {
				return nil, err
			}
		}
		if expected := finalPublicConfigurationHashes[relative]; expected != "" {
			hash, _, err := hashFile(source)
			if err != nil || hash != expected {
				return nil, errors.Join(err, errors.New("public final configuration differs from the accepted template"))
			}
		}
		target := filepath.Join(outputRoot, filepath.FromSlash(relative))
		if err := ensureParent(target); err != nil {
			return nil, err
		}
		if err := copyStableArtifact(source, target, 0o400); err != nil {
			return nil, err
		}
		commitment, err := commitment(outputRoot, relative)
		if err != nil {
			return nil, err
		}
		result = append(result, commitment)
	}
	return result, nil
}

func exactFinalSpec(commit, sourceHash string, config Config, clientHash, serverHash string,
	configurations []artifactCommitment, runtimeCompose artifactCommitment,
	supplyLock artifactCommitment, productReceipt finalProductReceipt, toolReceipt finalToolReceipt,
	hosts []finalObservedHost,
) finalSpec {
	common := func(id string, vcpu uint16, memory uint32, down, up uint16) finalHostClass {
		return finalHostClass{ID: id, OperatingSystem: "ubuntu-lts", Architecture: "x86-64",
			StorageClass: "ssd", Dedicated: true, VCPU: vcpu, MemoryMiB: memory,
			LinkDownMbit: down, LinkUpMbit: up}
	}
	endpoint := common("endpoint-reference", 4, 8_192, 100, 20)
	endpoint.CPUMeanCores, endpoint.CPUP95Cores, endpoint.MemoryP95MiB = .5, 1, 512
	reference := common("h3-s5-b1-v1", 2, 2_048, 100, 100)
	reference.CPUMaxCores, reference.CPUMeanCores, reference.CPUP95Cores = 1.6, 1.12, 1.28
	reference.MemoryMaxMiB, reference.MemoryP95MiB = 1_280, 896
	reference.HelperRSSP95MiB, reference.HelperFDs, reference.HelperSockets = 128, 64, 32
	reference.MinimumReservePC = 20
	strong := common("h3-s5-b1-v1-strong", 8, 8_192, 400, 400)
	strong.CPUMaxCores, strong.CPUMeanCores, strong.CPUP95Cores = 6.4, 4.48, 5.12
	strong.MemoryMaxMiB, strong.MemoryP95MiB = 5_120, 3_584
	strong.HelperRSSP95MiB, strong.HelperFDs, strong.HelperSockets = 512, 256, 128
	strong.MinimumReservePC = 20
	return finalSpec{Schema: "ardents-h3-s5-final-spec-v1", RepositoryCommit: commit,
		SourceSHA256: sourceHash, LinuxImage: config.LinuxImage, ImageSHA256: config.ImageSHA256,
		ProductImageID: config.ProductImageID, ToolImageID: config.ToolImageID,
		GoBuilderImageID: config.GoBuilderImageID, GoBuilderVersion: finalGoBuilderVersion,
		SupplyLock: supplyLock, RuntimeCompose: runtimeCompose,
		ProductReceipt: productReceipt, ToolReceipt: toolReceipt,
		Kernel: config.Kernel, ClientSHA256: clientHash, ServerSHA256: serverHash, Endpoint: endpoint,
		ReferenceBridge: reference, StrongerBridge: strong,
		Collector: common("h3-s5-collector-v1", 16, 32_768, 1_000, 1_000),
		Network:   finalNetwork{BaseRTTMillis: 80, LossPPM: 1_000, JitterP95Millis: 10},
		Clocks: finalClocks{OrdinaryBlockedMillis: 3_000, TransitionMillis: 2_000,
			AttemptMillis: 64_000, ContactMillis: 15_000, StartupMillis: 5_000,
			InterContactMillis: 1_000, AdapterCleanupMillis: 6_000, CellCleanupMillis: 15_000},
		CellOrder: finalCellOrder(), MutationCampaigns: finalMutationCampaigns(), Configurations: configurations,
		Hosts: hosts}
}

func ensureParent(path string) error { return os.MkdirAll(filepath.Dir(path), 0o700) }
