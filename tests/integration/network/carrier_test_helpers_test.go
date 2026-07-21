//go:build integration

package network_test

import (
	"ardents/internal/messaging"
	"ardents/internal/network"
)

func carrierEnvelope(in messaging.SealedEnvelope) network.Envelope {
	return network.Envelope{PubsubTopic: in.PubsubTopic, ContentTopic: in.ContentTopic, Payload: in.Payload}
}

func sealedEnvelope(in network.Envelope) messaging.SealedEnvelope {
	return messaging.SealedEnvelope{PubsubTopic: in.PubsubTopic, ContentTopic: in.ContentTopic, Payload: in.Payload}
}
