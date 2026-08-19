package blockedentry

import (
	"fmt"
	"strings"
	"testing"
)

func TestFinalMutationInputsCoverSixIndependentFiveEpisodeCampaigns(t *testing.T) {
	spec := &finalSpec{MutationCampaigns: finalMutationCampaigns()}
	seed := 1
	for index := range spec.MutationCampaigns {
		for range spec.MutationCampaigns[index].CellOrder {
			spec.MutationCampaigns[index].Seeds = append(spec.MutationCampaigns[index].Seeds,
				fmt.Sprintf("%064x", seed))
			seed++
		}
	}
	canaries := canaryCorpus{}
	for _, variant := range secretVariants() {
		canaries.Sets = append(canaries.Sets, canarySet{Variant: variant,
			Invite: strings.Repeat("a", 64), Address: strings.Repeat("b", 64),
			Path: strings.Repeat("c", 64), Certificate: strings.Repeat("d", 64)})
	}
	summary := &finalSummary{}
	for index, cell := range finalCellOrder() {
		summary.Cells = append(summary.Cells, finalCellObservation{ID: cell, Seed: fmt.Sprintf("%064x", index+100),
			ObserverEvidence:  artifactCommitment{Path: "observer", SHA256: strings.Repeat("e", 64), Bytes: 1},
			TelemetryEvidence: artifactCommitment{Path: "telemetry", SHA256: strings.Repeat("f", 64), Bytes: 1}})
	}
	root := t.TempDir()
	if err := publishFinalMutationInputs(root, spec, summary, canaries); err != nil {
		t.Fatal(err)
	}
	if len(summary.MutationArtifacts) != 30 ||
		summary.MutationArtifacts[0].Path != "mutations/collector-loss/collector-loss-0.json" ||
		summary.MutationArtifacts[29].Path != "mutations/pipeline-contamination-certificate/pipeline-contamination-certificate-4.json" {
		t.Fatalf("mutation artifacts=%+v", summary.MutationArtifacts)
	}
}
