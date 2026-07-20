package privacy

import (
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	db "ardents/internal/persistence"
)

const replayBucket = "network-privacy-replay"
const replayStateKey = "ledger"

type DurableReplayLedger struct {
	mu            sync.Mutex
	path          string
	key           [32]byte
	perChannelMax int
	globalMax     int
	state         replayState
}

type replayState struct {
	Version uint32                 `json:"version"`
	Entries map[string]replayEntry `json:"entries"`
}

type replayEntry struct {
	ChannelDigest string `json:"channel_digest"`
	ExpiresAt     int64  `json:"expires_at"`
}

func NewDurableReplayLedger(path string, localKey []byte, perChannelMax, globalMax int) (*DurableReplayLedger, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("durable replay ledger path is required")
	}
	if len(localKey) != 32 {
		return nil, fmt.Errorf("replay ledger key must be 32 bytes")
	}
	if perChannelMax <= 0 || globalMax <= 0 || perChannelMax > globalMax {
		return nil, fmt.Errorf("replay ledger capacity is invalid")
	}
	derived, err := hkdf.Key(sha256.New, localKey, nil, "ardents-private-replay-digest/1", 32)
	if err != nil {
		return nil, err
	}
	ledger := &DurableReplayLedger{
		path: path, perChannelMax: perChannelMax, globalMax: globalMax,
		state: replayState{Version: 1, Entries: map[string]replayEntry{}},
	}
	copy(ledger.key[:], derived)
	var stored replayState
	found, err := db.LoadJSON(path, replayBucket, replayStateKey, &stored)
	if err != nil {
		return nil, err
	}
	if found {
		if stored.Version != 1 || stored.Entries == nil {
			return nil, fmt.Errorf("replay ledger state is invalid")
		}
		ledger.state = stored
	}
	return ledger, nil
}

func (l *DurableReplayLedger) Admit(use ReplayUse) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := use.Now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if use.CapabilityRef == "" || use.Generation == 0 || zeroBytes(use.MessageID[:]) ||
		!now.Before(use.ExpiresAt.UTC()) {
		return envelopeError(CodeEnvelopeExpired, "replay admission input is invalid or expired")
	}
	channelDigest := l.channelDigest(use)
	messageDigest := l.messageDigest(channelDigest, use.MessageID)
	next := cloneReplayState(l.state)
	pruneReplayEntries(next.Entries, now)
	if _, exists := next.Entries[messageDigest]; exists {
		return envelopeError(CodeEnvelopeReplayed, "authenticated envelope was already admitted")
	}
	if len(next.Entries) >= l.globalMax || channelCount(next.Entries, channelDigest) >= l.perChannelMax {
		return envelopeError(CodeReplayCapacityExhausted, "replay ledger capacity is exhausted")
	}
	next.Entries[messageDigest] = replayEntry{
		ChannelDigest: channelDigest, ExpiresAt: use.ExpiresAt.UTC().Unix(),
	}
	if err := db.SaveJSON(l.path, replayBucket, replayStateKey, next); err != nil {
		return err
	}
	l.state = next
	return nil
}

func (l *DurableReplayLedger) channelDigest(use ReplayUse) string {
	mac := hmac.New(sha256.New, l.key[:])
	_, _ = mac.Write([]byte("ardents-private-replay-channel/1"))
	_, _ = mac.Write([]byte(use.CapabilityRef))
	var generation [4]byte
	binary.BigEndian.PutUint32(generation[:], use.Generation)
	_, _ = mac.Write(generation[:])
	return hex.EncodeToString(mac.Sum(nil))
}

func (l *DurableReplayLedger) messageDigest(channelDigest string, messageID [16]byte) string {
	mac := hmac.New(sha256.New, l.key[:])
	_, _ = mac.Write([]byte("ardents-private-replay-message/1"))
	_, _ = mac.Write([]byte(channelDigest))
	_, _ = mac.Write(messageID[:])
	return hex.EncodeToString(mac.Sum(nil))
}

func cloneReplayState(source replayState) replayState {
	out := replayState{Version: 1, Entries: make(map[string]replayEntry, len(source.Entries))}
	for key, entry := range source.Entries {
		out.Entries[key] = entry
	}
	return out
}

func pruneReplayEntries(entries map[string]replayEntry, now time.Time) {
	for key, entry := range entries {
		if entry.ExpiresAt <= now.Unix() {
			delete(entries, key)
		}
	}
}

func channelCount(entries map[string]replayEntry, channelDigest string) int {
	count := 0
	for _, entry := range entries {
		if hmac.Equal([]byte(entry.ChannelDigest), []byte(channelDigest)) {
			count++
		}
	}
	return count
}

var _ ReplayLedger = (*DurableReplayLedger)(nil)
