//go:build linux

package main

import (
	"errors"
	"os"

	"github.com/dianabuilds/ardents-network/internal/application/streamqualification"
)

type issue60EvidenceVerdict struct {
	ID, Scope string
	Evidence  []string
	Criteria  []streamqualification.Criterion
	Passed    bool
}

type cleanupOwnerInput struct {
	Role, ActiveState, Result string
	MainPID                   uint64
	ExecMainStatus            int
	ActiveWorkers             []string
	Passed                    bool
}

type cleanupResultsInput struct {
	Schema string
	Owners []cleanupOwnerInput
}

func readCleanupResults(path string) ([]streamqualification.Criterion, error) {
	body, err := os.ReadFile(path)
	if err != nil || len(body) == 0 || len(body) > 64<<10 {
		return nil, errors.Join(err, errors.New("cleanup result input is invalid"))
	}
	var input cleanupResultsInput
	if err := decodeExact(body, &input); err != nil {
		return nil, err
	}
	criteria := make([]streamqualification.Criterion, 0, 3)
	seen := make(map[string]bool, 2)
	for _, owner := range input.Owners {
		clean := (owner.Role == "reader" || owner.Role == "publisher") && !seen[owner.Role] &&
			owner.ActiveState == "inactive" && owner.MainPID == 0 && owner.Result == "success" &&
			owner.ExecMainStatus == 0 && len(owner.ActiveWorkers) == 0 && owner.Passed
		seen[owner.Role] = true
		criteria = append(criteria, streamqualification.Criterion{
			Name: owner.Role + "-joined-cleanup", Observed: boolNumber(clean), Relation: "=", Bound: 1, Passed: clean,
		})
	}
	complete := input.Schema == "ardents-qualification-cleanup-v1" && len(input.Owners) == 2 && seen["reader"] && seen["publisher"]
	criteria = append(criteria, streamqualification.Criterion{Name: "joined-cleanup-owner-set", Observed: float64(len(seen)), Relation: "=", Bound: 2, Passed: complete})
	if !streamqualification.CriteriaPassed(criteria) {
		return criteria, errors.New("installed owner cleanup evidence is incomplete")
	}
	return criteria, nil
}

func issue60Evidence(criteria []streamqualification.Criterion) []issue60EvidenceVerdict {
	specs := []struct {
		id, scope string
		evidence  []string
		names     []string
	}{
		{"P5", "issue-60 measured provider-period byte debit and complete runtime owner set",
			[]string{"reader.jsonl", "publisher.jsonl", "node-results.json"},
			[]string{"reader-provider-period-bound", "publisher-provider-period-bound", "reader-owner-hosting-reconciliation", "publisher-owner-hosting-reconciliation", "paired-endpoint-set", "route-node-owner-set", "state-source-owner-set", "whole-owner-slice-set"}},
		{"P7", "issue-60 installed Endpoint and worker joined cleanup",
			[]string{"cleanup-results.json"},
			[]string{"reader-joined-cleanup", "publisher-joined-cleanup", "joined-cleanup-owner-set"}},
		{"P8", "issue-60 ten-minute progress, byte reconciliation and whole-owner resource bounds",
			[]string{"reader.jsonl", "publisher.jsonl", "relay-results.json", "node-results.json"},
			[]string{"paired-retained-streams", "reader-endpoint-carrier-ratio", "publisher-endpoint-carrier-ratio",
				"paired-directional-useful-bytes", "reader-complete-owner-RSS-bytes", "publisher-complete-owner-RSS-bytes",
				"reader-complete-owner-CPU-percent-one-core", "publisher-complete-owner-CPU-percent-one-core",
				"reader-whole-owner-slice-RSS-bytes", "publisher-whole-owner-slice-RSS-bytes",
				"reader-whole-owner-slice-CPU-percent-one-core", "publisher-whole-owner-slice-CPU-percent-one-core",
				"relay-one-second-workload-window"}},
	}
	byName := make(map[string][]streamqualification.Criterion, len(criteria))
	for _, criterion := range criteria {
		byName[criterion.Name] = append(byName[criterion.Name], criterion)
	}
	verdicts := make([]issue60EvidenceVerdict, 0, len(specs))
	for _, spec := range specs {
		selected := make([]streamqualification.Criterion, 0, len(spec.names))
		passed := true
		for _, name := range spec.names {
			matches := byName[name]
			if len(matches) != 1 {
				selected = append(selected, streamqualification.Criterion{Name: name, Relation: "present exactly once", Bound: 1, Passed: false})
				passed = false
				continue
			}
			selected = append(selected, matches[0])
			passed = passed && matches[0].Passed
		}
		verdicts = append(verdicts, issue60EvidenceVerdict{ID: spec.id, Scope: spec.scope, Evidence: spec.evidence, Criteria: selected, Passed: passed})
	}
	return verdicts
}
