package issuance

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/binary"
)

const materialMagic = "ARDIKM01"
const maximumMaterial = 64 << 10

func generate(ctx context.Context, b Binding) ([]byte, error) {
	raw := append([]byte(materialMagic), bindingBytes(b)...)
	count := int((b.End.Unix()-b.Start.Unix())/3600) * 3
	raw = binary.BigEndian.AppendUint16(raw, uint16(count))
	for i := 0; i < count; i++ {
		if err := ctx.Err(); err != nil {
			clear(raw)
			return nil, err
		}
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			clear(raw)
			return nil, ErrUnavailable
		}
		if key.Validate() != nil || len(key.Primes) != 2 || key.E != 65537 || key.N.BitLen() != 2048 {
			clear(raw)
			return nil, ErrUnavailable
		}
		der := x509.MarshalPKCS1PrivateKey(key)
		raw = binary.BigEndian.AppendUint64(raw, uint64(b.Start.Unix()+int64(i/3)*3600))
		raw = append(raw, byte(i%3+1))
		raw = binary.BigEndian.AppendUint16(raw, uint16(len(der)))
		raw = append(raw, der...)
		clear(der)
	}
	if ctx.Err() != nil {
		clear(raw)
		return nil, ctx.Err()
	}
	return raw, nil
}

func decodeMaterial(raw []byte, b Binding) (Inventory, error) {
	if !b.valid() || len(raw) > maximumMaterial || len(raw) < 122 || string(raw[:8]) != materialMagic || !bytes.Equal(raw[8:120], bindingBytes(b)) {
		return Inventory{}, ErrUnavailable
	}
	count := int(binary.BigEndian.Uint16(raw[120:122]))
	if count != int((b.End.Unix()-b.Start.Unix())/3600)*3 {
		return Inventory{}, ErrUnavailable
	}
	inventory := Inventory{Binding: b}
	seen := map[[32]byte]bool{}
	offset := 122
	for i := 0; i < count; i++ {
		if len(raw)-offset < 11 {
			return Inventory{}, ErrUnavailable
		}
		window := binary.BigEndian.Uint64(raw[offset:])
		class := raw[offset+8]
		n := int(binary.BigEndian.Uint16(raw[offset+9:]))
		offset += 11
		if window != uint64(b.Start.Unix()+int64(i/3)*3600) || class != byte(i%3+1) || n == 0 || len(raw)-offset < n {
			return Inventory{}, ErrUnavailable
		}
		der := raw[offset : offset+n]
		offset += n
		key, err := x509.ParsePKCS1PrivateKey(der)
		if err != nil || key.Validate() != nil || len(key.Primes) != 2 || key.E != 65537 || key.N.BitLen() != 2048 || !bytes.Equal(der, x509.MarshalPKCS1PrivateKey(key)) {
			return Inventory{}, ErrUnavailable
		}
		spki, err := tokenKeyEncoding(&key.PublicKey)
		if err != nil || len(spki) != 346 {
			return Inventory{}, ErrUnavailable
		}
		if _, valid := tokenKey(spki); !valid {
			return Inventory{}, ErrUnavailable
		}
		digest := sha256.Sum256(spki)
		if seen[digest] {
			return Inventory{}, ErrUnavailable
		}
		seen[digest] = true
		inventory.Keys = append(inventory.Keys, PublicKey{Window: window, Class: class, SPKI: spki})
	}
	if offset != len(raw) {
		return Inventory{}, ErrUnavailable
	}
	inventory.Digest = sha256.Sum256(inventory.canonical())
	return inventory, nil
}
