package waku

import (
	"ardents/internal/network"
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"slices"
	"sort"
	"strings"

	ma "github.com/multiformats/go-multiaddr"
	wakuNode "github.com/waku-org/go-waku/waku/v2/node"
	legacyStore "github.com/waku-org/go-waku/waku/v2/protocol/legacy_store"
	wpb "github.com/waku-org/go-waku/waku/v2/protocol/pb"
)

func FetchEnvelopes(ctx context.Context, node *wakuNode.WakuNode, endpoints []string, maximumFetchedEnvelopes int, pubsubTopic string, contentTopics ...string) ([]network.Envelope, error) {
	if err := validateStoreQuery(node, endpoints, maximumFetchedEnvelopes, contentTopics); err != nil {
		return nil, err
	}
	query := legacyStore.Query{PubsubTopic: pubsubTopic, ContentTopics: contentTopics}
	seen := make(map[[32]byte]network.Envelope)
	var lastErr error
	for _, endpoint := range endpoints {
		addr, ok := storeEndpoint(endpoint)
		if !ok {
			continue
		}
		messages, err := queryStoreMessages(ctx, node, query, addr, maximumFetchedEnvelopes)
		if err != nil {
			lastErr = err
			continue
		}
		for _, message := range messages {
			envelope, ok := storedEnvelope(pubsubTopic, contentTopics, message)
			if !ok {
				continue
			}
			seen[envelopeDigest(envelope)] = envelope
			if len(seen) >= maximumFetchedEnvelopes {
				return envelopeValues(seen), nil
			}
		}
	}
	if len(seen) == 0 {
		return nil, lastErr
	}
	return envelopeValues(seen), nil
}

func validateStoreQuery(node *wakuNode.WakuNode, endpoints []string, maximum int, contentTopics []string) error {
	if node == nil {
		return fmt.Errorf("waku node is not started")
	}
	if maximum < 1 {
		return fmt.Errorf("store result limit is invalid")
	}
	if len(endpoints) == 0 || len(endpoints) > 4 {
		return fmt.Errorf("store query requires between one and four endpoints")
	}
	if len(contentTopics) == 0 || len(contentTopics) > maxContentTopics {
		return fmt.Errorf("store query requires between one and eight content topics")
	}
	return nil
}

func queryStoreMessages(
	ctx context.Context,
	node *wakuNode.WakuNode,
	query legacyStore.Query,
	addr ma.Multiaddr,
	maximumFetchedEnvelopes int,
) ([]*wpb.WakuMessage, error) {
	result, err := node.LegacyStore().Query(
		ctx, query, legacyStore.WithPeerAddr(addr),
		legacyStore.WithAutomaticRequestID(), legacyStore.WithPaging(false, 128),
	)
	if err != nil {
		return nil, err
	}
	out := append([]*wpb.WakuMessage(nil), result.Messages...)
	for !result.IsComplete() && len(out) < maximumFetchedEnvelopes {
		result, err = node.LegacyStore().Next(ctx, result)
		if err != nil {
			return nil, err
		}
		out = append(out, result.Messages...)
	}
	if len(out) > maximumFetchedEnvelopes {
		out = out[:maximumFetchedEnvelopes]
	}
	return out, nil
}

func storeEndpoint(endpoint string) (ma.Multiaddr, bool) {
	endpoint = strings.TrimSpace(endpoint)
	if !strings.HasPrefix(endpoint, "/") {
		return nil, false
	}
	addr, err := ma.NewMultiaddr(endpoint)
	return addr, err == nil
}

func storedEnvelope(pubsubTopic string, contentTopics []string, message *wpb.WakuMessage) (network.Envelope, bool) {
	if message == nil || !containsTopic(contentTopics, message.ContentTopic) {
		return network.Envelope{}, false
	}
	return network.Envelope{
		PubsubTopic: pubsubTopic, ContentTopic: message.ContentTopic,
		Payload: append([]byte(nil), message.Payload...), Timestamp: messageTimestamp(message),
	}, true
}

func containsTopic(topics []string, wanted string) bool {
	return slices.Contains(topics, wanted)
}

func envelopeDigest(envelope network.Envelope) [32]byte {
	raw := append([]byte(envelope.PubsubTopic), 0)
	raw = append(raw, envelope.ContentTopic...)
	raw = append(raw, 0)
	raw = append(raw, envelope.Payload...)
	return sha256.Sum256(raw)
}

func envelopeValues(items map[[32]byte]network.Envelope) []network.Envelope {
	out := make([]network.Envelope, 0, len(items))
	for _, envelope := range items {
		out = append(out, envelope)
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].Timestamp.Equal(out[j].Timestamp) {
			return out[i].Timestamp.Before(out[j].Timestamp)
		}
		if out[i].ContentTopic != out[j].ContentTopic {
			return out[i].ContentTopic < out[j].ContentTopic
		}
		return bytes.Compare(out[i].Payload, out[j].Payload) < 0
	})
	return out
}
