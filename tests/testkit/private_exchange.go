package testkit

import (
	"context"

	"ardents/internal/messaging"
	"ardents/internal/network"
	"ardents/internal/transfer"
)

type PrivateExchange struct{ exchange *transfer.PrivateExchange }

func NewPrivateExchange(channel *messaging.Channel, carrier network.Service) *PrivateExchange {
	return &PrivateExchange{exchange: transfer.NewPrivateExchange(channel, PrivateCarrier(carrier))}
}

type PrivateCarrierContract interface {
	messaging.Carrier
	messaging.LiveCarrier
}

func PrivateCarrier(service network.Service) PrivateCarrierContract {
	return networkCarrier{service: service}
}

type networkCarrier struct{ service network.Service }

func (c networkCarrier) PublishPrivateEnvelope(ctx context.Context, envelope messaging.SealedEnvelope) error {
	return c.service.PublishRelayEnvelope(ctx, network.Envelope{PubsubTopic: envelope.PubsubTopic, ContentTopic: envelope.ContentTopic, Payload: envelope.Payload})
}

func (c networkCarrier) SubscribePrivateEnvelopes(ctx context.Context, topic string) (<-chan messaging.SealedEnvelope, error) {
	items, err := c.service.SubscribeRelayEnvelopes(ctx, network.DefaultPubsubTopic, topic)
	if err != nil {
		return nil, err
	}
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
				out <- messaging.SealedEnvelope{PubsubTopic: item.PubsubTopic, ContentTopic: item.ContentTopic, Payload: item.Payload}
			}
		}
	}()
	return out, nil
}

func (c networkCarrier) FetchPrivateEnvelopes(ctx context.Context, endpoints []string, topic string) ([]messaging.SealedEnvelope, error) {
	items, err := c.service.FetchEnvelopes(ctx, endpoints, topic)
	if err != nil {
		return nil, err
	}
	out := make([]messaging.SealedEnvelope, 0, len(items))
	for _, item := range items {
		out = append(out, messaging.SealedEnvelope{PubsubTopic: item.PubsubTopic, ContentTopic: item.ContentTopic, Payload: item.Payload})
	}
	return out, nil
}

func (p *PrivateExchange) Start(ctx context.Context) error { return p.exchange.Start(ctx) }

func (p *PrivateExchange) Publish(ctx context.Context, class messaging.MessageClass, payload []byte) error {
	return p.exchange.Publish(ctx, class, payload)
}

func (p *PrivateExchange) RegisterResponse(requestID string) (<-chan []byte, func(), error) {
	return p.exchange.RegisterResponse(requestID)
}

func (p *PrivateExchange) Failures() <-chan error { return p.exchange.Failures() }
