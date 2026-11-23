//go:build linux

package provisioning

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	artifactio "github.com/dianabuilds/ardents-network/internal/platform/artifacts"
)

var errInitialKeys = errors.New("initial Network key preparation unavailable")

type initialKeyPublic struct {
	Purpose string `json:"purpose"`
	Index   *int   `json:"index,omitempty"`
	File    string `json:"file"`
	Public  string `json:"ed25519_public_hex"`
	Digest  string `json:"public_sha256"`
}

type initialKeyReceipt struct {
	Schema      string             `json:"schema"`
	Operation   string             `json:"operation"`
	Keys        []initialKeyPublic `json:"keys"`
	Signed      bool               `json:"signed"`
	State       bool               `json:"state_accepted"`
	Installed   bool               `json:"installed_qualification"`
	Independent bool               `json:"independent_custody"`
}

func admitInitialKeyPlan(raw []byte) (int, error) {
	var p struct {
		Schema    string `json:"schema"`
		Operation string `json:"operation"`
		Count     int    `json:"node_count"`
	}
	if len(raw) > 1024 || artifactio.DecodeDeclaration(raw, &p, "schema", "operation", "node_count") != nil ||
		p.Schema != "ardents-initial-keys-plan-v1" || p.Operation != "prepare-initial-keys" || p.Count < 1 || p.Count > 64 {
		return 0, errInitialKeys
	}
	return p.Count, nil
}

// PrepareInitialNetworkKeys produces one State key and the exact number of
// distinct Node keys declared in the original public plan. It returns the
// original public receipt only after the exclusive output is joined.
func PrepareInitialNetworkKeys(ctx context.Context, plan, destination string) ([]byte, error) {
	return prepareInitialKeys(ctx, plan, destination, func() (ed25519.PublicKey, ed25519.PrivateKey, error) {
		return ed25519.GenerateKey(rand.Reader)
	}, artifactio.EncodeEd25519Key, nil)
}

func prepareInitialKeys(ctx context.Context, plan, destination string,
	generate func() (ed25519.PublicKey, ed25519.PrivateKey, error),
	encode func(ed25519.PrivateKey) ([]byte, error), observe func(string, *os.File)) (receipt []byte, err error) {
	if os.Geteuid() != 0 {
		return nil, errInitialKeys
	}
	input, err := artifactio.OpenProtectedInput(ctx, plan, 1024, false)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = errors.Join(err, input.Close(), ctx.Err())
		if err != nil {
			receipt = nil
		}
	}()
	count, err := admitInitialKeyPlan(input.Bytes())
	if err != nil {
		return nil, err
	}
	facts := initialKeyReceipt{Schema: "ardents-initial-keys-receipt-v1", Operation: "prepare-initial-keys"}
	var files []artifactio.Content
	defer func() {
		for _, f := range files {
			clear(f.Data)
		}
	}()
	seen := map[string]bool{}
	for i := 0; i <= count; i++ {
		if err = errors.Join(input.Check(ctx), ctx.Err()); err != nil {
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
		parsed, e := artifactio.ParseEd25519Key(raw)
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
		facts.Keys = append(facts.Keys, initialKeyPublic{purpose, index, name, hex.EncodeToString(pub), hex.EncodeToString(digest[:])})
		files = append(files, artifactio.Content{Name: name, Data: raw})
	}
	public, err := json.Marshal(facts)
	if err != nil {
		return nil, err
	}
	public = append(public, '\n')
	inventory := append(files, artifactio.Content{Name: "public-receipt.json", Data: public})
	if err = artifactio.PublishInventory(ctx, destination, inventory, true, func() error { return input.Check(ctx) }, observe); err != nil {
		return nil, err
	}
	return public, nil
}
