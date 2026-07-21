package daemon

import (
	"context"

	"ardents/internal/messaging"
	"ardents/internal/network"
)

type messagingCarrier struct{ network network.Service }

func (c messagingCarrier) PublishPrivateEnvelope(ctx context.Context, envelope messaging.SealedEnvelope) error {
	return c.network.PublishRelayEnvelope(ctx, carrierEnvelope(envelope))
}

func (c messagingCarrier) SubscribePrivateEnvelopes(ctx context.Context, contentTopic string) (<-chan messaging.SealedEnvelope, error) {
	items, err := c.network.SubscribeRelayEnvelopes(ctx, network.DefaultPubsubTopic, contentTopic)
	if err != nil {
		return nil, err
	}
	return sealedEnvelopes(ctx, items), nil
}

func (c messagingCarrier) FetchPrivateEnvelopes(ctx context.Context, endpoints []string, contentTopic string) ([]messaging.SealedEnvelope, error) {
	items, err := c.network.FetchEnvelopes(ctx, endpoints, contentTopic)
	if err != nil {
		return nil, err
	}
	out := make([]messaging.SealedEnvelope, 0, len(items))
	for _, item := range items {
		out = append(out, sealedEnvelope(item))
	}
	return out, nil
}

func carrierEnvelope(envelope messaging.SealedEnvelope) network.Envelope {
	return network.Envelope{PubsubTopic: envelope.PubsubTopic, ContentTopic: envelope.ContentTopic, Payload: envelope.Payload}
}

func sealedEnvelope(envelope network.Envelope) messaging.SealedEnvelope {
	return messaging.SealedEnvelope{PubsubTopic: envelope.PubsubTopic, ContentTopic: envelope.ContentTopic, Payload: envelope.Payload}
}

func sealedEnvelopes(ctx context.Context, items <-chan network.Envelope) <-chan messaging.SealedEnvelope {
	out := make(chan messaging.SealedEnvelope, 16)
	go func() {
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				return
			case item, ok := <-items:
				if !ok {
					return
				}
				select {
				case <-ctx.Done():
					return
				case out <- sealedEnvelope(item):
				}
			}
		}
	}()
	return out
}
