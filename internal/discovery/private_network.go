package discovery

import (
	"context"
	"encoding/json"
	"sort"

	discoveryfreshness "ardents/internal/discovery/freshness"
	discoverysource "ardents/internal/discovery/source"
	networkprivacy "ardents/internal/network/privacy"
)

type PrivateFetchResult struct {
	Entries  []Entry
	Rejected int
	Replayed int
	Reason   string
}

func FetchPrivateRecords(ctx context.Context, endpoints []string, channel *networkprivacy.Channel, carrier networkprivacy.Carrier) (PrivateFetchResult, error) {
	if channel == nil || carrier == nil {
		return PrivateFetchResult{Reason: networkprivacy.CodeCapabilityMissing}, nil
	}
	contentTopic, err := channel.StoreContentTopic()
	if err != nil {
		reason := networkprivacy.CodeOf(err)
		if reason == "" {
			reason = networkprivacy.CodeCapabilityMissing
		}
		return PrivateFetchResult{Reason: reason}, nil
	}
	envelopes, err := carrier.FetchPrivateEnvelopes(ctx, endpoints, contentTopic)
	if err != nil {
		return PrivateFetchResult{}, err
	}
	return decodePrivateRecords(envelopes, channel), nil
}

func decodePrivateRecords(envelopes []networkprivacy.SealedEnvelope, channel *networkprivacy.Channel) PrivateFetchResult {
	result := PrivateFetchResult{}
	seen := make(map[string]Entry)
	for _, envelope := range envelopes {
		opened, openErr := channel.Open(envelope)
		if openErr != nil {
			result.recordRejection(networkprivacy.CodeOf(openErr))
			continue
		}
		entry, ok := privateDiscoveryEntry(opened)
		if !ok {
			result.Rejected++
			result.Reason = networkprivacy.CodeEnvelopeMalformed
			continue
		}
		current, exists := seen[entry.Record.ID]
		if !exists || privateEntryNewer(entry, current) {
			seen[entry.Record.ID] = entry
		}
	}
	for _, entry := range seen {
		result.Entries = append(result.Entries, entry)
	}
	sort.Slice(result.Entries, func(i, j int) bool {
		return result.Entries[i].Record.ID < result.Entries[j].Record.ID
	})
	return result
}

func (r *PrivateFetchResult) recordRejection(code string) {
	if code == networkprivacy.CodeEnvelopeReplayed {
		r.Replayed++
		return
	}
	r.Rejected++
	if code == "" {
		code = networkprivacy.CodeEnvelopeMalformed
	}
	r.Reason = code
}

func privateDiscoveryEntry(opened networkprivacy.OpenedMessage) (Entry, bool) {
	if opened.Class != networkprivacy.MessageClassDiscoveryRecord || opened.PayloadVersion != 1 {
		return Entry{}, false
	}
	var record Record
	if err := json.Unmarshal(opened.Payload, &record); err != nil || record.ID == "" {
		return Entry{}, false
	}
	return Entry{Record: record, Source: discoverysource.Network, SeenAt: opened.IssuedAt}, true
}

func privateEntryNewer(candidate, current Entry) bool {
	candidateFreshness := discoveryfreshness.Score(candidate.Record)
	currentFreshness := discoveryfreshness.Score(current.Record)
	if candidateFreshness != currentFreshness {
		return candidateFreshness > currentFreshness
	}
	return candidate.SeenAt.After(current.SeenAt)
}
