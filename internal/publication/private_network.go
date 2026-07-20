package publication

import (
	"context"
	"encoding/json"

	discovery "ardents/internal/discovery"
	discoverysource "ardents/internal/discovery/source"
	transport "ardents/internal/network/api"
	networkprivacy "ardents/internal/network/privacy"
)

func PublishPrivateDiscoveryEntries(ctx context.Context, entries []discovery.Entry, channel *networkprivacy.Channel, carrier networkprivacy.Carrier) error {
	if channel == nil || carrier == nil {
		return networkprivacy.CapabilityUnavailable()
	}
	published := 0
	for _, entry := range entries {
		if entry.Source != discoverysource.Local || entry.Record.ID == "" {
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
