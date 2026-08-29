package publiccontrol

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestInspectDeclaredEvidenceAlwaysRetainsExternalIndependenceGate(t *testing.T) {
	report, err := Inspect([]byte(validManifest()))
	if err != nil {
		t.Fatal(err)
	}
	if !hasOutcome(report, OutcomeExternalEvidenceRequired) {
		t.Fatalf("outcomes = %#v, want external evidence requirement", report.Outcomes)
	}
	if len(report.Outcomes) != 1 || report.Qualified {
		t.Fatalf("outcomes = %#v qualified = %t, want only external gate", report.Outcomes, report.Qualified)
	}
}

func TestInspectDeclaredEvidenceRejectsSharedCustodyBoundary(t *testing.T) {
	raw := strings.Replace(validManifest(), `"organization":"custodian-organization-5"`, `"organization":"custodian-organization-1"`, 1)
	if strings.Contains(raw, `"organization":"custodian-organization-5"`) {
		t.Fatal("test manifest did not replace the fifth custody organization")
	}
	report, err := Inspect([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if !hasOutcome(report, OutcomeIndependenceConflict) {
		t.Fatalf("outcomes = %#v, want independence conflict", report.Outcomes)
	}
}

func TestInspectDeclaredEvidenceReportsMissingBuilderAttestation(t *testing.T) {
	manifest := strings.Replace(validManifest(), fmt.Sprintf(`,{"builder":"builder-2","artifact":"sha256:%s"}`, digest('f')), "", 1)
	report, err := Inspect([]byte(manifest))
	if err != nil {
		t.Fatal(err)
	}
	if !hasOutcome(report, OutcomeUnavailable) {
		t.Fatalf("outcomes = %#v, want unavailable", report.Outcomes)
	}
}

func TestInspectDeclaredEvidenceRejectsCustodyWithoutAuthorityKey(t *testing.T) {
	manifest := strings.Replace(validManifest(), fmt.Sprintf(`,"public_key":"ed25519:%s"`, digest('0')), "", 1)
	report, err := Inspect([]byte(manifest))
	if err != nil {
		t.Fatal(err)
	}
	if !hasOutcome(report, OutcomeForged) {
		t.Fatalf("outcomes = %#v, want missing custody authority key rejected", report.Outcomes)
	}
}

func TestInspectAtClassifiesPublicControlLifecycleFailures(t *testing.T) {
	for name, test := range map[string]struct {
		manifest func(string) string
		at       time.Time
		floor    uint64
		want     Outcome
	}{
		"stale":    {func(value string) string { return value }, time.Date(2031, time.January, 1, 0, 0, 0, 0, time.UTC), 0, OutcomeStale},
		"replayed": {func(value string) string { return value }, time.Date(2029, time.January, 1, 0, 0, 0, 0, time.UTC), 2, OutcomeReplayed},
		"revoked":  {func(value string) string { return strings.Replace(value, `"revoked":false`, `"revoked":true`, 1) }, time.Date(2029, time.January, 1, 0, 0, 0, 0, time.UTC), 0, OutcomeRevoked},
		"conflicting": {func(value string) string {
			return strings.Replace(value, `"conflicting":false`, `"conflicting":true`, 1)
		}, time.Date(2029, time.January, 1, 0, 0, 0, 0, time.UTC), 0, OutcomeConflicting},
	} {
		t.Run(name, func(t *testing.T) {
			report, err := InspectAt([]byte(test.manifest(validManifest())), InspectionConfig{At: test.at, AuditFloorGeneration: test.floor})
			if err != nil {
				t.Fatal(err)
			}
			if !hasOutcome(report, test.want) {
				t.Fatalf("outcomes = %#v, want %q", report.Outcomes, test.want)
			}
		})
	}
}

func TestInspectAtRejectsUnexpectedPredecessor(t *testing.T) {
	report, err := InspectAt([]byte(validManifest()), InspectionConfig{ExpectedPredecessor: "sha256:" + digest('a')})
	if err != nil {
		t.Fatal(err)
	}
	if !hasOutcome(report, OutcomeReplayed) {
		t.Fatalf("outcomes = %#v, want predecessor mismatch as replayed", report.Outcomes)
	}
}

func TestInspectRejectsUnknownAndTrailingManifestData(t *testing.T) {
	for name, raw := range map[string]string{
		"unknown field": strings.Replace(validManifest(), `"candidate":"sha256:`+digest('f')+`"`, `"candidate":"sha256:`+digest('f')+`","unexpected":true`, 1),
		"trailing data": validManifest() + "\n{}",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Inspect([]byte(raw)); err == nil {
				t.Fatal("Inspect accepted non-canonical evidence manifest")
			}
		})
	}
}

func hasOutcome(report Report, expected Outcome) bool {
	for _, outcome := range report.Outcomes {
		if outcome == expected {
			return true
		}
	}
	return false
}

func validManifest() string {
	return fmt.Sprintf(`{
"schema":"ardents-public-control-evidence-v1",
"candidate":"sha256:%s",
"transition":{"generation":1,"predecessor":"sha256:%s","not_after":"2030-01-01T00:00:00Z","revoked":false,"conflicting":false},
"custody":{"threshold":3,"emergency_threshold":4,"members":[
%s]},
"candidate_view":{"epoch":"sha256:%s","input_log":"sha256:%s","materialization_rules":"sha256:%s","audits":[
{"auditor":"auditor-1","input_log":"sha256:%s","output":"sha256:%s"},
{"auditor":"auditor-2","input_log":"sha256:%s","output":"sha256:%s"}]},
"builders":[%s],
"auditors":[%s],
"packages":[{"artifact":"sha256:%s","source":"sha256:%s","dependencies":"sha256:%s","recipe":"sha256:%s","sbom":"sha256:%s","qualification":"sha256:%s","builder_attestations":[
{"builder":"builder-1","artifact":"sha256:%s"},{"builder":"builder-2","artifact":"sha256:%s"}]}]
}`,
		digest('f'), digest('0'), actors("custodian", 5), digest('a'), digest('b'), digest('c'), digest('b'), digest('d'), digest('b'), digest('e'),
		actors("builder", 2), actors("auditor", 2), digest('f'), digest('a'), digest('b'), digest('c'), digest('d'), digest('e'), digest('f'), digest('f'))
}

func actors(role string, count int) string {
	values := make([]string, count)
	for index := range count {
		keyFill := byte('0' + index)
		if role == "builder" {
			keyFill = byte('a' + index)
		}
		if role == "auditor" {
			keyFill = byte('c' + index)
		}
		values[index] = fmt.Sprintf(`{"id":"%s-%d","public_key":"ed25519:%s","operator":"%s-operator-%d","organization":"%s-organization-%d","administration":"%s-administration-%d","evidence":"sha256:%s"}`,
			role, index+1, digest(keyFill), role, index+1, role, index+1, role, index+1, digest(byte('a'+index)))
	}
	return strings.Join(values, ",")
}

func digest(fill byte) string {
	return strings.Repeat(string(fill), 64)
}
