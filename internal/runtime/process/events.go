package process

import (
	"context"
	"strings"
	"time"

	nodeapi "ardents/internal/node/api"
)

func (n *Node) publishLocked(topic string, data map[string]any) {
	n.seq++
	now := time.Now().UTC()
	event := nodeapi.Event{
		Seq:   n.seq,
		Time:  now,
		Topic: topic,
		Data:  cloneEventData(data),
	}
	domain, eventType := splitEventTopic(topic)
	resource := ""
	if id, ok := data["id"].(string); ok {
		resource = id
	} else if subject, ok := data["subject"].(string); ok {
		resource = subject
	}
	n.diag.RecordEvent(domain, eventType, resource, topic, "", data)
	for ch := range n.subs {
		select {
		case ch <- event:
		default:
		}
	}
}

func (n *Node) Subscribe(ctx context.Context) <-chan nodeapi.Event {
	ch := make(chan nodeapi.Event, 16)

	n.mu.Lock()
	n.subs[ch] = struct{}{}
	n.mu.Unlock()

	go func() {
		<-ctx.Done()
		n.mu.Lock()
		delete(n.subs, ch)
		n.mu.Unlock()
		close(ch)
	}()
	return ch
}

func cloneEventData(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func splitEventTopic(topic string) (string, string) {
	domain, eventType, ok := strings.Cut(topic, ".")
	if !ok {
		return topic, topic
	}
	return domain, eventType
}
