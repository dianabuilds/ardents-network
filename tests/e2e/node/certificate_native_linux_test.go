//go:build linux && network_signing_native

package node_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	sourcefixture "github.com/dianabuilds/ardents-network/tests/e2e/source"
)

func TestNetworkSigningNodeCertificateCommand(t *testing.T) {
	commandBinary := buildNodeCommand(t)
	args := sourcefixture.CertificateArguments(t, commandBinary, sourcefixture.SigningWorkspace(t))
	before, err := os.ReadFile(args[4])
	if err != nil {
		t.Fatal(err)
	}
	defer clear(before)
	wrongState := append([]string(nil), args...)
	wrongState[4] = filepath.Join(filepath.Dir(args[4]), "state-key.pem")
	if err := exec.CommandContext(t.Context(), commandBinary, append([]string{"network"}, wrongState...)...).Run(); err == nil {
		t.Fatal("generated State key substituted for Node")
	}
	if _, err := os.Stat(args[5]); !os.IsNotExist(err) {
		t.Fatal("wrong key created output", err)
	}
	command := exec.CommandContext(t.Context(), commandBinary, append([]string{"network"}, args...)...)
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
	if json.Unmarshal(stdout, &receipt) != nil || receipt.Operation != args[0] || receipt.Digest != hex.EncodeToString(digest[:]) || !receipt.Verified || receipt.State || receipt.Installed {
		t.Fatal("public receipt claims")
	}
	peer, err := tls.X509KeyPair(raw, before)
	if err != nil {
		t.Fatal("runtime key-pair compatibility", err)
	}
	leaf, err := x509.ParseCertificate(peer.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	key := peer.PrivateKey.(ed25519.PrivateKey)
	defer clear(key)
	if !leaf.PublicKey.(ed25519.PublicKey).Equal(key.Public()) || leaf.CheckSignature(leaf.SignatureAlgorithm, leaf.RawTBSCertificate, leaf.Signature) != nil {
		t.Fatal("certificate does not authenticate the generated Node key")
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
	if err := exec.CommandContext(t.Context(), commandBinary, append([]string{"network"}, args...)...).Run(); err == nil {
		t.Fatal("completed output adopted")
	}
}
