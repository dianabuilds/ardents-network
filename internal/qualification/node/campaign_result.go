package node

import (
	"context"
	"errors"
	"fmt"
)

var errInvalidNodeCampaign = errors.New("node campaign evidence is invalid")

func invalidNodeCampaign(cause error) error {
	return fmt.Errorf("%w: %w", errInvalidNodeCampaign, cause)
}

// Result is the terminal machine outcome of one Node operation.
type Result struct {
	Verdict        string `json:"verdict"`
	Reason         string `json:"reason"`
	EvidenceRoot   string `json:"evidence_root,omitempty"`
	EvidenceDigest string `json:"evidence_digest,omitempty"`
}

func nodeCandidateFailure(message string, cause error) error {
	if errors.Is(cause, errInvalidNodeCampaign) || errors.Is(cause, context.Canceled) || errors.Is(cause, context.DeadlineExceeded) {
		return cause
	}
	return errors.New(message)
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
