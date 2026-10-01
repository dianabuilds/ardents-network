//go:build linux

package main

import (
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/streamqualification"
)

func TestIssue60CleanupEvidenceRequiresEveryJoinedOwner(t *testing.T) {
	path := writeQualificationJSON(t, cleanupResultsInput{Schema: "ardents-qualification-cleanup-v1", Owners: []cleanupOwnerInput{
		{Role: "reader", ActiveState: "inactive", Result: "success", Passed: true},
		{Role: "publisher", ActiveState: "inactive", Result: "success", Passed: true},
	}})
	criteria, err := readCleanupResults(path)
	if err != nil || !streamqualification.CriteriaPassed(criteria) {
		t.Fatalf("complete joined cleanup refused: %+v / %v", criteria, err)
	}
	path = writeQualificationJSON(t, cleanupResultsInput{Schema: "ardents-qualification-cleanup-v1", Owners: []cleanupOwnerInput{
		{Role: "reader", ActiveState: "inactive", Result: "success", Passed: true, ActiveWorkers: []string{"worker.service"}},
		{Role: "publisher", ActiveState: "inactive", Result: "success", Passed: true},
	}})
	if criteria, err := readCleanupResults(path); err == nil || streamqualification.CriteriaPassed(criteria) {
		t.Fatal("cleanup with an active worker qualified")
	}
}

func TestIssue60AssuranceRefusesMissingNamedEvidence(t *testing.T) {
	names := []string{
		"reader-provider-period-bound", "publisher-provider-period-bound", "reader-owner-hosting-reconciliation", "publisher-owner-hosting-reconciliation",
		"paired-endpoint-set", "route-node-owner-set", "state-source-owner-set", "whole-owner-slice-set",
		"reader-joined-cleanup", "publisher-joined-cleanup", "joined-cleanup-owner-set",
		"paired-retained-streams", "reader-endpoint-carrier-ratio", "publisher-endpoint-carrier-ratio", "paired-directional-useful-bytes",
		"reader-complete-owner-RSS-bytes", "publisher-complete-owner-RSS-bytes", "reader-complete-owner-CPU-percent-one-core", "publisher-complete-owner-CPU-percent-one-core",
		"reader-whole-owner-slice-RSS-bytes", "publisher-whole-owner-slice-RSS-bytes", "reader-whole-owner-slice-CPU-percent-one-core",
		"publisher-whole-owner-slice-CPU-percent-one-core", "relay-one-second-workload-window",
	}
	criteria := make([]streamqualification.Criterion, 0, len(names))
	for _, name := range names {
		criteria = append(criteria, streamqualification.Criterion{Name: name, Passed: true})
	}
	verdicts := issue60Evidence(criteria)
	if len(verdicts) != 3 {
		t.Fatalf("assurance count=%d", len(verdicts))
	}
	for _, verdict := range verdicts {
		if !verdict.Passed || len(verdict.Criteria) == 0 || len(verdict.Evidence) == 0 {
			t.Fatalf("complete assurance refused: %+v", verdict)
		}
	}
	criteria = criteria[:len(criteria)-1]
	for _, verdict := range issue60Evidence(criteria) {
		if verdict.ID == "P8" && verdict.Passed {
			t.Fatal("P8 accepted missing named evidence")
		}
	}
}

func TestQualificationEndpointCgroupRequiresExactUnifiedRecord(t *testing.T) {
	exact := "0::" + qualificationOwnerControlGroup + "/ardents-endpoint.service"
	if !inQualificationEndpointCgroup(exact) {
		t.Fatal("exact Endpoint cgroup was refused")
	}
	for _, changed := range []string{"0::/foreign" + qualificationOwnerControlGroup + "/ardents-endpoint.service", exact + "/child", "1:name=" + exact} {
		if inQualificationEndpointCgroup(changed) {
			t.Fatalf("non-exact Endpoint cgroup accepted: %q", changed)
		}
	}
}
func TestWholeOwnerSliceEvidenceRequiresEffectiveLimitsAndCompleteWindow(t *testing.T) {
	owners := map[streamqualification.Role]ownerNetworkVerdict{
		streamqualification.ReaderRole:    {Started: time.Unix(1000, 0).UTC(), Stopped: time.Unix(1597, 0).UTC()},
		streamqualification.PublisherRole: {Started: time.Unix(1000, 0).UTC(), Stopped: time.Unix(1597, 0).UTC()},
	}
	inputs := qualificationOwnerSliceInputs()
	evidence, criteria := evaluateOwnerSlices(inputs, owners)
	if len(evidence) != 2 || !streamqualification.CriteriaPassed(criteria) {
		t.Fatalf("effective whole-owner slices refused: %+v / %+v", evidence, criteria)
	}
	inputs[0].CPUMax = "max 100000"
	if _, criteria := evaluateOwnerSlices(inputs, owners); streamqualification.CriteriaPassed(criteria) {
		t.Fatal("unbounded reader CPU slice qualified")
	}
	inputs = qualificationOwnerSliceInputs()
	inputs[1].Samples = inputs[1].Samples[:100]
	if _, criteria := evaluateOwnerSlices(inputs, owners); streamqualification.CriteriaPassed(criteria) {
		t.Fatal("incomplete publisher slice sampling qualified")
	}
	inputs = qualificationOwnerSliceInputs()
	inputs[0].Receipt = []string{strings.ReplaceAll(strings.Join(inputs[0].Receipt, "\n"), "IPAccounting=yes", "IPAccounting=no")}
	if _, criteria := evaluateOwnerSlices(inputs, owners); streamqualification.CriteriaPassed(criteria) {
		t.Fatal("slice without effective IP accounting qualified")
	}
}
