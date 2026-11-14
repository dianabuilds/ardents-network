//go:build linux && descriptor_installed_native

package descriptor_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/custody"
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
)

// A separate finite helper admits a new holder through genuine retained
// Custody/issuer quota/results. Volatile unused rights from an earlier helper
// are not reconstructed, copied or refunded. Every new physical operation owns
// a fresh presentation and the original absolute channel deadline.
func TestDescriptorInstalledNativeReplacementExchange(t *testing.T) {
	descriptorNativeStand(t)
	stage := os.Getenv("ARDENTS_DESCRIPTOR_REPLACEMENT_STAGE")
	if stage != "resource" && stage != "program" {
		t.Fatal("invalid environment: select one exact replacement stage")
	}
	parent := filepath.Join(descriptorNativeParent, "replacement-"+stage)
	var input struct {
		Authority, Peer [32]byte
		Lifecycle       bool
	}
	if err := json.Unmarshal(descriptorNativeRead(t, "exchange-inputs.json"), &input); err != nil || !input.Lifecycle {
		t.Fatal("original installed lifecycle inputs required", err)
	}
	var created custody.Receipt
	if err := json.Unmarshal(descriptorNativeRead(t, "permission-origin.json"), &created); err != nil {
		t.Fatal(err)
	}
	password := descriptorNativeRead(t, "permission-secret")
	defer clear(password)
	vault, err := custody.Open(custody.VaultConfig{Root: filepath.Join(descriptorNativeParent, "permission-vault")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := vault.Close(); err != nil {
			t.Error(err)
		}
	})
	p := prepareDescriptorStock(t, parent, descriptorNativeParent, false, false, vault, created, descriptorTokenTestSecrets{password}, descriptorNativeRead(t, "bundle.bin"), descriptorNativeRead(t, "profile.bin"), input.Authority, 16, input.Peer)
	var original struct {
		Target, First, Conflict, Second, Foreign, Future [32]byte
		Lifecycle                                        bool
	}
	if err := json.Unmarshal(descriptorNativeRead(t, "exchange-prepared.json"), &original); err != nil || !original.Lifecycle {
		t.Fatal("original public proof commitments required", err)
	}
	first, second := descriptorNativeRead(t, "descriptor-proof-1.bin"), descriptorNativeRead(t, "descriptor-proof-2.bin")
	if sha256.Sum256(first) != original.First || sha256.Sum256(second) != original.Second {
		t.Fatal("original signed proof bytes changed")
	}
	peer := hex.EncodeToString(input.Peer[:])[:2]
	certificate, err := tls.LoadX509KeyPair(filepath.Join(descriptorNativeParent, "delivery/peer-certificate/node-certificate.pem"), filepath.Join(descriptorNativeParent, "canonical-preparation/node-key-"+peer, "node-key.pem"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if key, ok := certificate.PrivateKey.(ed25519.PrivateKey); ok {
			clear(key)
		}
		certificate.PrivateKey = nil
	}()
	prepared, err := json.Marshal(struct {
		Stage                 string
		Target, First, Second [32]byte
	}{stage, original.Target, original.First, original.Second})
	if err != nil {
		t.Fatal(err)
	}
	declared, err := os.ReadFile(filepath.Join(parent, "exchange-prepared.json"))
	if err != nil || !bytes.Equal(declared, prepared) {
		t.Fatal("exact public replacement proof input differs", err)
	}
	file, err := os.OpenFile(filepath.Join(parent, "exchange-ready"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	ready := []byte("original replacement rights prepared\n")
	n, writeErr := file.Write(ready)
	syncErr, closeErr := file.Sync(), file.Close()
	if writeErr != nil || n != len(ready) || syncErr != nil || closeErr != nil {
		t.Fatal("genuine original holder readiness", writeErr, syncErr, closeErr)
	}
	timer, ticker := time.NewTimer(time.Minute), time.NewTicker(20*time.Millisecond)
	defer timer.Stop()
	defer ticker.Stop()
	for {
		raw, err := os.ReadFile(filepath.Join(parent, "exchange-start"))
		if err == nil {
			if string(raw) != "original replacement Descriptor ready\n" {
				t.Fatal("foreign replacement exchange start")
			}
			break
		}
		if !os.IsNotExist(err) {
			t.Fatal(err)
		}
		select {
		case <-t.Context().Done():
			t.Fatal(t.Context().Err())
		case <-timer.C:
			t.Fatal("original replacement startup unavailable")
		case <-ticker.C:
		}
	}
	perform := func(name string, request ardp.DescriptorRequest, expectedStatus uint8, expected []byte) {
		t.Helper()
		before := descriptorNativeRead(t, "runtime/spend/closed-token-spends")
		status, raw := descriptorNativeOperation(t, p, certificate, request)
		if status != expectedStatus || !bytes.Equal(raw, expected) {
			t.Fatal(name, "exact retained result differs", status, expectedStatus)
		}
		after := descriptorNativeRead(t, "runtime/spend/closed-token-spends")
		if len(after) != len(before)+41 || !bytes.Equal(before, after[:len(before)]) || after[len(after)-1] != 1 {
			t.Fatal(name, "original irreversible receiving history changed")
		}
		t.Log(name, "actual operation and physical join verified")
	}
	perform("replacement exact revision2 lookup", ardp.DescriptorRequest{Operation: ardp.DescriptorLookup, Target: original.Target}, 0, second)
	perform("replacement lower revision refusal", ardp.DescriptorRequest{Operation: ardp.DescriptorPublish, Proof: first}, 3, nil)
	perform("replacement refusal preserved revision2", ardp.DescriptorRequest{Operation: ardp.DescriptorLookup, Target: original.Target}, 0, second)
}
