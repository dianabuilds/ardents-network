//go:build linux && network_signing_native

package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// The complete production key/preparation/Record/certificate chain runs before
// each native custody control. State/Admission authorities remain separate.
func nativeCertificateArguments(t *testing.T) []string {
	t.Helper()
	keyPlan, keyRoot := initialKeyArguments(t, 1)
	command := compiledCommand(t)
	run := func(args ...string) {
		t.Helper()
		if raw, err := exec.CommandContext(t.Context(), command, args...).CombinedOutput(); err != nil {
			t.Fatal("genuine provisioning command", err, string(raw))
		}
	}
	run("network", "prepare-initial-keys", keyPlan, keyRoot)
	keys := verifyInitialKeys(t, keyRoot, 1, nil)
	parent := signingNativeParent(t)
	p := preparationRecordPlan(ed25519.PublicKey(keys[1][32:]))
	now := time.Now().UTC().Truncate(time.Second)
	p["valid_from"], p["valid_until"] = now.Add(-time.Minute).Format(time.RFC3339), now.Add(time.Hour).Format(time.RFC3339)
	plan, prepared, recordPlan, signedRoot := filepath.Join(parent, "record-plan"), filepath.Join(parent, "prepared"), filepath.Join(parent, "signing-plan"), filepath.Join(parent, "signed")
	if err := os.WriteFile(plan, preparationJSON(t, p), 0600); err != nil {
		t.Fatal(err)
	}
	run("network", "prepare-initial-node-record", plan, prepared)
	message, err := os.ReadFile(filepath.Join(prepared, "node-record.signing-input"))
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(message)
	if err := os.WriteFile(recordPlan, signingDeclaration(t, "node-record", p, now), 0600); err != nil {
		t.Fatal(err)
	}
	key := filepath.Join(keyRoot, "node-00-key.pem")
	run("network", "sign-initial-node-record", recordPlan, filepath.Join(prepared, "node-record.signing-input"), preparationHex(digest[:]), key, signedRoot)
	record := filepath.Join(signedRoot, "node-record.bin")
	raw, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	digest = sha256.Sum256(raw)
	declaration := filepath.Join(parent, "certificate-plan")
	if err := os.WriteFile(declaration, certificateDeclaration(t, p, now), 0600); err != nil {
		t.Fatal(err)
	}
	return []string{"prepare-node-certificate", declaration, record, preparationHex(digest[:]), key, filepath.Join(parent, "certificate")}
}

func TestNetworkSigningNodeCertificateCommand(t *testing.T) {
	args := nativeCertificateArguments(t)
	before, err := os.ReadFile(args[4])
	if err != nil {
		t.Fatal(err)
	}
	defer clear(before)
	wrongState := append([]string(nil), args...)
	wrongState[4] = filepath.Join(filepath.Dir(args[4]), "state-key.pem")
	if code := runNetworkSigning(t.Context(), wrongState, &bytes.Buffer{}); code == 0 {
		t.Fatal("generated State key substituted for Node")
	}
	if _, err := os.Stat(args[5]); !os.IsNotExist(err) {
		t.Fatal("wrong key created output", err)
	}
	command := exec.CommandContext(t.Context(), compiledCommand(t), append([]string{"network"}, args...)...)
	stdout, err := command.Output()
	if err != nil {
		t.Fatal("actual certificate producer", err)
	}
	var receipt struct {
		Operation string `json:"operation"`
		Digest    string `json:"signed_sha256"`
		Verified  bool   `json:"signatures_verified"`
		State     bool   `json:"state_accepted"`
		Installed bool   `json:"installed_qualification"`
	}
	raw, err := os.ReadFile(filepath.Join(args[5], "node-certificate.pem"))
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(raw)
	if json.Unmarshal(stdout, &receipt) != nil || receipt.Operation != args[0] || receipt.Digest != preparationHex(digest[:]) || !receipt.Verified || receipt.State || receipt.Installed {
		t.Fatal("public receipt claims")
	}
	entries, err := os.ReadDir(args[5])
	if err != nil || len(entries) != 1 || entries[0].Name() != "node-certificate.pem" {
		t.Fatal("single public file", err)
	}
	after, err := os.ReadFile(args[4])
	if err != nil {
		t.Fatal(err)
	}
	defer clear(after)
	if !bytes.Equal(before, after) {
		t.Fatal("original key changed")
	}
	if code := runNetworkSigning(t.Context(), args, &bytes.Buffer{}); code == 0 {
		t.Fatal("completed output adopted")
	}
}

func TestNetworkSigningNodeCertificateLateTime(t *testing.T) {
	for _, stage := range []string{"inputs-read", "before-directory-sync"} {
		t.Run(stage, func(t *testing.T) {
			args := nativeCertificateArguments(t)
			now := time.Now().UTC().Truncate(time.Second)
			until := now.Add(3 * time.Second)
			changeSigningTestTime(t, args, now.Add(-time.Minute), until, now, true)
			acted := false
			_, err := executeObservedNetworkSigning(t.Context(), args, func(step string, _ *os.File) {
				if step == stage {
					acted = true
					if delay := time.Until(until); delay > 0 {
						time.Sleep(delay + time.Millisecond)
					}
				}
			})
			if !acted || err == nil {
				t.Fatal("late original certificate accepted", err)
			}
			if stage == "inputs-read" {
				if _, err := os.Stat(args[5]); !os.IsNotExist(err) {
					t.Fatal("expired before crypto created output", err)
				}
			}
		})
	}
}

func TestNetworkSigningNodeCertificateFinalOutputAndPhysicalFailure(t *testing.T) {
	for _, fault := range []string{"bytes", "inventory", "canceled-close"} {
		t.Run(fault, func(t *testing.T) {
			args := nativeCertificateArguments(t)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			acted := false
			_, err := executeObservedNetworkSigning(ctx, args, func(step string, f *os.File) {
				if fault == "canceled-close" && step == "signed" {
					acted = true
					if e := f.Close(); e != nil {
						t.Fatal(e)
					}
					cancel()
				}
				if step == "commit" && fault != "canceled-close" {
					acted = true
					name := "node-certificate.pem"
					if fault == "inventory" {
						name = "foreign"
					}
					if e := os.WriteFile(filepath.Join(args[5], name), []byte("changed after original input IO"), 0600); e != nil {
						t.Fatal(e)
					}
				}
			})
			if !acted || err == nil {
				t.Fatal("late original failure accepted", err)
			}
			if fault == "canceled-close" && (!errors.Is(err, context.Canceled) || !errors.Is(err, os.ErrClosed)) {
				t.Fatal("independent physical/cancellation errors lost", err)
			}
		})
	}
}

func TestNetworkSigningNodeCertificateReceiptAndContention(t *testing.T) {
	for _, writer := range []io.Writer{initialKeysFailedWriter{}, initialKeysShortWriter{}} {
		args := nativeCertificateArguments(t)
		if runNetworkSigning(t.Context(), args, writer) == 0 {
			t.Fatal("failed receipt accepted")
		}
		if runNetworkSigning(t.Context(), args, &bytes.Buffer{}) == 0 {
			t.Fatal("failed receipt output adopted")
		}
	}
	args := nativeCertificateArguments(t)
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Go(func() { <-start; _, err := executeNetworkSigning(t.Context(), args); results <- err })
	}
	close(start)
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatal("fresh output winners", success)
	}
}
