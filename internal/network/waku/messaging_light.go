package waku

import (
	"ardents/internal/network"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/multiformats/go-multiaddr"
	wakuNode "github.com/waku-org/go-waku/waku/v2/node"
	"github.com/waku-org/go-waku/waku/v2/protocol"
	"github.com/waku-org/go-waku/waku/v2/protocol/filter"
	"github.com/waku-org/go-waku/waku/v2/protocol/lightpush"
	wpb "github.com/waku-org/go-waku/waku/v2/protocol/pb"
	"github.com/waku-org/go-waku/waku/v2/protocol/subscription"
)

const (
	maxLightProviders = 2
	maxContentTopics  = 8
)

func PublishLightpush(ctx context.Context, node *wakuNode.WakuNode, provider string, envelope network.Envelope) error {
	if node == nil || node.Lightpush() == nil {
		return fmt.Errorf("waku Lightpush client is not active")
	}
	address, err := multiaddr.NewMultiaddr(provider)
	if err != nil {
		return fmt.Errorf("lightpush provider address is invalid")
	}
	if envelope.ContentTopic == "" {
		return fmt.Errorf("content topic is required")
	}
	pubsubTopic := envelope.PubsubTopic
	if pubsubTopic == "" {
		pubsubTopic = network.DefaultPubsubTopic
	}
	message := &wpb.WakuMessage{
		Payload: append([]byte(nil), envelope.Payload...), ContentTopic: envelope.ContentTopic, Timestamp: new(time.Now().UnixNano()),
	}
	_, err = node.Lightpush().Publish(ctx, message,
		lightpush.WithPeerAddr(address), lightpush.WithPubSubTopic(pubsubTopic), lightpush.WithAutomaticRequestID())
	if err != nil {
		return fmt.Errorf("lightpush provider did not accept the message: %w", err)
	}
	return nil
}

func SubscribeFilter(ctx context.Context, node *wakuNode.WakuNode, providers []string, pubsubTopic string, contentTopics ...string) (<-chan network.Envelope, error) {
	if node == nil || node.FilterLightnode() == nil {
		return nil, fmt.Errorf("waku Filter client is not active")
	}
	if len(providers) == 0 || len(providers) > maxLightProviders {
		return nil, fmt.Errorf("filter subscription requires one or two providers")
	}
	if len(contentTopics) == 0 || len(contentTopics) > maxContentTopics {
		return nil, fmt.Errorf("filter subscription requires between one and eight content topics")
	}
	if pubsubTopic == "" {
		pubsubTopic = network.DefaultPubsubTopic
	}
	contentFilter := protocol.NewContentFilter(pubsubTopic, contentTopics...)
	subs, err := subscribeFilterProviders(ctx, node, providers, contentFilter)
	if err != nil {
		return nil, err
	}
	out := make(chan network.Envelope, 16)
	go filterSubscriptionLoop(ctx, node, pubsubTopic, subs, out)
	return out, nil
}

func subscribeFilterProviders(ctx context.Context, node *wakuNode.WakuNode, providers []string, contentFilter protocol.ContentFilter) ([]*subscription.SubscriptionDetails, error) {
	var subscriptions []*subscription.SubscriptionDetails
	for _, provider := range providers {
		address, err := multiaddr.NewMultiaddr(provider)
		if err != nil {
			return nil, errors.Join(fmt.Errorf("filter provider address is invalid"), closeFilterSubscriptions(node, subscriptions))
		}
		items, err := node.FilterLightnode().Subscribe(ctx, contentFilter, filter.WithPeerAddr(address))
		if err != nil || len(items) == 0 {
			cleanupErr := closeFilterSubscriptions(node, append(subscriptions, items...))
			if err != nil {
				return nil, errors.Join(fmt.Errorf("filter provider rejected the subscription: %w", err), cleanupErr)
			}
			return nil, errors.Join(fmt.Errorf("filter provider returned no subscription"), cleanupErr)
		}
		subscriptions = append(subscriptions, items...)
	}
	return subscriptions, nil
}

func filterSubscriptionLoop(ctx context.Context, node *wakuNode.WakuNode, pubsubTopic string, subs []*subscription.SubscriptionDetails, out chan<- network.Envelope) {
	var workers sync.WaitGroup
	for _, sub := range subs {
		workers.Add(1)
		go consumeFilterSubscription(ctx, &workers, pubsubTopic, sub, out)
	}
	workers.Wait()
	if err := closeFilterSubscriptions(node, subs); err != nil {
		slog.Error("close filter subscriptions", "error", err)
	}
	close(out)
}

func consumeFilterSubscription(ctx context.Context, workers *sync.WaitGroup, pubsubTopic string, sub *subscription.SubscriptionDetails, out chan<- network.Envelope) {
	defer workers.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case item, ok := <-sub.C:
			if !ok {
				return
			}
			envelope, ok := relayEnvelope(pubsubTopic, item)
			if !ok {
				continue
			}
			select {
			case <-ctx.Done():
				return
			case out <- envelope:
			}
		}
	}
}

func closeFilterSubscriptions(node *wakuNode.WakuNode, subs []*subscription.SubscriptionDetails) error {
	if node == nil || node.FilterLightnode() == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var failures []error
	for _, sub := range subs {
		if _, err := node.FilterLightnode().UnsubscribeWithSubscription(ctx, sub); err != nil {
			failures = append(failures, fmt.Errorf("unsubscribe filter subscription: %w", err))
		}
	}
	return errors.Join(failures...)
}
