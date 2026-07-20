package lifecycle

import nodeapi "ardents/internal/node/api"

func APISnapshot(in Snapshot) nodeapi.LifecycleSnapshot {
	out := nodeapi.LifecycleSnapshot{
		Current:        in.Current,
		Previous:       in.Previous,
		EnteredAt:      in.EnteredAt,
		TransitionedAt: in.TransitionedAt,
	}
	if len(in.Transitions) > 0 {
		out.Transitions = make([]nodeapi.LifecycleTransitionSnapshot, 0, len(in.Transitions))
		for _, item := range in.Transitions {
			out.Transitions = append(out.Transitions, nodeapi.LifecycleTransitionSnapshot{
				From: item.From,
				To:   item.To,
				At:   item.At,
			})
		}
	}
	return out
}
