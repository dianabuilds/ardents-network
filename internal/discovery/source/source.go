package source

import (
	"time"

	discoveryrecord "ardents/internal/discovery/record"
)

const (
	Local    = "local"
	Imported = "imported"
	Network  = "network"
)

func LocalEntries(entries []discoveryrecord.Entry) []discoveryrecord.Entry {
	out := make([]discoveryrecord.Entry, 0, len(entries))
	for _, item := range entries {
		if item.Source != Local {
			continue
		}
		out = append(out, item)
	}
	return out
}

func RefreshSeenAt(entries []discoveryrecord.Entry, now time.Time) []discoveryrecord.Entry {
	out := make([]discoveryrecord.Entry, 0, len(entries))
	for _, item := range entries {
		item.SeenAt = now
		out = append(out, item)
	}
	return out
}
