package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const requirementCoveragePath = "docs/qa/requirements-coverage.json"

var mandatoryCoverageSources = []string{
	"docs/system-properties.md",
	"docs/reference-invariants.md",
	"docs/communication-contracts.md",
	"docs/domains/data-substrate.md",
	"docs/domains/diagnostics.md",
	"docs/domains/discovery.md",
	"docs/domains/hosted-services.md",
	"docs/domains/identity.md",
	"docs/domains/network-foundation-messaging.md",
	"docs/domains/node-runtime.md",
	"docs/domains/policy.md",
	"docs/domains/publication.md",
	"docs/domains/workload-control.md",
	"docs/network-privacy-requirements.md",
	"docs/persistent-state-security.md",
	"docs/operator-access-contract.md",
	"docs/operator-configuration-contract.md",
	"docs/workload-and-services-requirements.md",
	"docs/data-substrate-requirements.md",
	"docs/production-observability-contract.md",
	"docs/deployment-contract.md",
}

func collectRequirementCoverage(scenarios []scenarioDoc) ([]inventoryRequirement, error) {
	raw, err := os.ReadFile(requirementCoveragePath)
	if err != nil {
		return nil, fmt.Errorf("read requirement coverage: %w", err)
	}
	var manifest requirementCoverageFile
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return nil, fmt.Errorf("decode requirement coverage: %w", err)
	}
	if manifest.Version != 1 {
		return nil, fmt.Errorf("unsupported requirement coverage version %d", manifest.Version)
	}
	scenarioIDs := map[string]bool{}
	for _, scenario := range scenarios {
		scenarioIDs[scenario.ScenarioID] = true
	}
	seenIDs := map[string]bool{}
	coveredSources := map[string]bool{}
	for index := range manifest.Requirements {
		requirement := &manifest.Requirements[index]
		requirement.Source = filepath.ToSlash(requirement.Source)
		coveredSources[requirement.Source] = true
		validateRequirement(requirement, seenIDs, scenarioIDs)
	}
	for _, source := range mandatoryCoverageSources {
		if !coveredSources[source] {
			manifest.Requirements = append(manifest.Requirements, inventoryRequirement{
				ID: "missing-source:" + source, Source: source, Status: "missing",
				Issues: []string{"mandatory source has no requirement coverage entry"},
			})
		}
	}
	return manifest.Requirements, nil
}

func validateRequirement(requirement *inventoryRequirement, seenIDs map[string]bool, scenarioIDs map[string]bool) {
	if requirement.ID == "" {
		requirement.Issues = append(requirement.Issues, "requirement id is empty")
	} else if seenIDs[requirement.ID] {
		requirement.Issues = append(requirement.Issues, "duplicate requirement id")
	}
	seenIDs[requirement.ID] = true
	content, err := os.ReadFile(requirement.Source)
	if err != nil {
		requirement.Issues = append(requirement.Issues, "requirement source does not exist")
	} else if requirement.Section == "" || !strings.Contains(string(content), requirement.Section) {
		requirement.Issues = append(requirement.Issues, "requirement section is missing from source")
	}
	for _, scenarioID := range requirement.Scenarios {
		if !scenarioIDs[scenarioID] {
			requirement.Issues = append(requirement.Issues, "unknown scenario evidence: "+scenarioID)
		}
	}
	for _, evidence := range requirement.StaticEvidence {
		path := strings.SplitN(evidence, "::", 2)[0]
		if _, err := os.Stat(path); err != nil {
			requirement.Issues = append(requirement.Issues, "static evidence does not exist: "+evidence)
		}
	}
	switch requirement.Status {
	case "covered":
		if len(requirement.Scenarios) == 0 && len(requirement.StaticEvidence) == 0 {
			requirement.Issues = append(requirement.Issues, "covered requirement has no evidence")
		}
	case "blocked":
		if strings.TrimSpace(requirement.BlockedReason) == "" {
			requirement.Issues = append(requirement.Issues, "blocked requirement has no reason")
		}
	default:
		requirement.Issues = append(requirement.Issues, "requirement status must be covered or blocked")
	}
}

func accumulateRequirementSummaries(summary inventorySummary, requirements []inventoryRequirement) inventorySummary {
	for _, requirement := range requirements {
		summary.RequirementCount++
		summary.RequirementIssueCount += len(requirement.Issues)
		summary.IssueCount += len(requirement.Issues)
		if requirement.Status == "covered" {
			summary.CoveredRequirementCount++
		} else if requirement.Status == "blocked" {
			summary.BlockedRequirementCount++
		}
	}
	return summary
}
