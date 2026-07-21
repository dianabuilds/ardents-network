package diagnostics

import (
	"bytes"
	"strings"
	"testing"
	"time"

	ardentsv1 "ardents/internal/localapi/protocol"

	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestRenderDiagnosticsPendingHumanIncludesRecoveryContext(t *testing.T) {
	var out bytes.Buffer
	renderDiagnosticsPendingHuman(&out, &ardentsv1.PendingOperationsResponse{
		Status: &ardentsv1.OperationStatus{State: "degraded", Reason: "waiting"},
		Operations: []*ardentsv1.OperationSnapshot{{
			Id:             "op-1",
			Kind:           "transfer.fetch",
			Domain:         "data",
			Resource:       "blob-1",
			State:          "pending",
			Reason:         "awaiting remote source",
			Recoverable:    true,
			RecoveryAction: "retry after peer recovery",
			StartedAt:      timestamppb.New(time.Unix(100, 0)),
			UpdatedAt:      timestamppb.New(time.Unix(200, 0)),
		}},
	})

	text := out.String()
	for _, want := range []string{
		"diagnostics pending",
		"resource: blob-1",
		"recovery_action: retry after peer recovery",
		"started_at: 1970-01-01T00:01:40Z",
		"updated_at: 1970-01-01T00:03:20Z",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in output:\n%s", want, text)
		}
	}
}

func TestRenderDiagnosticsExplainHumanIncludesReasonDetailAndNextSteps(t *testing.T) {
	var out bytes.Buffer
	renderDiagnosticsExplainHuman(&out, &ardentsv1.FailureExplanationResponse{
		Status: &ardentsv1.OperationStatus{State: "failed", Reason: "policy rejected"},
		Explanation: &ardentsv1.FailureExplanationSnapshot{
			Scope:      "workload",
			ResourceId: "wl-1",
			State:      "failed",
			Reason: &ardentsv1.ReasonSnapshot{
				Summary:                "publication blocked",
				Detail:                 "runtime backing is missing",
				Impact:                 "service remains unpublished",
				Recovery:               "start workload and reconcile",
				OperatorActionRequired: true,
			},
			NextSteps: []string{"inspect workload", "reconcile publication"},
		},
	})

	text := out.String()
	for _, want := range []string{
		"diagnostics explain",
		"detail: runtime backing is missing",
		"impact: service remains unpublished",
		"recovery: start workload and reconcile",
		"operator_action_required: true",
		"next_steps: inspect workload, reconcile publication",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in output:\n%s", want, text)
		}
	}
}
