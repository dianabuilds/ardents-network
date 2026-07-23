package records

import (
	"time"
)

const (
	Local     = "local"
	Imported  = "imported"
	Network   = "network"
	Bootstrap = "bootstrap"
)

func ValidSource(source string) bool {
	return source == Local || source == Imported || source == Network || source == Bootstrap
}

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
