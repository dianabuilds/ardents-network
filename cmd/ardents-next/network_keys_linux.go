//go:build linux

package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
)

func admitInitialKeyPlan(raw []byte) (int, error) {
	var p struct {
		Schema    string `json:"schema"`
		Operation string `json:"operation"`
		Count     int    `json:"node_count"`
	}
	if len(raw) > 1024 || preparationObject(raw, &p, "schema", "operation", "node_count") != nil ||
		p.Schema != "ardents-initial-keys-plan-v1" || p.Operation != "prepare-initial-keys" || p.Count < 1 || p.Count > 64 {
		return 0, errInitialKeys
	}
	return p.Count, nil
}

// All returned plaintext belongs to the caller until its synchronous native
// write/readback/Close completes. ASN.1 containers and the seed copy are cleared
// here. Standard-library/runtime internal copies are outside this guarantee.
func encodeInitialKey(key ed25519.PrivateKey) ([]byte, error) {
	if len(key) != ed25519.PrivateKeySize {
		return nil, errInitialKeys
	}
	seed := key.Seed()
	defer clear(seed)
	inner, err := asn1.Marshal(seed)
	defer clear(inner)
	if err != nil {
		return nil, err
	}
	der, err := asn1.Marshal(struct {
		Version    int
		Algorithm  pkix.AlgorithmIdentifier
		PrivateKey []byte
	}{0, pkix.AlgorithmIdentifier{Algorithm: asn1.ObjectIdentifier{1, 3, 101, 112}}, inner})
	defer clear(der)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), nil
}

// The production path always uses OS-backed crypto/rand. The isolated producer
// seam below permits causal failure/duplicate controls, never a successful
// supplied entropy source at the real command entrypoint.
func executeInitialKeys(ctx context.Context, plan, destination string) (*initialKeyReceipt, error) {
	return prepareInitialKeys(ctx, plan, destination, func() (ed25519.PublicKey, ed25519.PrivateKey, error) {
		return ed25519.GenerateKey(rand.Reader)
	}, encodeInitialKey, nil)
}

func prepareInitialKeys(ctx context.Context, plan, destination string,
	generate func() (ed25519.PublicKey, ed25519.PrivateKey, error),
	encode func(ed25519.PrivateKey) ([]byte, error), observe func(string, *os.File)) (receipt *initialKeyReceipt, err error) {
	if os.Geteuid() != 0 {
		return nil, errInitialKeys
	}
	input, err := openSigningInput(ctx, plan, 1024, false)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, input.close(), ctx.Err()) }()
	count, err := admitInitialKeyPlan(input.raw)
	if err != nil {
		return nil, err
	}
	receipt = &initialKeyReceipt{Schema: "ardents-initial-keys-receipt-v1", Operation: "prepare-initial-keys"}
	var files []networkOutputFile
	defer func() {
		for _, f := range files {
			clear(f.raw)
		}
		if err != nil {
			receipt = nil
		}
	}()
	seen := map[string]bool{}
	for i := 0; i <= count; i++ {
		if err = errors.Join(input.check(ctx), ctx.Err()); err != nil {
			return nil, err
		}
		pub, key, e := generate()
		// Even a failing producer may return owned material. Erasure never
		// depends on its success, and no diagnostic contains that material.
		if e != nil {
			clear(key)
			return nil, e
		}
		if len(key) != ed25519.PrivateKeySize || len(pub) != ed25519.PublicKeySize || !bytes.Equal(key[32:], pub) || seen[string(pub)] {
			clear(key)
			return nil, errInitialKeys
		}
		seen[string(pub)] = true
		raw, e := encode(key)
		clear(key)
		if e != nil {
			clear(raw)
			return nil, e
		}
		// Canonical complete encoding and seed/public consistency use the
		// independent existing private-input admission before any output.
		parsed, e := parseNetworkSigningKey(raw)
		matches := e == nil && bytes.Equal(parsed[32:], pub)
		clear(parsed)
		if !matches {
			clear(raw)
			return nil, errInitialKeys
		}
		purpose, name := "state", "state-key.pem"
		var index *int
		if i > 0 {
			n := i - 1
			index = &n
			purpose, name = "node", fmt.Sprintf("node-%02d-key.pem", n)
		}
		digest := sha256.Sum256(pub)
		receipt.Keys = append(receipt.Keys, initialKeyPublic{purpose, index, name, hex.EncodeToString(pub), hex.EncodeToString(digest[:])})
		files = append(files, networkOutputFile{name, raw})
	}
	public, err := json.Marshal(receipt)
	if err != nil {
		return nil, err
	}
	files = append(files, networkOutputFile{"public-receipt.json", append(public, '\n')})
	err = persistNetworkFiles(ctx, destination, files, true, func() error { return input.check(ctx) }, observe)
	return receipt, err
}
