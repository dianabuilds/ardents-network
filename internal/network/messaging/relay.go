package messaging

import (
	"context"
	"fmt"
	"sync"
	"time"

	wakuNode "github.com/waku-org/go-waku/waku/v2/node"
	"github.com/waku-org/go-waku/waku/v2/protocol"
	wpb "github.com/waku-org/go-waku/waku/v2/protocol/pb"
	"github.com/waku-org/go-waku/waku/v2/protocol/relay"
)

func Publish(ctx context.Context, node *wakuNode.WakuNode, envelope Envelope) error {
	if node == nil {
		return fmt.Errorf("waku node is not started")
	}
	if envelope.ContentTopic == "" {
		return fmt.Errorf("content topic is required")
	}
	pubsubTopic := envelope.PubsubTopic
	if pubsubTopic == "" {
		pubsubTopic = DefaultPubsubTopic
	}
	ts := time.Now().UnixNano()
	msg := &wpb.WakuMessage{
		Payload:      append([]byte(nil), envelope.Payload...),
		ContentTopic: envelope.ContentTopic,
		Timestamp:    &ts,
	}
	_, err := node.Relay().Publish(ctx, msg, relay.WithPubSubTopic(pubsubTopic))
	return err
}

func Subscribe(ctx context.Context, node *wakuNode.WakuNode, pubsubTopic string, contentTopics ...string) (<-chan Envelope, error) {
	if node == nil {
		return nil, fmt.Errorf("waku node is not started")
	}
	if pubsubTopic == "" {
		pubsubTopic = DefaultPubsubTopic
	}
	if len(contentTopics) == 0 || len(contentTopics) > maxContentTopics {
		return nil, fmt.Errorf("Relay subscription requires between one and eight content topics")
	}
	subs, err := node.Relay().Subscribe(ctx, protocol.NewContentFilter(pubsubTopic, contentTopics...))
	if err != nil {
		return nil, err
	}
	out := make(chan Envelope, 16)
	go relaySubscriptionLoop(ctx, pubsubTopic, subs, out)
	return out, nil
}

func AwaitRelayPeerCount(ctx context.Context, node *wakuNode.WakuNode, pubsubTopic string) int {
	if node == nil {
		return 0
	}
	if pubsubTopic == "" {
		pubsubTopic = DefaultPubsubTopic
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		peers := len(node.Relay().PubSub().ListPeers(pubsubTopic))
		if peers > 0 || time.Now().After(deadline) {
			return peers
		}
		select {
		case <-ctx.Done():
			return peers
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func messageTimestamp(message *wpb.WakuMessage) time.Time {
	if message == nil || message.Timestamp == nil || message.GetTimestamp() <= 0 {
		return time.Time{}
	}
	return time.Unix(0, message.GetTimestamp()).UTC()
}

func relaySubscriptionLoop(ctx context.Context, pubsubTopic string, subs []*relay.Subscription, out chan<- Envelope) {
	var wg sync.WaitGroup
	for _, sub := range subs {
		if sub == nil {
			continue
		}
		wg.Add(1)
		go consumeRelaySubscription(ctx, &wg, pubsubTopic, sub, out)
	}
	wg.Wait()
	close(out)
}

func consumeRelaySubscription(ctx context.Context, wg *sync.WaitGroup, pubsubTopic string, sub *relay.Subscription, out chan<- Envelope) {
	defer wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case env, ok := <-sub.Ch:
			if !ok {
				return
			}
			item, ok := relayEnvelope(pubsubTopic, env)
			if !ok {
				continue
			}
			select {
			case <-ctx.Done():
				return
			case out <- item:
			}
		}
	}
}

func relayEnvelope(pubsubTopic string, env *protocol.Envelope) (Envelope, bool) {
	if env == nil || env.Message() == nil {
		return Envelope{}, false
	}
	return Envelope{
		PubsubTopic:  pubsubTopic,
		ContentTopic: env.Message().ContentTopic,
		Payload:      append([]byte(nil), env.Message().Payload...),
		Timestamp:    time.Unix(0, env.Message().GetTimestamp()).UTC(),
	}, true
}
