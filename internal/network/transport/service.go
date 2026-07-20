package transport

import (
	"context"
	"fmt"
	"strings"

	discovery "ardents/internal/discovery"
	networkmessaging "ardents/internal/network/messaging"
	networkprivacy "ardents/internal/network/privacy"
	networkreadiness "ardents/internal/network/readiness"
	networkroute "ardents/internal/network/route"
)

func (s *Service) BuildCandidates(record discovery.Record, trusted bool) []networkroute.Candidate {
	return networkroute.BuildCandidates(record.Subject, record.Service, record.Mode, record.Endpoints, trusted, s.isObservedUsable)
}

func (s *Service) PublishRelayEnvelope(ctx context.Context, envelope networkmessaging.Envelope) error {
	s.mu.Lock()
	constrained := s.cfg.NodeProfile == networkreadiness.NodeProfileConstrainedClient
	s.mu.Unlock()
	if constrained {
		return fmt.Errorf("Relay publication is unavailable in constrained light-client mode")
	}
	done, err := s.acquireNetworkOperation(len(envelope.Payload), "")
	if err != nil {
		return err
	}
	s.mu.Lock()
	node := s.node
	s.mu.Unlock()
	err = networkmessaging.Publish(ctx, node, envelope)
	done(err)
	return err
}

func (s *Service) PublishPrivateEnvelope(ctx context.Context, envelope networkprivacy.SealedEnvelope) error {
	if s.cfg.NodeProfile == networkreadiness.NodeProfileConstrainedClient {
		return fmt.Errorf("Relay publication is unavailable in constrained light-client mode")
	}
	return s.PublishRelayEnvelope(ctx, networkmessaging.Envelope{
		PubsubTopic: envelope.PubsubTopic, ContentTopic: envelope.ContentTopic,
		Payload: envelope.Payload,
	})
}

func (s *Service) PublishPrivateLightpush(ctx context.Context, provider string, envelope networkprivacy.SealedEnvelope) error {
	s.mu.Lock()
	node := s.node
	constrained := s.cfg.NodeProfile == networkreadiness.NodeProfileConstrainedClient
	s.mu.Unlock()
	if !constrained {
		return fmt.Errorf("Lightpush client operation is available only in constrained light-client mode")
	}
	done, err := s.acquireNetworkOperation(len(envelope.Payload), provider)
	if err != nil {
		return err
	}
	err = networkmessaging.PublishLightpush(ctx, node, provider, networkmessaging.Envelope{
		PubsubTopic: envelope.PubsubTopic, ContentTopic: envelope.ContentTopic, Payload: envelope.Payload,
	})
	done(err)
	return err
}

func (s *Service) SubscribePrivateFilter(ctx context.Context, providers []string, contentTopic string) (<-chan networkprivacy.SealedEnvelope, error) {
	s.mu.Lock()
	node := s.node
	constrained := s.cfg.NodeProfile == networkreadiness.NodeProfileConstrainedClient
	s.mu.Unlock()
	if !constrained {
		return nil, fmt.Errorf("Filter client operation is available only in constrained light-client mode")
	}
	providerKey := strings.Join(providers, "\n")
	done, err := s.acquireNetworkOperation(0, providerKey)
	if err != nil {
		return nil, err
	}
	items, err := networkmessaging.SubscribeFilter(ctx, node, providers, networkmessaging.DefaultPubsubTopic, contentTopic)
	done(err)
	if err != nil {
		return nil, err
	}
	return projectPrivateEnvelopes(ctx, items), nil
}

func (s *Service) SubscribePrivateEnvelopes(ctx context.Context, contentTopic string) (<-chan networkprivacy.SealedEnvelope, error) {
	if s.cfg.NodeProfile == networkreadiness.NodeProfileConstrainedClient {
		return nil, fmt.Errorf("Relay subscription is unavailable in constrained light-client mode")
	}
	items, err := s.SubscribeRelayEnvelopes(ctx, networkmessaging.DefaultPubsubTopic, contentTopic)
	if err != nil {
		return nil, err
	}
	out := make(chan networkprivacy.SealedEnvelope, 16)
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
				case out <- networkprivacy.SealedEnvelope{
					PubsubTopic: item.PubsubTopic, ContentTopic: item.ContentTopic, Payload: item.Payload,
				}:
				}
			}
		}
	}()
	return out, nil
}

func projectPrivateEnvelopes(ctx context.Context, items <-chan networkmessaging.Envelope) <-chan networkprivacy.SealedEnvelope {
	out := make(chan networkprivacy.SealedEnvelope, 16)
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
				sealed := networkprivacy.SealedEnvelope{PubsubTopic: item.PubsubTopic, ContentTopic: item.ContentTopic, Payload: item.Payload}
				select {
				case <-ctx.Done():
					return
				case out <- sealed:
				}
			}
		}
	}()
	return out
}

func (s *Service) FetchPrivateEnvelopes(ctx context.Context, endpoints []string, contentTopic string) ([]networkprivacy.SealedEnvelope, error) {
	done, err := s.acquireNetworkOperation(0, strings.Join(endpoints, "\n"))
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	node := s.node
	maxResults := s.cfg.Limits.MaxStoreResults
	s.mu.Unlock()
	items, err := networkmessaging.FetchEnvelopes(
		ctx, node, endpoints, maxResults, networkmessaging.DefaultPubsubTopic, contentTopic,
	)
	done(err)
	if err != nil {
		return nil, err
	}
	out := make([]networkprivacy.SealedEnvelope, 0, len(items))
	for _, item := range items {
		out = append(out, networkprivacy.SealedEnvelope{
			PubsubTopic: item.PubsubTopic, ContentTopic: item.ContentTopic,
			Payload: item.Payload,
		})
	}
	return out, nil
}

func (s *Service) SubscribeRelayEnvelopes(ctx context.Context, pubsubTopic string, contentTopics ...string) (<-chan networkmessaging.Envelope, error) {
	s.mu.Lock()
	constrained := s.cfg.NodeProfile == networkreadiness.NodeProfileConstrainedClient
	s.mu.Unlock()
	if constrained {
		return nil, fmt.Errorf("Relay subscription is unavailable in constrained light-client mode")
	}
	done, err := s.acquireNetworkOperation(0, "")
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	node := s.node
	s.mu.Unlock()
	items, err := networkmessaging.Subscribe(ctx, node, pubsubTopic, contentTopics...)
	done(err)
	if err != nil {
		return nil, err
	}
	out := make(chan networkmessaging.Envelope, 16)
	go func() {
		defer close(out)
		for item := range items {
			out <- item
		}
	}()
	return out, nil
}

func (s *Service) isObservedUsable(endpoint string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, ok := s.observed[strings.TrimSpace(endpoint)]
	return ok && state.usable
}
