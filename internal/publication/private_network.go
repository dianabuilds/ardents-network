package publication

import (
	"context"
	"encoding/json"

	"ardents/internal/discovery"
	discoverysource "ardents/internal/discovery/records"
	networkprivacy "ardents/internal/messaging"
	transport "ardents/internal/network"
)

func PublishPrivateDiscoveryEntries(ctx context.Context, entries []discovery.Entry, channel *networkprivacy.Channel, carrier networkprivacy.Carrier) error {
	if channel == nil || carrier == nil {
		return networkprivacy.CapabilityUnavailable()
	}
	published := 0
	for _, entry := range entries {
		if entry.Source != discoverysource.Local || entry.Record.RecordID() == "" {
			continue
		}
		payload, err := json.Marshal(entry.Record)
		if err != nil {
			return err
		}
		envelope, err := channel.Seal(networkprivacy.MessageClassDiscoveryRecord, 1, payload)
		if err != nil {
			return err
		}
		if err := carrier.PublishPrivateEnvelope(ctx, envelope); err != nil {
			return &transport.DiscoveryPublishError{Published: published, Err: err}
		}
		published++
	}
	return nil
}
