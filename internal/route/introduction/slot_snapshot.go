package introduction

import (
	"bytes"
	"encoding/binary"
	"errors"
	"sort"
	"time"
)

const (
	slotHeaderSize = 120
	slotMaximum    = 1024
)

// Binding is the exact public duty binding of one independent Route history.
// It is not a Network authority observation.
type Binding struct {
	NetworkID, ProfileDigest, ReceiverNodeID [32]byte
	ReceiverDutyGeneration                   uint64
}

// slotSnapshot owns the canonical binding, hashed slots and original expiry
// floor representation. It grants no durable or live registration authority.
// History supplies the actual independently leased and committed storage.
type slotSnapshot struct {
	binding Binding
	entries map[[32]byte]time.Time
	floor   time.Time
}

func (h *slotSnapshot) encode(entries map[[32]byte]time.Time, floor time.Time) []byte {
	raw := make([]byte, 0, slotHeaderSize+len(entries)*40)
	raw = append(raw, "ARDISL01"...)
	for _, id := range [][32]byte{h.binding.NetworkID, h.binding.ProfileDigest, h.binding.ReceiverNodeID} {
		raw = append(raw, id[:]...)
	}
	raw = binary.BigEndian.AppendUint64(raw, h.binding.ReceiverDutyGeneration)
	var seconds uint64
	if !floor.IsZero() {
		seconds = uint64(floor.Unix())
	}
	raw = binary.BigEndian.AppendUint64(raw, seconds)
	ids := make([][32]byte, 0, len(entries))
	for id := range entries {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(a, b int) bool { return bytes.Compare(ids[a][:], ids[b][:]) < 0 })
	for _, id := range ids {
		raw = append(raw, id[:]...)
		raw = binary.BigEndian.AppendUint64(raw, uint64(entries[id].Unix()))
	}
	return raw
}

func (h *slotSnapshot) decode(raw []byte) error {
	header := h.encode(nil, time.Time{})
	if len(raw) < slotHeaderSize || (len(raw)-slotHeaderSize)%40 != 0 || !bytes.Equal(raw[:112], header[:112]) {
		return errors.New("introduction history damaged or rebound")
	}
	seconds := binary.BigEndian.Uint64(raw[112:120])
	if seconds > 1<<63-1 {
		return errors.New("introduction time floor invalid")
	}
	if seconds != 0 {
		h.floor = time.Unix(int64(seconds), 0).UTC()
	}
	if len(raw) > slotHeaderSize && seconds == 0 {
		return errors.New("introduction claimed history lacks time floor")
	}
	var previous [32]byte
	for offset := slotHeaderSize; offset < len(raw); offset += 40 {
		var digest [32]byte
		copy(digest[:], raw[offset:offset+32])
		expiry := binary.BigEndian.Uint64(raw[offset+32 : offset+40])
		if digest == [32]byte{} || expiry <= seconds || expiry > 1<<63-1 || (offset != slotHeaderSize && bytes.Compare(previous[:], digest[:]) >= 0) {
			return errors.New("introduction history entry invalid")
		}
		h.entries[digest] = time.Unix(int64(expiry), 0).UTC()
		previous = digest
	}
	return nil
}
