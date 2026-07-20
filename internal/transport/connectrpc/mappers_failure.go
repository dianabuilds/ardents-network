package connectrpc

import (
	diagapi "ardents/internal/diagnostics/api"
	ardentsv1 "ardents/proto/ardents/v1"
)

func toFailureExplanationSnapshot(in diagapi.FailureExplanationSnapshot) *ardentsv1.FailureExplanationSnapshot {
	out := &ardentsv1.FailureExplanationSnapshot{
		Scope:      in.Scope,
		ResourceId: in.ResourceID,
		State:      in.State,
		Impact:     in.Impact,
		Recovery:   in.Recovery,
		NextSteps:  append([]string(nil), in.NextSteps...),
	}
	if in.Reason != nil {
		out.Reason = toReasonSnapshot(*in.Reason)
	}
	return out
}
