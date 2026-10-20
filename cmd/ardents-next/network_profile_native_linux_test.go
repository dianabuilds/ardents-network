//go:build linux && network_signing_native

package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func nativeProfileSigningArguments(t *testing.T) []string {
	t.Helper()
	parent := signingNativeParent(t)
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
	_, body, e := prepareNetworkPublic("prepare-closed-profile", preparationJSON(t, p))
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

func TestNetworkSigningProfileNativeCommit(t *testing.T) {
	for _, fault := range []string{"healthy", "file replacement", "extra", "late bytes", "commit bytes", "writer close", "directory close", "cancel"} {
		t.Run(fault, func(t *testing.T) {
			args := nativeProfileSigningArguments(t)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			acted := false
			_, err := executeObservedNetworkSigning(ctx, args, func(stage string, f *os.File) {
				var e error
				if stage == "file-created" && !acted {
					switch fault {
					case "file replacement":
						e = os.Rename(filepath.Join(args[5], "profile.bin"), filepath.Join(filepath.Dir(args[5]), "original"))
						if e == nil {
							e = os.WriteFile(filepath.Join(args[5], "profile.bin"), []byte("changed"), 0600)
						}
						acted = true
					case "writer close":
						e = f.Close()
						acted = true
					case "cancel":
						cancel()
						acted = true
					}
				}
				if stage == "before-directory-sync" && !acted {
					switch fault {
					case "extra":
						e = os.WriteFile(filepath.Join(args[5], "extra"), []byte("retained"), 0600)
						acted = true
					case "late bytes":
						e = os.WriteFile(filepath.Join(args[5], "profile.bin"), []byte("changed"), 0600)
						acted = true
					}
				}
				if stage == "directory-sync" && fault == "directory close" {
					e = f.Close()
					acted = true
				}
				if stage == "commit" && fault == "commit bytes" {
					e = os.WriteFile(filepath.Join(args[5], "profile.bin"), []byte("changed after input reobservation"), 0600)
					acted = true
				}
				if e != nil {
					t.Fatal(e)
				}
			})
			if (err == nil) != (fault == "healthy") || (fault != "healthy" && !acted) {
				t.Fatal("actual profile commit verdict", err)
			}
			if fault == "writer close" && !errors.Is(err, os.ErrClosed) {
				t.Fatal("original physical failures lost", err)
			}
			if _, e := executeNetworkSigning(t.Context(), args); e == nil {
				t.Fatal("signed or incomplete output adopted")
			}
			if _, e := executeNetworkSigning(t.Context(), nativeProfileSigningArguments(t)); e != nil {
				t.Fatal("healthy profile control", e)
			}
		})
	}
}

func TestNetworkSigningProfileActualExpiry(t *testing.T) {
	for _, barrier := range []string{"inputs-read", "before-directory-sync"} {
		t.Run(barrier, func(t *testing.T) {
			now := time.Now().UTC().Truncate(time.Second)
			p, key, _, _ := signingProfileFixture(t, now.Add(-time.Hour))
			defer clear(key)
			keys := p["token_keys"].([]map[string]any)
			var retained []map[string]any
			for _, k := range keys {
				if k["window_start"] == now.Truncate(time.Hour).Add(-time.Hour).Format(time.RFC3339) {
					retained = append(retained, k)
				}
			}
			p["token_keys"] = retained
			until := time.Now().UTC().Truncate(time.Second).Add(3 * time.Second)
			p["not_after"] = until.Format(time.RFC3339)
			args := writeNativeProfileArguments(t, signingNativeParent(t), p, key, now)
			observed := false
			_, err := executeObservedNetworkSigning(t.Context(), args, func(step string, _ *os.File) {
				if step == barrier {
					observed = true
					if d := time.Until(until); d > 0 {
						time.Sleep(d + time.Millisecond)
					}
				}
			})
			if !observed || err == nil {
				t.Fatal("late actual profile accepted", err)
			}
			_, e := os.Stat(args[5])
			if barrier == "inputs-read" && !os.IsNotExist(e) {
				t.Fatal("expired input created output", e)
			}
			if barrier != "inputs-read" && e != nil {
				t.Fatal("incomplete output removed", e)
			}
			if _, e = executeNetworkSigning(t.Context(), nativeProfileSigningArguments(t)); e != nil {
				t.Fatal("healthy profile control", e)
			}
		})
	}
}

func TestNetworkSigningProfileContention(t *testing.T) {
	args := nativeProfileSigningArguments(t)
	var wg sync.WaitGroup
	start := make(chan struct{})
	results := make(chan error, 2)
	for range 2 {
		wg.Go(func() { <-start; _, e := executeNetworkSigning(t.Context(), args); results <- e })
	}
	close(start)
	wg.Wait()
	close(results)
	winners := 0
	for e := range results {
		if e == nil {
			winners++
		}
	}
	if winners != 1 {
		t.Fatal("actual profile winners", winners)
	}
	files, e := os.ReadDir(args[5])
	if e != nil || len(files) != 1 || files[0].Name() != "profile.bin" {
		t.Fatal("finite profile inventory", e)
	}
	var receipt bytes.Buffer
	if code := runNetworkSigning(t.Context(), nativeProfileSigningArguments(t), &receipt); code != 0 || !bytes.Contains(receipt.Bytes(), []byte(`"state_accepted":false`)) || !bytes.Contains(receipt.Bytes(), []byte(`"installed_qualification":false`)) {
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
	args := writeNativeProfileArguments(t, signingNativeParent(t), p, key, now)
	w := delayedProfileReceipt{until: until}
	if code := runNetworkSigning(t.Context(), args, &w); code != 1 || !w.entered {
		t.Fatal("late acknowledgement accepted or delay unexercised", code)
	}
	// Output and any partially delivered report remain retained, while the
	// original invocation has failed. Neither can be adopted or automatically signed again.
	if _, err := os.Stat(filepath.Join(args[5], "profile.bin")); err != nil {
		t.Fatal("original output lost", err)
	}
	if _, err := executeNetworkSigning(t.Context(), args); err == nil {
		t.Fatal("failed acknowledgement revived")
	}
	if _, err := executeNetworkSigning(t.Context(), nativeProfileSigningArguments(t)); err != nil {
		t.Fatal("healthy receipt control", err)
	}
}
