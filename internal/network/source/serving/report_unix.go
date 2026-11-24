//go:build linux || darwin

package serving

import (
	"context"
	"encoding/json"
	"io"

	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
	"github.com/dianabuilds/ardents-network/internal/processresource"
)

// WriteFailureReport projects the original failure onto the bounded command report.
func WriteFailureReport(out io.Writer, phase string, operation, processClose error) error {
	facts := DescribeFailure(operation, processClose)
	raw, err := json.Marshal(struct {
		Operation string                        `json:"operation"`
		Phase     string                        `json:"phase"`
		Outcome   string                        `json:"outcome"`
		First     string                        `json:"first_owner,omitempty"`
		Cleanup   string                        `json:"cleanup_owner,omitempty"`
		Resource  *processresource.FailureFacts `json:"resource,omitempty"`
	}{"source.serve", phase, "failed", facts.FirstOwner, facts.CleanupOwners, facts.Resource})
	if err != nil {
		return err
	}
	return artifacts.WriteReport(context.Background(), out, append(raw, '\n'))
}
