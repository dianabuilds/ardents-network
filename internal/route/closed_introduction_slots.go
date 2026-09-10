package route

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"time"
)

const closedIntroductionSlotMaximum = 1024

// ClosedIntroductionSlots retains only slot hashes and terminal expiry under
// the receiving duty's exclusive spend-root lease. It grants no registration.
type ClosedIntroductionSlots struct {
	owner   *ClosedSpendLedger
	path    string
	entries map[[32]byte]time.Time
	floor   time.Time
	closed  bool
	failure error
}

// IntroductionSlots opens the replay floor before the first class-3 spend.
// A missing floor with prior spends is damage, never permission to reset it.
func (ledger *ClosedSpendLedger) IntroductionSlots() (*ClosedIntroductionSlots, error) {
	if ledger == nil {
		return nil, errors.New("closed Introduction slot owner absent")
	}
	ledger.mu.Lock()
	defer ledger.mu.Unlock()
	if ledger.closed {
		return nil, errors.New("closed Introduction slot owner released")
	}
	if ledger.slots != nil {
		return ledger.slots, nil
	}
	owner := &ClosedIntroductionSlots{owner: ledger, path: filepath.Join(filepath.Dir(ledger.path), "closed-introduction-slots"), entries: make(map[[32]byte]time.Time)}
	header := owner.header()
	raw, err := readClosedSpendFile(owner.path, len(header)+closedIntroductionSlotMaximum*40)
	if errors.Is(err, os.ErrNotExist) {
		if len(ledger.spent) != 0 {
			return nil, errors.New("closed Introduction slot floor missing after admission")
		}
		if err := replaceClosedReplayFile(owner.path, header); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	} else {
		if len(raw) < len(header) || !bytes.Equal(raw[:closedSpendLedgerHeaderSize], header[:closedSpendLedgerHeaderSize]) || (len(raw)-len(header))%40 != 0 {
			return nil, errors.New("closed Introduction slot floor invalid or rebound")
		}
		seconds := binary.BigEndian.Uint64(raw[closedSpendLedgerHeaderSize:len(header)])
		if seconds > 1<<63-1 {
			return nil, errors.New("closed Introduction slot time floor invalid")
		}
		if seconds != 0 {
			owner.floor = time.Unix(int64(seconds), 0).UTC()
		}
		for offset := len(header); offset < len(raw); offset += 40 {
			var digest [32]byte
			copy(digest[:], raw[offset:offset+32])
			expiry := binary.BigEndian.Uint64(raw[offset+32 : offset+40])
			if digest == [32]byte{} || expiry == 0 || expiry > 1<<63-1 || !owner.entries[digest].IsZero() {
				return nil, errors.New("closed Introduction slot floor entry invalid")
			}
			owner.entries[digest] = time.Unix(int64(expiry), 0).UTC()
		}
	}
	ledger.slots = owner
	return owner, nil
}

// Claim burns this slot until its original expiry before success can be sent.
// Ambiguous persistence terminalizes the owner; reopening reads the actual
// committed snapshot. It cannot be cleared by withdrawal or a fresh token.
func (slots *ClosedIntroductionSlots) Claim(slot [32]byte, expiry, now time.Time) error {
	if slots == nil || slot == [32]byte{} || now.IsZero() || !now.Before(expiry) || expiry.After(now.Add(600*time.Second)) || !expiry.Equal(expiry.UTC().Truncate(time.Second)) {
		return errors.New("closed Introduction slot claim invalid")
	}
	slots.owner.mu.Lock()
	defer slots.owner.mu.Unlock()
	if slots.closed || slots.failure != nil {
		return errors.Join(errors.New("closed Introduction slot floor unavailable"), slots.failure)
	}
	if now.Before(slots.floor) || now.Unix() <= 0 {
		return errors.New("closed Introduction slot clock below retained floor")
	}
	digest := sha256.Sum256(slot[:])
	if prior, exists := slots.entries[digest]; exists && now.Before(prior) {
		return errors.New("closed Introduction slot already used")
	}
	next := make(map[[32]byte]time.Time)
	for id, end := range slots.entries {
		if now.Before(end) {
			next[id] = end
		}
	}
	if len(next) >= closedIntroductionSlotMaximum {
		return errors.New("closed Introduction slot floor capacity exhausted")
	}
	next[digest] = expiry
	ids := make([][32]byte, 0, len(next))
	for id := range next {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(a, b int) bool { return bytes.Compare(ids[a][:], ids[b][:]) < 0 })
	raw := slots.header()
	newFloor := now.UTC().Truncate(time.Second)
	binary.BigEndian.PutUint64(raw[closedSpendLedgerHeaderSize:], uint64(newFloor.Unix()))
	for _, id := range ids {
		raw = append(raw, id[:]...)
		raw = binary.BigEndian.AppendUint64(raw, uint64(next[id].Unix()))
	}
	if err := replaceClosedReplayFile(slots.path, raw); err != nil {
		slots.failure = err
		return err
	}
	slots.entries, slots.floor = next, newFloor
	return nil
}

func (slots *ClosedIntroductionSlots) header() []byte {
	header := encodeClosedSpendHeader(slots.owner.binding)
	copy(header[:8], "ARDISL01")
	var seconds uint64
	if !slots.floor.IsZero() {
		seconds = uint64(slots.floor.Unix())
	}
	return binary.BigEndian.AppendUint64(header, seconds)
}
