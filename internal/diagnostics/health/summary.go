package health

import (
	"sort"
	"time"
)

const (
	Ready    = "ready"
	Degraded = "degraded"
	Failed   = "failed"
)

type SubsystemStatus struct {
	Domain    string    `json:"domain"`
	State     string    `json:"state"`
	Reason    *Reason   `json:"reason,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Summary struct {
	State         string            `json:"state"`
	PrimaryReason *Reason           `json:"primary_reason,omitempty"`
	Subsystems    []SubsystemStatus `json:"subsystems,omitempty"`
	UpdatedAt     time.Time         `json:"updated_at"`
}

func CloneSubsystem(in SubsystemStatus) SubsystemStatus {
	return SubsystemStatus{
		Domain:    in.Domain,
		State:     in.State,
		Reason:    Clone(in.Reason),
		UpdatedAt: in.UpdatedAt,
	}
}

func CloneSummary(in Summary) Summary {
	out := Summary{
		State:         in.State,
		PrimaryReason: Clone(in.PrimaryReason),
		UpdatedAt:     in.UpdatedAt,
	}
	if len(in.Subsystems) == 0 {
		return out
	}
	out.Subsystems = make([]SubsystemStatus, 0, len(in.Subsystems))
	for _, item := range in.Subsystems {
		out.Subsystems = append(out.Subsystems, CloneSubsystem(item))
	}
	return out
}

func Compose(now time.Time, primaryState string, primarySet bool, primary *Reason, subsystems map[string]SubsystemStatus) Summary {
	state := Ready
	currentPrimary := (*Reason)(nil)

	if primarySet && primary != nil {
		state = Degraded
		if primaryState == Failed {
			state = Failed
		}
		currentPrimary = Clone(primary)
	}

	items := make([]SubsystemStatus, 0, len(subsystems))
	for _, item := range subsystems {
		items = append(items, CloneSubsystem(item))
		if item.State == Failed {
			state = Failed
			if currentPrimary == nil && item.Reason != nil {
				currentPrimary = Clone(item.Reason)
			}
			continue
		}
		if state == Ready {
			state = Degraded
			if currentPrimary == nil && item.Reason != nil {
				currentPrimary = Clone(item.Reason)
			}
		}
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Domain < items[j].Domain
	})
	return Summary{
		State:         state,
		PrimaryReason: currentPrimary,
		Subsystems:    items,
		UpdatedAt:     now,
	}
}

func Restore(in Summary) (*Reason, bool, string, map[string]SubsystemStatus) {
	primary := Clone(in.PrimaryReason)
	primarySet := in.PrimaryReason != nil
	primaryState := ""
	if primarySet {
		primaryState = in.State
	}
	subsystems := map[string]SubsystemStatus{}
	for _, item := range in.Subsystems {
		subsystems[item.Domain] = CloneSubsystem(item)
	}
	return primary, primarySet, primaryState, subsystems
}
