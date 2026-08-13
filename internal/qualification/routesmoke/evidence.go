package routesmoke

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	qualification "github.com/dianabuilds/ardents-network/internal/qualification/route"
	"github.com/dianabuilds/ardents-network/internal/route"
)

func terminalEvidence(raw []byte) (route.Evidence, []byte, error) {
	var terminal route.Evidence
	var line []byte
	complete := 0
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 256<<10)
	for scanner.Scan() {
		candidate := bytes.TrimSpace(scanner.Bytes())
		if len(candidate) == 0 || candidate[0] != '{' {
			continue
		}
		var value route.Evidence
		if err := json.Unmarshal(candidate, &value); err != nil {
			return terminal, nil, errors.New("route process emitted malformed JSON evidence")
		}
		if value.Kind == "complete" {
			complete++
			terminal, line = value, append([]byte(nil), candidate...)
		}
	}
	if err := scanner.Err(); err != nil {
		return terminal, nil, err
	}
	if complete != 1 || len(line) == 0 {
		return terminal, nil, errors.New("route process evidence does not contain exactly one terminal record")
	}
	if terminal.Error != "" || terminal.Terminal != "success" {
		return terminal, nil, errors.New("terminal Route process evidence is not successful")
	}
	return terminal, line, nil
}

func joinEvidence(values [][]byte) []byte {
	var output []byte
	for _, value := range values {
		output = append(output, value...)
		output = append(output, '\n')
	}
	return output
}

func writeAttempt(root string, input qualification.Case) error {
	evidencePath := filepath.Join(root, "evidence.jsonl")
	if err := os.WriteFile(evidencePath, input.RawEvidence, 0o600); err != nil {
		return err
	}
	candidates := make([]map[string]any, len(input.Candidates))
	for index, value := range input.Candidates {
		candidates[index] = map[string]any{"node_id": hex32(value.NodeID),
			"public_key": hex32(value.PublicKey), "family": value.Family, "endpoint": value.Endpoint, "domain": value.Domain,
			"capacity": value.Capacity, "valid_from": value.ValidFrom, "valid_until": value.ValidUntil}
	}
	manifest := map[string]any{"evidence": "/run/ardents/evidence/" + filepath.Base(root) + "/evidence.jsonl",
		"evidence_digest": hex32(input.EvidenceDigest), "manifest_digest": hex32(input.ManifestDigest),
		"network_id": hex32(input.NetworkID), "generation": input.Generation, "epoch": input.Epoch,
		"epoch_digest": hex32(input.EpochDigest), "profile": input.Profile, "view_root": hex32(input.ViewRoot),
		"selection_seed": hex32(input.SelectionSeed), "selection_at": input.SelectionAt, "candidates": candidates,
		"excluded_identities": hexIDs(input.ExcludedIdentities), "excluded_families": input.ExcludedFamilies,
		"excluded_domains": input.ExcludedDomains, "node_ids": hexIDArray(input.NodeIDs),
		"public_keys": hexIDArray(input.PublicKeys), "families": input.Families, "endpoints": input.Endpoints,
		"client_pin": hex32(input.ClientPin), "publisher_id": hex32(input.PublisherID), "source_id": input.SourceID,
		"build_digest": hex32(input.BuildDigest), "exited_pids": input.ExitedPIDs,
		"exited_runtime_ids": input.ExitedRuntimeIDs, "container_ids": input.ContainerIDs,
		"cleanup_verified": input.CleanupVerified}
	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, "manifest.json"), append(raw, '\n'), 0o600)
}

func acceptVerifier(root string, raw []byte) (string, error) {
	var result qualification.Result
	if err := json.Unmarshal(bytes.TrimSpace(raw), &result); err != nil {
		return "", err
	}
	if result.Verdict != "pass" && result.Verdict != "fail" && result.Verdict != "invalid" {
		return "", errors.New("independent Route verifier returned an unknown verdict")
	}
	if err := os.WriteFile(filepath.Join(root, "verdict.json"), append(bytes.TrimSpace(raw), '\n'), 0o600); err != nil {
		return "", err
	}
	return result.Verdict, nil
}

func hexIDs(values [][32]byte) []string {
	if values == nil {
		return nil
	}
	result := make([]string, len(values))
	for index := range values {
		result[index] = hex32(values[index])
	}
	return result
}
func hexIDArray(values [4][32]byte) []string {
	result := make([]string, 4)
	for index := range values {
		result[index] = hex32(values[index])
	}
	return result
}
