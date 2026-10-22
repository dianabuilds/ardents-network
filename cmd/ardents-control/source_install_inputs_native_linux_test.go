//go:build linux && network_signing_native

package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Public credential preparation is offline. A nonexistent opaque private-key
// declaration and absent future roots must suffice; no installed factory runs.
func TestSourcePreparationNativeInstallInputsPublicCustody(t *testing.T) {
	parent := sourceArtifactNativeParent(t)
	now := time.Now().UTC()
	caPublic, caPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(caPrivate)
	ca := &x509.Certificate{SerialNumber: big.NewInt(1), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour)}
	caDER, err := x509.CreateCertificate(rand.Reader, ca, ca, caPublic, caPrivate)
	if err != nil {
		t.Fatal(err)
	}
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})
	issue := func(serial int64, usage x509.ExtKeyUsage) ([]byte, string) {
		public, private, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		defer clear(private)
		leaf := &x509.Certificate{SerialNumber: big.NewInt(serial), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{usage}, DNSNames: []string{"source.test"}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour)}
		der, err := x509.CreateCertificate(rand.Reader, leaf, ca, public, caPrivate)
		if err != nil {
			t.Fatal(err)
		}
		pin := sha256.Sum256(append([]byte("ardents-h3-source-transport-key-v1\x00"), public...))
		return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), hex.EncodeToString(pin[:])
	}
	server, serverPin := issue(2, x509.ExtKeyUsageServerAuth)
	client, clientPin := issue(3, x509.ExtKeyUsageClientAuth)
	digest := func(raw []byte) string { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }
	publics := map[string][]byte{"server.pem": server, "server-ca.pem": caPEM, "client.pem": client, "client-ca.pem": caPEM}
	for name, raw := range publics {
		if err := os.WriteFile(filepath.Join(parent, name), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	p := sourceInstallInputOptions{Network: strings.Repeat("11", 32), StateKey: strings.Repeat("22", 32), Identity: strings.Repeat("33", 32), Family: "one-effective-controller", Handle: "one-source", Address: "127.0.0.1:18443", ServerName: "source.test",
		Installation: "/var/lib/source-installation", State: "/var/lib/source-state", Participation: "/var/lib/source-participation", Credentials: "/var/lib/source-credentials", Clock: "/var/lib/source-clock", PrivateKey: filepath.Join(parent, "absent-runtime-private.pem"), Output: filepath.Join(parent, "prepared"),
		Certificate: filepath.Join(parent, "server.pem"), CertificateSHA: digest(server), ServerCA: filepath.Join(parent, "server-ca.pem"), ServerCASHA: digest(caPEM), ServerPin: serverPin,
		ClientCertificate: filepath.Join(parent, "client.pem"), ClientCertificateSHA: digest(client), ClientCA: filepath.Join(parent, "client-ca.pem"), ClientCASHA: digest(caPEM), ClientPin: clientPin}
	command := os.Getenv("ARDENTS_SOURCE_CONTROL_PROGRAM")
	compiled, err := os.ReadFile(command)
	if err != nil || !filepath.IsAbs(command) || digest(compiled) != os.Getenv("ARDENTS_SOURCE_CONTROL_PROGRAM_SHA256") {
		t.Fatal("invalid environment: pinned canonical Control required", err)
	}
	args := []string{"source", "prepare-install-inputs"}
	for name, value := range map[string]string{"network": p.Network, "state-key": p.StateKey, "identity": p.Identity, "family": p.Family, "handle": p.Handle, "address": p.Address, "server-name": p.ServerName, "installation-root": p.Installation, "state-root": p.State, "participation-root": p.Participation, "credential-directory": p.Credentials, "clock": p.Clock, "server-private-key": p.PrivateKey, "output": p.Output,
		"server-certificate": p.Certificate, "server-certificate-sha256": p.CertificateSHA, "server-ca": p.ServerCA, "server-ca-sha256": p.ServerCASHA, "server-key-pin": p.ServerPin, "client-certificate": p.ClientCertificate, "client-certificate-sha256": p.ClientCertificateSHA, "client-ca": p.ClientCA, "client-ca-sha256": p.ClientCASHA, "client-key-pin": p.ClientPin} {
		args = append(args, "--"+name, value)
	}
	output, err := exec.CommandContext(t.Context(), command, args...).CombinedOutput()
	if err != nil {
		t.Fatal("actual public input command", string(output), err)
	}
	var receipt struct {
		Operation string            `json:"operation"`
		Files     map[string]string `json:"files"`
		Native    bool              `json:"native_admission"`
	}
	if json.Unmarshal(output, &receipt) != nil || receipt.Operation != "source.prepare-install-inputs" || receipt.Native || len(receipt.Files) != 6 {
		t.Fatal("bounded public receipt", string(output))
	}
	for name, pin := range receipt.Files {
		raw, err := os.ReadFile(filepath.Join(p.Output, name))
		if err != nil || digest(raw) != pin {
			t.Fatal("exact native output", name, err)
		}
	}
	for name, want := range map[string][]byte{"source-certificate.pem": server, "source-ca.pem": caPEM, "client-certificate.pem": client, "client-ca.pem": caPEM} {
		raw, err := os.ReadFile(filepath.Join(p.Output, name))
		if err != nil || !bytes.Equal(raw, want) {
			t.Fatal("public copy differs", name, err)
		}
	}
	request, err := os.ReadFile(filepath.Join(p.Output, "source-request.json"))
	if err != nil {
		t.Fatal(err)
	}
	var facts map[string]string
	if json.Unmarshal(request, &facts) != nil || len(facts) != 13 || facts["private_key"] != p.PrivateKey || facts["plan"] != filepath.Join(p.Output, "source-serve.json") || facts["state_root"] != p.State || facts["address"] != p.Address {
		t.Fatal("canonical request facts", string(request))
	}
	for _, path := range []string{p.PrivateKey, p.Installation, p.State, p.Participation, p.Credentials, p.Clock} {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("declaration must remain absent", path, err)
		}
	}
	if _, err := prepareSourceInstallInputs(t.Context(), p, nil); err == nil {
		t.Fatal("existing output adopted")
	}
	for _, fault := range []string{"commitment", "name", "runtime-namespace", "cancel-and-close"} {
		t.Run(fault, func(t *testing.T) {
			changed := p
			changed.Output = filepath.Join(parent, "refused-"+fault)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			var original *os.File
			var observe func(string, *os.File)
			switch fault {
			case "commitment":
				changed.CertificateSHA = strings.Repeat("00", 32)
			case "name":
				changed.ServerName = "wrong.test"
			case "runtime-namespace":
				changed.State = "/tmp/invisible-state"
			case "cancel-and-close":
				observe = func(stage string, file *os.File) {
					if stage == "before-directory" {
						original = file
						cancel()
						if err := file.Close(); err != nil {
							t.Fatal(err)
						}
					}
				}
			}
			result, err := prepareSourceInstallInputs(ctx, changed, observe)
			if err == nil || result != nil {
				t.Fatal("invalid public preparation accepted", err)
			}
			if fault == "cancel-and-close" && (original == nil || !errors.Is(err, context.Canceled) || !errors.Is(err, os.ErrClosed)) {
				t.Fatal("original cancellation/physical close lost", err)
			}
			if _, err := os.Lstat(changed.Output); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("pre-effect refusal created output", err)
			}
		})
	}
}
