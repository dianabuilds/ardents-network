//go:build linux && descriptor_installed_native

package descriptor_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/custody"
)

const descriptorNativeParent = "/var/lib/descriptor-initial"

func descriptorNativeStand(t *testing.T) {
	t.Helper()
	marker, err := os.ReadFile("/etc/ardents-descriptor-initial-native-stand")
	if err != nil || string(marker) != "ardents-descriptor-initial-native-v1\n" || os.Geteuid() != 0 {
		t.Fatal("invalid environment: isolated Descriptor stand required", err)
	}
}

// Only public receipts cross the two launches. Encrypted Custody records and a
// fresh local password remain private to this disposable native stand.
func TestDescriptorInstalledNativeAuthority(t *testing.T) {
	descriptorNativeStand(t)
	password := make([]byte, 32)
	if _, err := rand.Read(password); err != nil {
		t.Fatal(err)
	}
	defer clear(password)
	secret := []byte(hex.EncodeToString(password))
	defer clear(secret)
	vault, err := custody.Open(custody.VaultConfig{Root: filepath.Join(descriptorNativeParent, "permission-vault")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := vault.Close(); err != nil {
			t.Error(err)
		}
	})
	created, err := vault.Execute(t.Context(), custody.Operation{Kind: custody.OperationCreateAdmissionAuthority, Binding: custody.AuthorityBinding{Environment: sha256.Sum256([]byte("bounded installed Descriptor exchange")), Network: [32]byte{1}, Root: sha256.Sum256([]byte("separate Descriptor Permission authority")), Kind: custody.AuthorityAdmission}}, descriptorTokenTestSecrets{secret})
	if err != nil {
		t.Fatal(err)
	}
	descriptorNativeWrite(t, "permission-secret", secret, 0600)
	raw, err := json.Marshal(created)
	if err != nil {
		t.Fatal(err)
	}
	descriptorNativeWrite(t, "permission-origin.json", raw, 0600)
	raw, err = json.Marshal(struct {
		Public [32]byte `json:"public"`
	}{created.AdmissionAuthority.Public})
	if err != nil {
		t.Fatal(err)
	}
	descriptorNativeWrite(t, "permission-public.json", raw, 0600)
}

func descriptorNativeRead(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(descriptorNativeParent, name))
	if err != nil {
		t.Fatal(name, err)
	}
	return raw
}
func descriptorNativeWrite(t *testing.T, name string, raw []byte, mode os.FileMode) {
	t.Helper()
	f, err := os.OpenFile(filepath.Join(descriptorNativeParent, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		t.Fatal(err)
	}
	n, writeErr := f.Write(raw)
	syncErr := f.Sync()
	closeErr := f.Close()
	if writeErr != nil || n != len(raw) || syncErr != nil || closeErr != nil {
		t.Fatal("original native output", n, writeErr, syncErr, closeErr)
	}
}

func TestDescriptorInstalledNativeExchange(t *testing.T) {
	descriptorNativeStand(t)
	var input struct {
		Authority, Peer        [32]byte
		Lifecycle, Replacement bool
	}
	if err := json.Unmarshal(descriptorNativeRead(t, "exchange-inputs.json"), &input); err != nil {
		t.Fatal(err)
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
	prepared := prepareDescriptorStock(t, descriptorNativeParent, descriptorNativeParent, true, !input.Replacement, vault, created, descriptorTokenTestSecrets{password}, descriptorNativeRead(t, "bundle.bin"), descriptorNativeRead(t, "profile.bin"), input.Authority, 16, input.Peer)
	window := 5 * time.Minute
	if input.Replacement {
		window = 10 * time.Minute
	}
	proofs := prepareDescriptorNativeProofs(t, prepared, window)
	peerText := hex.EncodeToString(input.Peer[:])[:2]
	certificate, err := tls.LoadX509KeyPair(filepath.Join(descriptorNativeParent, "delivery/peer-certificate/node-certificate.pem"), filepath.Join(descriptorNativeParent, "canonical-preparation/node-key-"+peerText, "node-key.pem"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if key, ok := certificate.PrivateKey.(ed25519.PrivateKey); ok {
			clear(key)
		}
		certificate.PrivateKey = nil
	}()
	public, err := json.Marshal(struct {
		Target, First, Conflict, Second, Foreign, Future [32]byte
		Lifecycle                                        bool
	}{proofs.target, sha256.Sum256(proofs.first), sha256.Sum256(proofs.conflict), sha256.Sum256(proofs.second), sha256.Sum256(proofs.foreign), sha256.Sum256(proofs.future), input.Lifecycle})
	if err != nil {
		t.Fatal(err)
	}
	descriptorNativeWrite(t, "descriptor-proof-1.bin", proofs.first, 0444)
	descriptorNativeWrite(t, "descriptor-proof-2.bin", proofs.second, 0444)
	descriptorNativeWrite(t, "exchange-prepared.json", public, 0444)
	timer := time.NewTimer(time.Minute)
	defer timer.Stop()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		raw, err := os.ReadFile(filepath.Join(descriptorNativeParent, "exchange-start"))
		if err == nil {
			if !bytes.Equal(raw, []byte("original installed Descriptor ready\n")) {
				t.Fatal("foreign exchange start")
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
			t.Fatal("installed Descriptor start not observed")
		case <-ticker.C:
		}
	}
	if input.Lifecycle {
		runDescriptorNativeRestartExchange(t, prepared, certificate, proofs, input.Replacement)
	} else {
		runDescriptorNativeOperations(t, prepared, certificate, proofs)
	}
}

func descriptorNativePinnedProgram(t *testing.T, variable string) (string, []byte) {
	t.Helper()
	path, pin := os.Getenv(variable), os.Getenv(variable+"_SHA256")
	raw, err := os.ReadFile(path)
	digest := sha256.Sum256(raw)
	if err != nil || !filepath.IsAbs(path) || len(pin) != 64 || hex.EncodeToString(digest[:]) != pin {
		t.Fatal("invalid environment: original independently pinned program required", variable, err)
	}
	return path, raw
}
