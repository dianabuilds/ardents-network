package blockedverify

import "testing"

func TestFinalInputInventoryRequiresExactEmptyDockerAuthority(t *testing.T) {
	spec := validFinalSpec()
	values := []artifactCommitment{
		{Path: "canaries.json", SHA256: spec.SourceSHA256, Bytes: 1},
		{Path: "supply/runner", SHA256: spec.SourceSHA256, Bytes: 1},
		{Path: "supply/client", SHA256: spec.SourceSHA256, Bytes: 1},
		{Path: "supply/server", SHA256: spec.SourceSHA256, Bytes: 1},
		{Path: "final-spec.json", SHA256: spec.SourceSHA256, Bytes: 1},
		spec.RuntimeCompose,
		spec.SupplyLock,
		{Path: "runtime/docker-config/config.json", SHA256: emptyDockerConfigSHA256, Bytes: 3},
	}
	values = append(values, spec.Configurations...)
	if reasons := verifyFinalInputArtifacts(values, spec); len(reasons) != 0 {
		t.Fatalf("exact harness inventory rejected: %v", reasons)
	}
	values[7].SHA256 = spec.SourceSHA256
	if reasons := verifyFinalInputArtifacts(values, spec); len(reasons) == 0 {
		t.Fatal("credential-bearing Docker configuration was accepted")
	}
}
