package blockedentry

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

type finalMutationCampaign struct {
	ID              string   `json:"id"`
	CellOrder       []string `json:"cell_order"`
	Seeds           []string `json:"seeds"`
	ExpectedVerdict string   `json:"expected_verdict"`
}

type finalMutationInput struct {
	Schema          string              `json:"schema"`
	Campaign        string              `json:"campaign"`
	CellID          string              `json:"cell_id"`
	Seed            string              `json:"seed"`
	SourceCellID    string              `json:"source_cell_id"`
	SourceObserver  *artifactCommitment `json:"source_observer,omitempty"`
	SourceTelemetry *artifactCommitment `json:"source_telemetry,omitempty"`
	Publishable     *artifactCommitment `json:"publishable,omitempty"`
}

func publishFinalMutationInputs(root string, spec *finalSpec, summary *finalSummary, canaries canaryCorpus) error {
	if spec == nil && summary == nil {
		return nil
	}
	if spec == nil || summary == nil || len(spec.MutationCampaigns) != 6 || len(summary.Cells) != 564 {
		return errors.New("final evidence-integrity suite is incomplete")
	}
	directory := filepath.Join(root, "mutations")
	if err := os.Mkdir(directory, 0o700); err != nil {
		return err
	}
	for _, campaign := range spec.MutationCampaigns {
		if len(campaign.CellOrder) != 5 || len(campaign.Seeds) != 5 {
			return errors.New("final evidence-integrity campaign inventory is incomplete")
		}
		for episode := range campaign.CellOrder {
			value, err := materializeFinalMutation(root, campaign, episode, canaries, summary.Cells[episode])
			if err != nil {
				return err
			}
			relative := filepath.ToSlash(filepath.Join("mutations", campaign.ID,
				campaign.ID+"-"+string(rune('0'+episode))+".json"))
			absolute := filepath.Join(root, filepath.FromSlash(relative))
			if err := os.MkdirAll(filepath.Dir(absolute), 0o700); err != nil {
				return err
			}
			if err := writeJSON(absolute, value); err != nil {
				return err
			}
			artifact, err := commitment(root, relative)
			if err != nil {
				return err
			}
			summary.MutationArtifacts = append(summary.MutationArtifacts, artifact)
		}
	}
	return nil
}

func materializeFinalMutation(root string, campaign finalMutationCampaign, episode int,
	canaries canaryCorpus, source finalCellObservation,
) (finalMutationInput, error) {
	value := finalMutationInput{Schema: "ardents-h3-final-evidence-mutation-v2", Campaign: campaign.ID,
		CellID: campaign.CellOrder[episode], Seed: campaign.Seeds[episode], SourceCellID: source.ID}
	if campaign.ID != "collector-loss" {
		value.SourceTelemetry = &source.TelemetryEvidence
	}
	if campaign.ID != "blocker-loss" {
		value.SourceObserver = &source.ObserverEvidence
	}
	directory := filepath.ToSlash(filepath.Join("mutations", campaign.ID, "episode-"+string(rune('0'+episode))))
	write := func(name, contents string) (*artifactCommitment, error) {
		relative := filepath.ToSlash(filepath.Join(directory, name))
		absolute := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(absolute), 0o700); err != nil {
			return nil, err
		}
		if err := os.WriteFile(absolute, []byte(contents), 0o600); err != nil {
			return nil, err
		}
		artifact, err := commitment(root, relative)
		return &artifact, err
	}
	var err error
	if strings.HasPrefix(campaign.ID, "pipeline-contamination-") {
		set, ok := finalCanarySet(canaries, campaign.ID)
		if !ok {
			return finalMutationInput{}, errors.New("evidence-integrity canary is absent")
		}
		field := campaign.ID[strings.LastIndexByte(campaign.ID, '-')+1:]
		probe := map[string]string{"invite": set.Invite, "address": set.Address,
			"path": set.Path, "certificate": set.Certificate}[field]
		if probe == "" {
			return finalMutationInput{}, errors.New("evidence-integrity canary field is invalid")
		}
		value.Publishable, err = write("publishable.bin", probe)
		if err != nil {
			return finalMutationInput{}, err
		}
	}
	return value, nil
}

func finalCanarySet(corpus canaryCorpus, variant string) (canarySet, bool) {
	for _, set := range corpus.Sets {
		if set.Variant == variant {
			return set, true
		}
	}
	return canarySet{}, false
}
