package records

import (
	"time"
)

const (
	Local    = "local"
	Imported = "imported"
	Network  = "network"
)

func LocalEntries(entries []Entry) []Entry {
	out := make([]Entry, 0, len(entries))
	for _, item := range entries {
		if item.Source != Local {
			continue
		}
		out = append(out, item)
	}
	return out
}

func RefreshSeenAt(entries []Entry, now time.Time) []Entry {
	out := make([]Entry, 0, len(entries))
	for _, item := range entries {
		item.SeenAt = now
		out = append(out, item)
	}
	return out
}
