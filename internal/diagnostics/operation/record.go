// Package operation owns long-running operation state and history.
// It does not own executing product operations.
package operation

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	Pending    = "pending"
	Running    = "running"
	Recovering = "recovering"
	Completed  = "completed"
	Failed     = "failed"
	Abandoned  = "abandoned"
)

type Record struct {
	ID             string     `json:"id"`
	Kind           string     `json:"kind"`
	State          string     `json:"state"`
	Domain         string     `json:"domain"`
	Resource       string     `json:"resource,omitempty"`
	Reason         string     `json:"reason,omitempty"`
	Recoverable    bool       `json:"recoverable"`
	RecoveryAction string     `json:"recovery_action,omitempty"`
	StartedAt      time.Time  `json:"started_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	FinishedAt     *time.Time `json:"finished_at,omitempty"`
}

func Clone(in []Record) []Record {
	if len(in) == 0 {
		return nil
	}
	out := make([]Record, len(in))
	copy(out, in)
	return out
}

func New(kind, domain, resource string, recoverable bool, recoveryAction string, now time.Time) Record {
	return Record{
		ID:             fmt.Sprintf("%s-%d", strings.ReplaceAll(kind, ".", "-"), now.UnixNano()),
		Kind:           kind,
		State:          Running,
		Domain:         domain,
		Resource:       resource,
		Recoverable:    recoverable,
		RecoveryAction: recoveryAction,
		StartedAt:      now,
		UpdatedAt:      now,
	}
}

func Transition(item Record, state, reason string, now time.Time) Record {
	item.State = state
	item.Reason = reason
	item.UpdatedAt = now
	if !IsOpen(state) {
		item.FinishedAt = &now
	}
	return item
}

func MarkRecovering(item Record, reason string, now time.Time) Record {
	item.State = Recovering
	if item.Reason == "" {
		item.Reason = reason
	}
	item.UpdatedAt = now
	return item
}

func IsOpen(state string) bool {
	switch state {
	case Pending, Running, Recovering:
		return true
	default:
		return false
	}
}

func PendingItems(items map[string]Record) []Record {
	out := make([]Record, 0)
	for _, item := range items {
		if IsOpen(item.State) {
			out = append(out, item)
		}
	}
	sortRecords(out)
	return out
}

func Records(items map[string]Record) []Record {
	out := make([]Record, 0, len(items))
	for _, item := range items {
		out = append(out, item)
	}
	sortRecords(out)
	return out
}

func Normalize(item Record, now time.Time) (Record, bool) {
	changed := false
	switch item.State {
	case Pending, Running, Recovering, Completed, Failed, Abandoned:
	default:
		item.State = Recovering
		if item.Reason == "" {
			item.Reason = "invalid persisted operation state"
		}
		changed = true
	}
	if item.StartedAt.IsZero() {
		switch {
		case !item.UpdatedAt.IsZero():
			item.StartedAt = item.UpdatedAt
		case item.FinishedAt != nil && !item.FinishedAt.IsZero():
			item.StartedAt = *item.FinishedAt
		default:
			item.StartedAt = now
		}
		changed = true
	}
	if item.UpdatedAt.IsZero() {
		item.UpdatedAt = item.StartedAt
		changed = true
	}
	if item.ID == "" {
		item.ID = recoveredID(item)
		changed = true
	}
	return item, changed
}

func Compact(items []Record, maxClosed int) []Record {
	if len(items) == 0 {
		return nil
	}
	if maxClosed < 0 {
		maxClosed = 0
	}

	openItems := make([]Record, 0, len(items))
	closedItems := make([]Record, 0, len(items))
	for _, item := range items {
		if IsOpen(item.State) {
			openItems = append(openItems, item)
			continue
		}
		closedItems = append(closedItems, item)
	}

	if len(closedItems) > maxClosed {
		sort.Slice(closedItems, func(i, j int) bool {
			left := closedSortTime(closedItems[i])
			right := closedSortTime(closedItems[j])
			if left.Equal(right) {
				return closedItems[i].ID < closedItems[j].ID
			}
			return left.After(right)
		})
		closedItems = append([]Record(nil), closedItems[:maxClosed]...)
	}

	out := make([]Record, 0, len(openItems)+len(closedItems))
	out = append(out, openItems...)
	out = append(out, closedItems...)
	sortRecords(out)
	return out
}

func closedSortTime(item Record) time.Time {
	if item.FinishedAt != nil && !item.FinishedAt.IsZero() {
		return *item.FinishedAt
	}
	if !item.UpdatedAt.IsZero() {
		return item.UpdatedAt
	}
	return item.StartedAt
}

func sortRecords(items []Record) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].StartedAt.Equal(items[j].StartedAt) {
			return items[i].ID < items[j].ID
		}
		return items[i].StartedAt.Before(items[j].StartedAt)
	})
}

func recoveredID(item Record) string {
	return fmt.Sprintf(
		"recovered-%s-%s-%s-%d",
		sanitizeIDPart(item.Kind),
		sanitizeIDPart(item.Domain),
		sanitizeIDPart(item.Resource),
		item.StartedAt.UnixNano(),
	)
}

func sanitizeIDPart(in string) string {
	in = strings.TrimSpace(strings.ToLower(in))
	if in == "" {
		return "unknown"
	}
	in = strings.ReplaceAll(in, ".", "-")
	in = strings.ReplaceAll(in, "/", "-")
	in = strings.ReplaceAll(in, "\\", "-")
	in = strings.ReplaceAll(in, " ", "-")
	return in
}
