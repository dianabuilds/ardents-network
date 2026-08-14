package recoverysmoke

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

func parseAttachmentCount(raw []byte, role string) (uint32, error) {
	var highest uint32
	seen := map[uint32]bool{}
	for _, line := range splitLines(raw) {
		var value struct {
			Kind, Role, Terminal string
			Attachment           uint32
			PeerAuthenticated    bool `json:"peer_authenticated"`
		}
		if err := json.Unmarshal(line, &value); err != nil {
			return 0, errors.Join(err, errors.New("decode "+role+" attachment evidence"))
		}
		if value.Kind == "complete" && value.Role == role && value.Terminal == "success" && value.PeerAuthenticated {
			if value.Attachment == 0 || seen[value.Attachment] {
				return 0, errors.New(role + " attachment evidence is duplicated or unnumbered")
			}
			seen[value.Attachment] = true
			highest = max(highest, value.Attachment)
		}
	}
	if highest == 0 {
		return 0, errors.New(role + " authenticated attachment evidence is missing")
	}
	return highest, nil
}

func (observer dockerObserver) waitContainerStopped(ctx context.Context, identity string, limit time.Duration) error {
	if !validContainerID(identity) {
		return errors.New("replacement process identity is invalid")
	}
	deadline := time.Now().Add(limit)
	for time.Now().Before(deadline) {
		raw, err := observer.docker(ctx, 10*time.Second, "inspect", "--format", "{{.Id}} {{.State.Running}}", identity)
		if err == nil && strings.TrimSpace(string(raw)) == identity+" false" {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	return errors.New("replacement process did not stop within its bounded lifetime")
}
