//go:build linux && network_signing_native

package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	sourcefixture "github.com/dianabuilds/ardents-network/tests/e2e/source"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/node/provisioning"
)

func nativeProfileSigningArguments(t *testing.T) []string {
	t.Helper()
	parent := sourcefixture.SigningWorkspace(t)
	now := time.Now().UTC().Truncate(time.Second)
	p, key, _, _ := signingProfileFixture(t, now.Add(-time.Hour))
	defer clear(key)
	// One complete previous-hour token window allows an actual imminent
	// profile expiry without changing or shortening the token-key grammar.
	keys := p["token_keys"].([]map[string]any)
	previous := now.Truncate(time.Hour).Add(-time.Hour).Format(time.RFC3339)
	var retained []map[string]any
	for _, k := range keys {
		if k["window_start"] == previous {
			retained = append(retained, k)
		}
	}
	p["token_keys"] = retained
	return writeNativeProfileArguments(t, parent, p, key, now)
}

func writeNativeProfileArguments(t *testing.T, parent string, p map[string]any, key ed25519.PrivateKey, reference time.Time) []string {
	t.Helper()
	_, body, e := provisioning.PrepareNetworkPublic("prepare-closed-profile", preparationJSON(t, p))
	if e != nil {
		t.Fatal(e)
	}
	digest := sha256.Sum256(body)
	plan, prepared, secret := filepath.Join(parent, "plan"), filepath.Join(parent, "prepared"), filepath.Join(parent, "key")
	if e = os.WriteFile(plan, profileSigningDeclaration(t, p, key.Public().(ed25519.PublicKey), reference), 0600); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(prepared, body, 0600); e != nil {
		t.Fatal(e)
	}
	writeSigningTestKey(t, secret, key)
	return []string{"sign-closed-profile", plan, prepared, preparationHex(digest[:]), secret, filepath.Join(parent, "signed")}
}

func TestNetworkSigningProfileReceiptClaims(t *testing.T) {
	var receipt bytes.Buffer
	if code := runNetwork(t.Context(), nativeProfileSigningArguments(t), &receipt); code != 0 || !bytes.Contains(receipt.Bytes(), []byte(`"state_accepted":false`)) || !bytes.Contains(receipt.Bytes(), []byte(`"installed_qualification":false`)) {
		t.Fatal("profile receipt", code, receipt.String())
	}
}

type delayedProfileReceipt struct {
	bytes.Buffer
	until   time.Time
	entered bool
}

func (w *delayedProfileReceipt) Write(raw []byte) (int, error) {
	w.entered = true
	if d := time.Until(w.until); d > 0 {
		time.Sleep(d + time.Millisecond)
	}
	return w.Buffer.Write(raw)
}

func TestNetworkSigningProfileReceiptExpiry(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	p, key, _, _ := signingProfileFixture(t, now.Add(-time.Hour))
	defer clear(key)
	var retained []map[string]any
	for _, k := range p["token_keys"].([]map[string]any) {
		if k["window_start"] == now.Truncate(time.Hour).Add(-time.Hour).Format(time.RFC3339) {
			retained = append(retained, k)
		}
	}
	p["token_keys"] = retained
	until := time.Now().UTC().Truncate(time.Second).Add(3 * time.Second)
	p["not_after"] = until.Format(time.RFC3339)
	args := writeNativeProfileArguments(t, sourcefixture.SigningWorkspace(t), p, key, now)
	w := delayedProfileReceipt{until: until}
	if code := runNetwork(t.Context(), args, &w); code != 1 || !w.entered {
		t.Fatal("late acknowledgement accepted or delay unexercised", code)
	}
	// Output and any partially delivered report remain retained, while the
	// original invocation has failed. Neither can be adopted or automatically signed again.
	if _, err := os.Stat(filepath.Join(args[5], "profile.bin")); err != nil {
		t.Fatal("original output lost", err)
	}
	if _, err := signNetworkFixture(t.Context(), args); err == nil {
		t.Fatal("failed acknowledgement revived")
	}
	if _, err := signNetworkFixture(t.Context(), nativeProfileSigningArguments(t)); err != nil {
		t.Fatal("healthy receipt control", err)
	}
}
