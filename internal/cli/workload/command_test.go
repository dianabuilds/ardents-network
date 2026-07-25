package workload

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	commandctx "ardents/internal/cli/command"
	"ardents/internal/cli/output"
	ardentsv1 "ardents/internal/localapi/protocol"

	"google.golang.org/protobuf/encoding/protojson"
)

func TestRenderWorkloadCommandPreservesRejectedResponse(t *testing.T) {
	response := &ardentsv1.WorkloadCommandResponse{
		Status: &ardentsv1.OperationStatus{
			State:  "rejected",
			Reason: "workload transition is not accepted",
		},
		Workload: &ardentsv1.WorkloadStatusSnapshot{
			Spec: &ardentsv1.WorkloadSpecSnapshot{Id: "work.rejected"},
		},
	}

	for _, jsonOutput := range []bool{false, true} {
		name := "human"
		if jsonOutput {
			name = "json"
		}
		t.Run(name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			command := &Command{ctx: commandctx.Context{
				Renderer: output.NewRenderer(&stdout, &stderr, jsonOutput),
			}}

			if code := renderWorkloadCommand(command, "workload start", response); code != 1 {
				t.Fatalf("exit code = %d, want 1", code)
			}
			if jsonOutput {
				var preserved ardentsv1.WorkloadCommandResponse
				if err := protojson.Unmarshal(stdout.Bytes(), &preserved); err != nil {
					t.Fatalf("stdout is not the preserved response: %v\n%s", err, stdout.String())
				}
				if preserved.GetWorkload().GetSpec().GetId() != "work.rejected" {
					t.Fatalf("preserved workload = %q", preserved.GetWorkload().GetSpec().GetId())
				}
				var failure map[string]any
				if err := json.Unmarshal(stderr.Bytes(), &failure); err != nil {
					t.Fatalf("stderr is not the common JSON error object: %v\n%s", err, stderr.String())
				}
				if !strings.Contains(failure["message"].(string), "mutation response rejected") {
					t.Fatalf("unexpected rejection: %#v", failure)
				}
				return
			}
			for _, expected := range []string{
				"workload start",
				"status: rejected",
				"reason: workload transition is not accepted",
				"workload: work.rejected",
			} {
				if !strings.Contains(stdout.String(), expected) {
					t.Fatalf("stdout missing %q:\n%s", expected, stdout.String())
				}
			}
			if !strings.Contains(stderr.String(), "mutation response rejected") {
				t.Fatalf("unexpected rejection:\n%s", stderr.String())
			}
		})
	}
}
