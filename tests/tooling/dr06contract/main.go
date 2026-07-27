// Package main validates the executable DR-06 preflight workflow contract.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type contractDocument struct {
	SchemaVersion int            `json:"schema_version"`
	Gates         []gateContract `json:"gates"`
}

type gateContract struct {
	ID                      string   `json:"id"`
	WorkflowJobID           string   `json:"workflow_job_id"`
	WorkflowJobName         string   `json:"workflow_job_name"`
	WorkflowJobNameTemplate string   `json:"workflow_job_name_template"`
	Command                 string   `json:"command"`
	WorkflowCommandFragment string   `json:"workflow_command_fragment"`
	Environment             string   `json:"environment"`
	EnvironmentOutputPath   string   `json:"environment_output_path"`
	MaterialNames           []string `json:"material_names"`
	Dependencies            []string `json:"dependencies"`
	CapabilityClaims        []string `json:"capability_claims"`
	AttemptArtifacts        []string `json:"attempt_artifacts"`
	RequiredArtifacts       []string `json:"required_artifacts"`
}

func main() {
	if err := run("."); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("DR-06 workflow contract valid")
}

func run(root string) error {
	workflow, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "ci.yml"))
	if err != nil {
		return fmt.Errorf("read workflow: %w", err)
	}
	contract, err := os.ReadFile(filepath.Join(root, "tests", "ci", "dr06-gates.json"))
	if err != nil {
		return fmt.Errorf("read DR-06 gate contract: %w", err)
	}
	readme, err := os.ReadFile(filepath.Join(root, "tests", "README.md"))
	if err != nil {
		return fmt.Errorf("read test documentation: %w", err)
	}
	nativeGate, err := os.ReadFile(filepath.Join(root, "tests", "ci", "native-install-gate.ps1"))
	if err != nil {
		return fmt.Errorf("read native-install gate: %w", err)
	}
	if err := validate(workflow, contract, readme, nativeGate); err != nil {
		return err
	}
	capabilities, err := os.ReadFile(filepath.Join(root, "docs", "engineering", "capabilities.json"))
	if err != nil {
		return fmt.Errorf("read capability catalogue: %w", err)
	}
	if err := validateCapabilityGateOwnership(contract, capabilities); err != nil {
		return err
	}
	return validatePinnedGateMaterials(root)
}

func validate(workflow, contractJSON, readme, nativeGate []byte) error {
	var workflowNode yaml.Node
	if err := yaml.Unmarshal(workflow, &workflowNode); err != nil {
		return fmt.Errorf("CI workflow YAML is invalid: %w", err)
	}
	var contract contractDocument
	decoder := json.NewDecoder(bytes.NewReader(contractJSON))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&contract); err != nil {
		return fmt.Errorf("DR-06 gate contract JSON is invalid: %w", err)
	}
	if contract.SchemaVersion != 1 || len(contract.Gates) != 17 {
		return fmt.Errorf("DR-06 gate contract must contain schema 1 and 17 gates")
	}

	root, err := documentMapping(&workflowNode)
	if err != nil {
		return err
	}
	dispatch, err := mappingPath(root, "on", "workflow_dispatch")
	if err != nil {
		return err
	}
	inputs, err := mappingPath(dispatch, "inputs")
	if err != nil {
		return err
	}
	for _, input := range []string{"release_version", "durable_evidence_uri", "native_install_contract"} {
		if _, ok := mappingValue(inputs, input); !ok {
			return fmt.Errorf("workflow_dispatch input is missing: %s", input)
		}
	}
	concurrency, err := mappingPath(root, "concurrency")
	if err != nil {
		return err
	}
	group, err := scalarValue(concurrency, "group")
	if err != nil {
		return err
	}
	if !strings.Contains(group, "qualification-") || !strings.Contains(group, "release-") {
		return errors.New("qualification/release concurrency is not isolated from ordinary ref CI")
	}
	cancel, err := scalarValue(concurrency, "cancel-in-progress")
	if err != nil {
		return err
	}
	if cancel != "${{ !(startsWith(github.ref, 'refs/tags/v') || inputs.release_version != '') }}" {
		return errors.New("qualification or release attempts can be cancelled in progress")
	}

	jobs, err := mappingPath(root, "jobs")
	if err != nil {
		return err
	}
	if err := validateQualificationControlPlane(jobs, workflow); err != nil {
		return err
	}
	seenIDs := map[string]struct{}{}
	seenJobContracts := map[string]gateContract{}
	for _, gate := range contract.Gates {
		if gate.ID == "" || gate.WorkflowJobID == "" || gate.WorkflowJobName == "" ||
			gate.WorkflowJobNameTemplate == "" || gate.Command == "" ||
			gate.WorkflowCommandFragment == "" || gate.Environment == "" ||
			gate.EnvironmentOutputPath == "" ||
			len(gate.AttemptArtifacts) == 0 || len(gate.RequiredArtifacts) == 0 {
			return fmt.Errorf("DR-06 gate contract is incomplete: %s", gate.ID)
		}
		if _, exists := seenIDs[gate.ID]; exists {
			return fmt.Errorf("DR-06 gate ID is duplicated: %s", gate.ID)
		}
		seenIDs[gate.ID] = struct{}{}

		job, ok := mappingValue(jobs, gate.WorkflowJobID)
		if !ok || job.Kind != yaml.MappingNode {
			return fmt.Errorf("DR-06 workflow job is missing: %s", gate.WorkflowJobID)
		}
		name, err := scalarValue(job, "name")
		if err != nil || name != gate.WorkflowJobNameTemplate {
			return fmt.Errorf("DR-06 workflow job name mismatch for %s", gate.ID)
		}
		if previous, exists := seenJobContracts[gate.WorkflowJobID]; exists {
			if previous.WorkflowJobNameTemplate != gate.WorkflowJobNameTemplate ||
				!equalStrings(previous.Dependencies, gate.Dependencies) ||
				previous.WorkflowCommandFragment != gate.WorkflowCommandFragment {
				return fmt.Errorf("shared workflow job contract diverges: %s", gate.WorkflowJobID)
			}
		} else {
			seenJobContracts[gate.WorkflowJobID] = gate
			actualNeeds, err := workflowNeeds(job)
			if err != nil {
				return fmt.Errorf("%s needs: %w", gate.WorkflowJobID, err)
			}
			if !equalStrings(actualNeeds, gate.Dependencies) {
				return fmt.Errorf("%s needs %v, contract requires %v", gate.WorkflowJobID, actualNeeds, gate.Dependencies)
			}
			runs, uploadPaths, err := workflowSteps(job)
			if err != nil {
				return fmt.Errorf("%s steps: %w", gate.WorkflowJobID, err)
			}
			if !strings.Contains(strings.Join(runs, "\n"), gate.WorkflowCommandFragment) {
				return fmt.Errorf("%s does not execute contract fragment %q", gate.WorkflowJobID, gate.WorkflowCommandFragment)
			}
			runText := strings.Join(runs, "\n")
			if !strings.Contains(runText, "capture-dr06-environment.ps1") {
				return fmt.Errorf("%s does not capture a DR-06 environment manifest", gate.WorkflowJobID)
			}
			captureGate := gate.ID
			switch gate.WorkflowJobID {
			case "tagged":
				captureGate = "tagged-${{ matrix.suite }}"
			case "focused-tagged":
				captureGate = "${{ matrix.gate }}"
			case "release-builds":
				captureGate = "release-build-${{ matrix.rebuild }}"
			}
			if !strings.Contains(runText, "capture-dr06-environment.ps1 -Gate "+captureGate) ||
				strings.Contains(runText, "-EnvironmentContract") ||
				strings.Contains(runText, "-OutputPath") {
				return fmt.Errorf("%s does not derive its environment mapping from the gate contract", gate.WorkflowJobID)
			}
			if len(uploadPaths) == 0 {
				return fmt.Errorf("%s has no 90-day artifact upload", gate.WorkflowJobID)
			}
			environmentOutput := gate.EnvironmentOutputPath
			switch gate.WorkflowJobID {
			case "tagged":
				environmentOutput = strings.Replace(environmentOutput, gate.ID, "tagged-${{ matrix.suite }}", 1)
			case "focused-tagged":
				environmentOutput = strings.Replace(environmentOutput, gate.ID, "${{ matrix.gate }}", 1)
			case "release-builds":
				if !strings.Contains(runText, "tests/.artifacts/release-build-${{ matrix.rebuild }} dist/repro/_dr06-evidence") {
					return fmt.Errorf("%s does not stage its contract-derived environment manifest", gate.WorkflowJobID)
				}
				environmentOutput = ""
			}
			if environmentOutput != "" && !pathCoveredByUpload(environmentOutput, uploadPaths) {
				return fmt.Errorf("%s does not upload contract environment path %s", gate.WorkflowJobID, environmentOutput)
			}
		}
	}

	requiredDocs := []string{
		"Linux test container is the canonical `fast` runtime",
		"privileged systemd acceptance",
		"container on a GitHub-hosted Ubuntu runner",
		"staging retention",
		"supported lifetime",
		"system temporary directory",
		"confirm-dr06-retention.ps1",
	}
	for _, fragment := range requiredDocs {
		if !bytes.Contains(readme, []byte(fragment)) {
			return fmt.Errorf("test documentation is missing DR-06 contract fragment %q", fragment)
		}
	}
	for _, fragment := range []string{"systemd.log", "environment.json", "SHA256SUMS"} {
		if !bytes.Contains(nativeGate, []byte(fragment)) {
			return fmt.Errorf("native-install gate does not retain %s", fragment)
		}
	}
	return nil
}

func validateQualificationControlPlane(jobs *yaml.Node, workflow []byte) error {
	for _, line := range strings.Split(string(workflow), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "uses:") && !strings.HasPrefix(line, "- uses:") {
			continue
		}
		reference := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "- "), "uses:"))
		parts := strings.Split(reference, "@")
		if len(parts) != 2 || len(parts[1]) != 40 {
			return fmt.Errorf("workflow action is not pinned to an immutable commit: %s", reference)
		}
		for _, character := range parts[1] {
			if !strings.ContainsRune("0123456789abcdef", character) {
				return fmt.Errorf("workflow action is not pinned to an immutable commit: %s", reference)
			}
		}
	}

	indexJob, ok := mappingValue(jobs, "qualification-index")
	if !ok || indexJob.Kind != yaml.MappingNode {
		return errors.New("qualification-index job is missing")
	}
	indexIf, err := scalarValue(indexJob, "if")
	if err != nil || !strings.Contains(indexIf, "always()") ||
		!strings.Contains(indexIf, "inputs.release_version != ''") {
		return errors.New("qualification-index must run for failed qualification dependencies")
	}
	needs, err := workflowNeeds(indexJob)
	if err != nil {
		return fmt.Errorf("qualification-index needs: %w", err)
	}
	requiredNeeds := []string{
		"windows-interface", "static", "critical-lifecycle", "fast", "tagged",
		"focused-tagged", "failure-contract", "security", "deployment",
		"native-install", "multinode", "release-builds", "release-candidate",
	}
	if !equalStrings(needs, requiredNeeds) {
		return fmt.Errorf("qualification-index needs %v, contract requires %v", needs, requiredNeeds)
	}
	indexText := string(workflow)
	for _, fragment := range []string{
		"create-dr06-rejection-index.ps1",
		"always() && steps.create-index.outcome != 'success'",
		"Keep rejected qualification runs failed",
		"if: github.event_name == 'push' && startsWith(github.ref, 'refs/tags/v')",
	} {
		if !strings.Contains(indexText, fragment) {
			return fmt.Errorf("qualification control plane is missing %q", fragment)
		}
	}
	return nil
}

func validateCapabilityGateOwnership(contractJSON, capabilitiesJSON []byte) error {
	var contract contractDocument
	if err := json.Unmarshal(contractJSON, &contract); err != nil {
		return fmt.Errorf("parse DR-06 gate contract for ownership: %w", err)
	}
	var catalogue struct {
		EvidenceGates []struct {
			ID                  string `json:"id"`
			CIJob               string `json:"ci_job"`
			ScenarioID          string `json:"scenario_id"`
			RequiredEnvironment string `json:"required_environment"`
		} `json:"evidence_gates"`
	}
	if err := json.Unmarshal(capabilitiesJSON, &catalogue); err != nil {
		return fmt.Errorf("parse capability catalogue for ownership: %w", err)
	}
	byID := make(map[string]gateContract, len(contract.Gates))
	for _, gate := range contract.Gates {
		byID[gate.ID] = gate
	}
	for _, evidenceGate := range catalogue.EvidenceGates {
		gate, exists := byID[evidenceGate.ID]
		if !exists {
			continue
		}
		if evidenceGate.CIJob != gate.WorkflowJobID {
			return fmt.Errorf(
				"capability gate %s is owned by CI job %s, DR-06 contract uses %s",
				evidenceGate.ID, evidenceGate.CIJob, gate.WorkflowJobID,
			)
		}
		if evidenceGate.ScenarioID != "" &&
			!strings.Contains(gate.Command, "-Scenario "+evidenceGate.ScenarioID) {
			return fmt.Errorf(
				"capability gate %s scenario %s is absent from the DR-06 command",
				evidenceGate.ID, evidenceGate.ScenarioID,
			)
		}
		if evidenceGate.ScenarioID != "" &&
			!gateEnvironmentMatchesCatalogue(gate.Environment, evidenceGate.RequiredEnvironment) {
			return fmt.Errorf(
				"capability gate %s environment %q does not match DR-06 environment %q",
				evidenceGate.ID, evidenceGate.RequiredEnvironment, gate.Environment,
			)
		}
	}
	return nil
}

func gateEnvironmentMatchesCatalogue(description, required string) bool {
	description = strings.ToLower(description)
	switch required {
	case "linux-container":
		return strings.Contains(description, "linux test container") ||
			strings.Contains(description, "linux container")
	case "local":
		return strings.Contains(description, "local linux")
	default:
		return strings.Contains(description, strings.ToLower(required))
	}
}

func validatePinnedGateMaterials(root string) error {
	materialsJSON, err := os.ReadFile(filepath.Join(root, "scripts", "release", "materials.json"))
	if err != nil {
		return fmt.Errorf("read release materials: %w", err)
	}
	var materials struct {
		Images []struct {
			Name      string `json:"name"`
			Reference string `json:"reference"`
		} `json:"images"`
	}
	if err := json.Unmarshal(materialsJSON, &materials); err != nil {
		return fmt.Errorf("parse release materials: %w", err)
	}
	references := map[string]string{}
	for _, image := range materials.Images {
		references[image.Name] = image.Reference
	}
	for _, name := range []string{"go", "debian", "powershell"} {
		if references[name] == "" || !strings.Contains(references[name], "@sha256:") {
			return fmt.Errorf("immutable material reference is missing: %s", name)
		}
	}
	checks := []struct {
		path      string
		reference string
	}{
		{"tests/ci/security-gate.ps1", references["go"]},
		{"tests/ci/native-install-gate.ps1", references["go"]},
		{"tests/fixtures/native-systemd.Dockerfile", "FROM " + references["debian"]},
		{"deploy/docker/images/test-runner.Dockerfile", references["go"]},
		{"deploy/docker/images/test-runner.Dockerfile", references["powershell"]},
	}
	for _, check := range checks {
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(check.path)))
		if err != nil {
			return fmt.Errorf("read gate material consumer %s: %w", check.path, err)
		}
		if !bytes.Contains(body, []byte(check.reference)) {
			return fmt.Errorf("%s does not use pinned material %s", check.path, check.reference)
		}
	}
	return nil
}

func documentMapping(document *yaml.Node) (*yaml.Node, error) {
	if document.Kind != yaml.DocumentNode || len(document.Content) != 1 ||
		document.Content[0].Kind != yaml.MappingNode {
		return nil, errors.New("CI workflow root must be a mapping")
	}
	return document.Content[0], nil
}

func mappingValue(mapping *yaml.Node, key string) (*yaml.Node, bool) {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil, false
	}
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		if mapping.Content[index].Value == key {
			return mapping.Content[index+1], true
		}
	}
	return nil, false
}

func mappingPath(mapping *yaml.Node, keys ...string) (*yaml.Node, error) {
	current := mapping
	for _, key := range keys {
		next, ok := mappingValue(current, key)
		if !ok || next.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("CI workflow mapping is missing: %s", strings.Join(keys, "."))
		}
		current = next
	}
	return current, nil
}

func scalarValue(mapping *yaml.Node, key string) (string, error) {
	value, ok := mappingValue(mapping, key)
	if !ok || value.Kind != yaml.ScalarNode {
		return "", fmt.Errorf("CI workflow scalar is missing: %s", key)
	}
	return value.Value, nil
}

func workflowNeeds(job *yaml.Node) ([]string, error) {
	value, ok := mappingValue(job, "needs")
	if !ok {
		return nil, nil
	}
	switch value.Kind {
	case yaml.ScalarNode:
		return []string{value.Value}, nil
	case yaml.SequenceNode:
		result := make([]string, 0, len(value.Content))
		for _, item := range value.Content {
			if item.Kind != yaml.ScalarNode {
				return nil, errors.New("needs must contain job IDs")
			}
			result = append(result, item.Value)
		}
		return result, nil
	default:
		return nil, errors.New("needs must be a scalar or sequence")
	}
}

func workflowSteps(job *yaml.Node) ([]string, []string, error) {
	steps, ok := mappingValue(job, "steps")
	if !ok || steps.Kind != yaml.SequenceNode {
		return nil, nil, errors.New("steps must be a sequence")
	}
	var runs []string
	var uploadPaths []string
	for _, step := range steps.Content {
		if step.Kind != yaml.MappingNode {
			return nil, nil, errors.New("step must be a mapping")
		}
		if run, ok := mappingValue(step, "run"); ok && run.Kind == yaml.ScalarNode {
			runs = append(runs, run.Value)
		}
		uses, hasUses := mappingValue(step, "uses")
		if !hasUses || uses.Kind != yaml.ScalarNode || !strings.Contains(uses.Value, "actions/upload-artifact@") {
			continue
		}
		with, ok := mappingValue(step, "with")
		if !ok || with.Kind != yaml.MappingNode {
			continue
		}
		retention, ok := mappingValue(with, "retention-days")
		if ok && retention.Kind == yaml.ScalarNode && retention.Value == "90" {
			path, ok := mappingValue(with, "path")
			if !ok || path.Kind != yaml.ScalarNode {
				continue
			}
			for _, line := range strings.Split(path.Value, "\n") {
				if trimmed := strings.TrimSpace(line); trimmed != "" {
					uploadPaths = append(uploadPaths, strings.TrimSuffix(trimmed, "/"))
				}
			}
		}
	}
	return runs, uploadPaths, nil
}

func pathCoveredByUpload(path string, uploadPaths []string) bool {
	path = strings.TrimSuffix(filepath.ToSlash(path), "/")
	for _, uploadPath := range uploadPaths {
		uploadPath = strings.TrimSuffix(filepath.ToSlash(uploadPath), "/")
		if path == uploadPath || strings.HasPrefix(path, uploadPath+"/") {
			return true
		}
	}
	return false
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	leftCopy, rightCopy := append([]string(nil), left...), append([]string(nil), right...)
	sort.Strings(leftCopy)
	sort.Strings(rightCopy)
	return strings.Join(leftCopy, "\x00") == strings.Join(rightCopy, "\x00")
}
