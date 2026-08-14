package recoverysmoke

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

type boundedTail struct {
	mu    sync.Mutex
	bytes []byte
	limit int
}

func (tail *boundedTail) Write(value []byte) (int, error) {
	tail.mu.Lock()
	defer tail.mu.Unlock()
	if len(value) >= tail.limit {
		tail.bytes = append(tail.bytes[:0], value[len(value)-tail.limit:]...)
		return len(value), nil
	}
	overflow := len(tail.bytes) + len(value) - tail.limit
	if overflow > 0 {
		tail.bytes = append(tail.bytes[:0], tail.bytes[overflow:]...)
	}
	tail.bytes = append(tail.bytes, value...)
	return len(value), nil
}

func (tail *boundedTail) value() []byte {
	tail.mu.Lock()
	defer tail.mu.Unlock()
	return append([]byte(nil), tail.bytes...)
}

func (observer dockerObserver) exitDiagnostic(ctx context.Context, identity string) string {
	bounded, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	tail := &boundedTail{limit: 8 << 10}
	command := exec.CommandContext(bounded, "docker", "logs", "--tail", "40", identity)
	command.Dir, command.Stdout, command.Stderr = observer.input.SourceRoot, tail, tail
	_ = command.Run()
	raw := tail.value()
	digest := sha256.Sum256(raw)
	return "log_sha256=" + hex.EncodeToString(digest[:]) + " class=" + classifyExitLog(raw)
}

func classifyExitLog(raw []byte) string {
	classification := "unclassified"
	for _, line := range splitLines(raw) {
		var evidence route.Evidence
		if json.Unmarshal(line, &evidence) == nil && evidence.Schema == "ardents-h3-route-observation-v1" {
			if value := allowlistedExitClass(evidence.Error); value != "unclassified" {
				classification = value
			}
			continue
		}
		if value := allowlistedExitClass(string(line)); value != "unclassified" {
			classification = value
		}
	}
	return classification
}

func allowlistedExitClass(value string) string {
	classes := []struct{ prefix, class string }{
		{"authenticate initiator:", "initiator-authentication"},
		{"confirm initiator Network State binding:", "initiator-binding"},
		{"carry raw Route Attachment:", "raw-attachment-relay"},
		{"authenticate upstream:", "upstream-authentication"},
		{"accept authenticated leg binding:", "upstream-binding"},
		{"authenticate next role:", "next-role-authentication"},
		{"confirm next authenticated leg binding:", "next-role-binding"},
		{"dial unix ", "local-attachment-unavailable"},
		{"context deadline exceeded", "deadline"},
	}
	trimmed := strings.TrimSpace(value)
	for _, candidate := range classes {
		if strings.HasPrefix(trimmed, candidate.prefix) {
			return candidate.class
		}
	}
	return "unclassified"
}
