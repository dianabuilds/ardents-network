package introduction

import (
	"bytes"
	"reflect"
	"testing"
	"time"
)

// The fixture states accepted offsets and integer bytes independently of the
// encoder. It supplies no durable ACK, fresh-root fact or registration authority.
func slotSnapshotWireFixture() (slotSnapshot, []byte) {
	snapshot := slotSnapshot{
		binding: Binding{NetworkID: [32]byte{1}, ProfileDigest: [32]byte{2}, ReceiverNodeID: [32]byte{3}, ReceiverDutyGeneration: 0x05060708},
		floor:   time.Unix(0x01020304, 0).UTC(),
		entries: map[[32]byte]time.Time{
			{8}: time.Unix(0x01020320, 0).UTC(),
			{7}: time.Unix(0x01020310, 0).UTC(),
		},
	}
	raw := make([]byte, 200)
	copy(raw[:8], []byte{'A', 'R', 'D', 'I', 'S', 'L', '0', '1'})
	raw[8], raw[40], raw[72] = 1, 2, 3
	copy(raw[108:112], []byte{5, 6, 7, 8})
	copy(raw[116:120], []byte{1, 2, 3, 4})
	raw[120], raw[160] = 7, 8
	copy(raw[156:160], []byte{1, 2, 3, 16})
	copy(raw[196:200], []byte{1, 2, 3, 32})
	return snapshot, raw
}

func TestSlotSnapshotMatchesIndependentCanonicalBytes(t *testing.T) {
	snapshot, expected := slotSnapshotWireFixture()
	if got := snapshot.encode(snapshot.entries, snapshot.floor); !bytes.Equal(got, expected) {
		t.Fatal("canonical binding, sorted hashes or original expiry bytes changed")
	}
	decoded := slotSnapshot{binding: snapshot.binding, entries: make(map[[32]byte]time.Time)}
	if err := decoded.decode(expected); err != nil {
		t.Fatal(err)
	}
	if decoded.floor != snapshot.floor || !reflect.DeepEqual(decoded.entries, snapshot.entries) {
		t.Fatal("decoder changed original expiries or the retained time floor")
	}
	if got := decoded.encode(decoded.entries, decoded.floor); !bytes.Equal(got, expected) {
		t.Fatal("canonical decode changed accepted bytes")
	}
}

func TestSlotSnapshotRefusesDamagedOrReboundBytes(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func([]byte) []byte
	}{
		{"magic", func(raw []byte) []byte { raw[0] = 'X'; return raw }},
		{"binding", func(raw []byte) []byte { raw[40] = 9; return raw }},
		{"generation", func(raw []byte) []byte { raw[111]++; return raw }},
		{"missing floor", func(raw []byte) []byte { clear(raw[112:120]); return raw }},
		{"overflow floor", func(raw []byte) []byte { raw[112] = 128; return raw }},
		{"empty hash", func(raw []byte) []byte { clear(raw[120:152]); return raw }},
		{"expired claim", func(raw []byte) []byte { copy(raw[152:160], raw[112:120]); return raw }},
		{"overflow expiry", func(raw []byte) []byte { raw[152] = 128; return raw }},
		{"duplicate hash", func(raw []byte) []byte { copy(raw[160:192], raw[120:152]); return raw }},
		{"unsorted hashes", func(raw []byte) []byte { raw[120], raw[160] = raw[160], raw[120]; return raw }},
		{"partial entry", func(raw []byte) []byte { return raw[:199] }},
		{"partial header", func(raw []byte) []byte { return raw[:119] }},
		{"trailing byte", func(raw []byte) []byte { return append(raw, 0) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			snapshot, raw := slotSnapshotWireFixture()
			decoded := slotSnapshot{binding: snapshot.binding, entries: make(map[[32]byte]time.Time)}
			if err := decoded.decode(test.change(raw)); err == nil {
				t.Fatal("damaged or rebound non-reclaim snapshot accepted")
			}
		})
	}
}
