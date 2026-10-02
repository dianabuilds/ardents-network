package issuance

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/binary"

	"github.com/cloudflare/circl/blindsign/blindrsa"
	"github.com/dianabuilds/ardents-network/internal/successor/admission"
)

func matchingInventory(store Store, b admission.LedgerBinding) (Inventory, error) {
	v, err := store.Inventory()
	if err != nil {
		return Inventory{}, err
	}
	if _, err = admission.BindingDigest(b); err != nil {
		return Inventory{}, ErrInvalid
	}
	if b.Network != v.Binding.Network || b.Issuer != v.Binding.Issuer || !b.Start.Equal(v.Binding.Start) || !b.End.Equal(v.Binding.End) || len(b.Keys) != len(v.Keys) {
		return Inventory{}, ErrInvalid
	}
	for i, k := range b.Keys {
		p := v.Keys[i]
		if k.Window != p.Window || k.Class != p.Class || !bytes.Equal(k.SPKI, p.SPKI) {
			return Inventory{}, ErrInvalid
		}
	}
	return v, nil
}

// The only callers are confirmed issuance and private retained-result checking.
// No method exposes arbitrary blinded signing to a command or another package.
func (store Store) sign(ctx context.Context, batch []byte) ([]byte, error) {
	if store.state == nil || len(batch) < 625 {
		return nil, ErrInvalid
	}
	s := store.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, ErrClosed
	}
	if err := s.checkIdentity(); err != nil {
		return nil, err
	}
	raw, _, err := readOwned(s.root, "issuer.keys", maximumMaterial)
	if err != nil {
		return nil, err
	}
	defer clear(raw)
	v, err := decodeMaterial(raw, s.inventory.Binding)
	if err != nil || v.Digest != s.inventory.Digest {
		return nil, ErrUnavailable
	}
	target := -1
	window := binary.BigEndian.Uint64(batch[269:277])
	class := batch[268]
	for i, k := range v.Keys {
		if k.Window == window && k.Class == class && bytes.Equal(k.SPKI, batch[277:623]) {
			target = i
		}
	}
	if target < 0 {
		return nil, ErrInvalid
	}
	offset := 122
	for i := 0; i < target; i++ {
		offset += 11 + int(binary.BigEndian.Uint16(raw[offset+9:]))
	}
	n := int(binary.BigEndian.Uint16(raw[offset+9:]))
	key, err := x509.ParsePKCS1PrivateKey(raw[offset+11 : offset+11+n])
	if err != nil {
		return nil, ErrUnavailable
	}
	count := int(binary.BigEndian.Uint16(batch[623:625]))
	if count < 1 || count > 32 || len(batch) != 625+count*259+64 {
		return nil, ErrInvalid
	}
	result := make([]byte, 16347)
	copy(result, "ARDIOR01")
	result[8] = 1
	result[9] = byte(count)
	signer := blindrsa.NewSigner(key)
	for i := 0; i < count; i++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		sig, err := signer.BlindSign(batch[628+i*259 : 884+i*259])
		if err != nil || len(sig) != 256 {
			return nil, ErrUnavailable
		}
		copy(result[10+i*256:], sig)
	}
	if err := s.checkIdentity(); err != nil {
		return nil, err
	}
	return result, nil
}
