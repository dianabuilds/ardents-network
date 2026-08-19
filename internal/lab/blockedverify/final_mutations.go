package blockedverify

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

func verifyFinalMutationCampaigns(root string, spec *finalSpec, summary *finalSummary,
	canaries canaryCorpus,
) ([]string, []string) {
	components := []string{"candidate:unverified:564"}
	if spec == nil || summary == nil || len(spec.MutationCampaigns) != 6 || len(summary.MutationArtifacts) != 30 {
		return components, []string{"final evidence-integrity suite is incomplete"}
	}
	artifactIndex := 0
	sources := make(map[string]finalCellObservation, len(summary.Cells))
	for _, source := range summary.Cells {
		sources[source.ID] = source
	}
	var reasons []string
	for _, campaign := range spec.MutationCampaigns {
		component := campaign.ID + ":invalid:5"
		for episode := range campaign.CellOrder {
			artifact := summary.MutationArtifacts[artifactIndex]
			artifactIndex++
			relative := fmt.Sprintf("mutations/%s/%s-%d.json", campaign.ID, campaign.ID, episode)
			value, err := readFinalMutation(root, relative, artifact)
			if err != nil || value.Campaign != campaign.ID || value.CellID != campaign.CellOrder[episode] ||
				value.Seed != campaign.Seeds[episode] ||
				!mutationIsIndependentlyInvalid(root, value, campaign.ID, episode, canaries, sources[value.SourceCellID]) {
				component = campaign.ID + ":invalid-input:5"
				reasons = append(reasons, "final evidence-integrity mutation is not independently rejected: "+campaign.ID)
				break
			}
		}
		components = append(components, component)
	}
	return components, reasons
}

func readFinalMutation(root, relative string, artifact artifactCommitment) (finalMutationInput, error) {
	path, safe := safeArtifactPath(root, artifact.Path)
	if !safe || artifact.Path != relative || artifact.Bytes < 1 || !isHexDigest(artifact.SHA256, 32) {
		return finalMutationInput{}, errors.New("mutation artifact identity is invalid")
	}
	raw, hash, size, err := snapshotMeasurement(path)
	if err != nil || hash != artifact.SHA256 || size != artifact.Bytes {
		return finalMutationInput{}, errors.New("mutation artifact commitment mismatch")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var value finalMutationInput
	if err := decoder.Decode(&value); err != nil || decoder.Decode(&struct{}{}) != io.EOF ||
		value.Schema != "ardents-h3-final-evidence-mutation-v2" {
		return finalMutationInput{}, errors.New("mutation artifact schema is invalid")
	}
	canonical, err := json.MarshalIndent(value, "", "  ")
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) {
		return finalMutationInput{}, errors.New("mutation artifact is not canonical")
	}
	return value, nil
}

func mutationIsIndependentlyInvalid(root string, value finalMutationInput, campaign string, episode int,
	canaries canaryCorpus, source finalCellObservation,
) bool {
	if source.ID == "" || value.SourceCellID != source.ID {
		return false
	}
	observer := value.SourceObserver != nil && *value.SourceObserver == source.ObserverEvidence &&
		len(verifyFinalObserverEvidence(root, []finalCellObservation{source})) == 0
	telemetry := value.SourceTelemetry != nil && *value.SourceTelemetry == source.TelemetryEvidence &&
		len(verifyFinalTelemetryEvidence(root, []finalCellObservation{source})) == 0
	switch campaign {
	case "collector-loss":
		return value.SourceTelemetry == nil && observer && value.Publishable == nil
	case "blocker-loss":
		return telemetry && value.SourceObserver == nil && value.Publishable == nil
	}
	if !observer || !telemetry || !strings.HasPrefix(campaign, "pipeline-contamination-") {
		return false
	}
	directory := filepath.ToSlash(filepath.Join("mutations", campaign, fmt.Sprintf("episode-%d", episode)))
	set, ok := canarySetForVariant(canaries, campaign)
	if !ok {
		return false
	}
	field := campaign[strings.LastIndexByte(campaign, '-')+1:]
	want := map[string]string{"invite": set.Invite, "address": set.Address,
		"path": set.Path, "certificate": set.Certificate}[field]
	path := filepath.ToSlash(filepath.Join(directory, "publishable.bin"))
	if want == "" || !validMutationArtifact(root, path, value.Publishable) {
		return false
	}
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
	return err == nil && string(raw) == want
}

func validMutationArtifact(root, expected string, artifact *artifactCommitment) bool {
	if artifact == nil || artifact.Path != expected || artifact.Bytes < 1 || !isHexDigest(artifact.SHA256, 32) {
		return false
	}
	path, safe := safeArtifactPath(root, expected)
	hash, size, err := hashFile(path)
	return safe && err == nil && hash == artifact.SHA256 && size == artifact.Bytes
}
