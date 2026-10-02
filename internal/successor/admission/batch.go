package admission

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"math"
	"math/big"

	"github.com/dianabuilds/ardents-network/internal/successor/admission/issuerprofile"
)

type verifiedBatch struct {
	request, digest, permission, commitment [32]byte
	window                                  uint64
	class                                   uint8
	count                                   uint16
	maxima                                  [3]uint32
}

func verifyBatch(ctx context.Context, raw []byte, facts Facts, binding LedgerBinding) (verifiedBatch, Outcome) {
	var b verifiedBatch
	const base = 8 + 228 + 32 + 1 + 8 + 346 + 2
	if len(raw) < base+259+64 || len(raw) > 16<<10 || string(raw[:8]) != "ARDIBR01" {
		return b, Malformed
	}
	count := int(binary.BigEndian.Uint16(raw[623:625]))
	if count < 1 || count > 32 || len(raw) != base+count*259+64 {
		return b, Malformed
	}
	if facts.Class != raw[268] || facts.Count != uint32(count) {
		return b, Binding
	}
	outcome := inspectPermission(ctx, raw[8:236], facts)
	if outcome != Accepted {
		return b, outcome
	}
	if !binding.matches(facts) {
		return b, Binding
	}
	copy(b.request[:], raw[236:268])
	copy(b.permission[:], raw[80:112])
	b.window = binary.BigEndian.Uint64(raw[269:277])
	b.class = raw[268]
	b.count = uint16(count)
	if b.request == [32]byte{} || b.window > math.MaxInt64-3600 || b.window%3600 != 0 || b.window != binary.BigEndian.Uint64(raw[144:152]) {
		return b, Malformed
	}
	spki := raw[277:623]
	public, ok := issuerprofile.ParseKey(spki)
	if !ok {
		return b, Malformed
	}
	found := false
	for _, key := range binding.Keys {
		if key.Window == b.window && key.Class == b.class && bytes.Equal(key.SPKI, spki) {
			found = true
			break
		}
	}
	if !found {
		return b, Binding
	}
	keyID := sha256.Sum256(spki)
	for offset := base; offset < base+count*259; offset += 259 {
		element := raw[offset : offset+259]
		value := new(big.Int).SetBytes(element[3:])
		if binary.BigEndian.Uint16(element[:2]) != 2 || element[2] != keyID[31] || value.Sign() <= 0 || value.Cmp(public.N) >= 0 {
			return b, Malformed
		}
	}
	transcript := append([]byte("ardents-issuance-request-v1\x00"), b.permission[:]...)
	transcript = append(transcript, b.request[:]...)
	transcript = append(transcript, b.class)
	transcript = binary.BigEndian.AppendUint64(transcript, b.window)
	transcript = binary.BigEndian.AppendUint16(transcript, b.count)
	digest := sha256.Sum256(raw[base : base+count*259])
	transcript = append(transcript, digest[:]...)
	if !ed25519.Verify(ed25519.PublicKey(facts.Holder[:]), transcript, raw[len(raw)-64:]) {
		return b, Signature
	}
	if ctx.Err() != nil {
		return b, Canceled
	}
	b.digest = sha256.Sum256(raw)
	b.commitment = sha256.Sum256(raw[8:236])
	for i := range b.maxima {
		b.maxima[i] = binary.BigEndian.Uint32(raw[160+i*4 : 164+i*4])
	}
	return b, Accepted
}
