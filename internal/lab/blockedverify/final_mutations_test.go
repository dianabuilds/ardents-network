package blockedverify

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIndependentVerifierRequiresPassPlusSixInvalidComponents(t *testing.T) {
	spec := validFinalSpec()
	canaries := testFinalMutationCanaries()
	root := t.TempDir()
	source := materializeFinalMutationSource(t, root, "profile/C0/00")
	if reasons := verifyFinalObserverEvidence(root, []finalCellObservation{source}); len(reasons) != 0 {
		t.Fatalf("mutation source observer evidence: %v", reasons)
	}
	if reasons := verifyFinalTelemetryEvidence(root, []finalCellObservation{source}); len(reasons) != 0 {
		raw, reason := loadFinalRawTelemetry(root, source)
		t.Fatalf("mutation source telemetry evidence: %v reason=%s files=%+v streams=%t", reasons, reason,
			raw.Files, validFinalTelemetryStreams(raw.Files))
	}
	summary := &finalSummary{Cells: []finalCellObservation{source}}
	for _, campaign := range spec.MutationCampaigns {
		for episode := range campaign.CellOrder {
			value := finalMutationInput{Schema: "ardents-h3-final-evidence-mutation-v2", Campaign: campaign.ID,
				CellID: campaign.CellOrder[episode], Seed: campaign.Seeds[episode], SourceCellID: source.ID}
			if campaign.ID != "collector-loss" {
				value.SourceTelemetry = &source.TelemetryEvidence
			}
			if campaign.ID != "blocker-loss" {
				value.SourceObserver = &source.ObserverEvidence
			}
			directory := fmt.Sprintf("mutations/%s/episode-%d", campaign.ID, episode)
			writePart := func(name, contents string) *artifactCommitment {
				relative := directory + "/" + name
				absolute := filepath.Join(root, filepath.FromSlash(relative))
				if err := os.MkdirAll(filepath.Dir(absolute), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(absolute, []byte(contents), 0o600); err != nil {
					t.Fatal(err)
				}
				hash, size, err := hashFile(absolute)
				if err != nil {
					t.Fatal(err)
				}
				return &artifactCommitment{Path: relative, SHA256: hash, Bytes: size}
			}
			switch campaign.ID {
			case "collector-loss", "blocker-loss":
			default:
				set, _ := canarySetForVariant(canaries, campaign.ID)
				field := campaign.ID[strings.LastIndexByte(campaign.ID, '-')+1:]
				probe := map[string]string{"invite": set.Invite, "address": set.Address,
					"path": set.Path, "certificate": set.Certificate}[field]
				value.Publishable = writePart("publishable.bin", probe)
			}
			if !mutationIsIndependentlyInvalid(root, value, campaign.ID, episode, canaries, source) {
				t.Fatalf("materialized mutation is not invalid: %+v", value)
			}
			relative := fmt.Sprintf("mutations/%s/%s-%d.json", campaign.ID, campaign.ID, episode)
			absolute := filepath.Join(root, filepath.FromSlash(relative))
			if err := os.MkdirAll(filepath.Dir(absolute), 0o700); err != nil {
				t.Fatal(err)
			}
			raw, err := json.MarshalIndent(value, "", "  ")
			if err != nil {
				t.Fatalf("write mutation artifact: %v", err)
			}
			if err := os.WriteFile(absolute, append(raw, '\n'), 0o600); err != nil {
				t.Fatal(err)
			}
			hash, size, err := hashFile(absolute)
			if err != nil {
				t.Fatal(err)
			}
			summary.MutationArtifacts = append(summary.MutationArtifacts,
				artifactCommitment{Path: relative, SHA256: hash, Bytes: size})
		}
	}
	components, reasons := verifyFinalMutationCampaigns(root, &spec, summary, canaries)
	if len(reasons) != 0 || len(components) != 7 || components[0] != "candidate:unverified:564" {
		t.Fatalf("components=%+v reasons=%v", components, reasons)
	}
	for _, component := range components[1:] {
		if !strings.Contains(component, ":invalid:5") {
			t.Fatalf("mutation component=%+v", component)
		}
	}
	summary.MutationArtifacts[0].SHA256 = strings.Repeat("0", 64)
	if _, reasons := verifyFinalMutationCampaigns(root, &spec, summary, canaries); len(reasons) == 0 {
		t.Fatal("mutation commitment was not independently checked")
	}
}

func TestFinalCandidateComponentReflectsIndependentVerdict(t *testing.T) {
	if got := finalCandidateComponent(nil, nil, nil); got != "candidate:pass:564" {
		t.Fatalf("clean component=%q", got)
	}
	if got := finalCandidateComponent(nil, []string{"threshold"}, nil); got != "candidate:fail:564" {
		t.Fatalf("failed component=%q", got)
	}
	if got := finalCandidateComponent(nil, nil, []string{"evidence"}); got != "candidate:invalid:564" {
		t.Fatalf("invalid component=%q", got)
	}
}

func testFinalMutationCanaries() canaryCorpus {
	result := canaryCorpus{}
	for _, variant := range secretVariants() {
		result.Sets = append(result.Sets, canarySet{Variant: variant,
			Invite: strings.Repeat("a", 64), Address: strings.Repeat("b", 64),
			Path: strings.Repeat("c", 64), Certificate: strings.Repeat("d", 64)})
	}
	return result
}
