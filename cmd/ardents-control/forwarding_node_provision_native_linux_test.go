//go:build linux && network_signing_native

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network/epoch"
)

func TestForwardingNodeProvisionNativeCanonicalKeyRecordAndCertificate(t *testing.T) {
	parent := sourceArtifactNativeParent(t)
	command, _ := sourceNativePinnedProgram(t, "ARDENTS_FORWARDING_CONTROL_PROGRAM")
	call := func(args ...string) []byte {
		t.Helper()
		output, err := exec.CommandContext(t.Context(), command, append([]string{"forwarding"}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatal("canonical Node preparation refused", args[0], err, string(output))
		}
		return output
	}
	keyDir := filepath.Join(parent, "node-key")
	keyReceipt := call("prepare-node-key", keyDir)
	var key struct {
		Operation string `json:"operation"`
		Public    string `json:"node_public_key"`
		Runtime   bool   `json:"runtime_authority"`
	}
	if json.Unmarshal(keyReceipt, &key) != nil || key.Operation != "forwarding.prepare-node-key" || len(key.Public) != 64 || key.Runtime || bytes.Contains(keyReceipt, []byte("PRIVATE KEY")) {
		t.Fatal("private or invalid key receipt")
	}
	privatePath := filepath.Join(keyDir, "node-key.pem")
	privateInfo, err := os.Lstat(privatePath)
	if err != nil || privateInfo.Mode() != 0600 {
		t.Fatal("protected key birth missing", err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	planDir := filepath.Join(parent, "record-preparation")
	call("prepare-node-record", "--network-id", strings.Repeat("01", 32), "--node-id", strings.Repeat("02", 32), "--node-key", key.Public, "--family", "controlled-component-family", "--endpoint", "127.0.0.1:9401", "--valid-from", now.Add(-time.Minute).Format(time.RFC3339), "--valid-until", now.Add(time.Hour).Format(time.RFC3339), "--capacity", "1", "--output", planDir)
	planPath, preparedPath := filepath.Join(planDir, "node-record-plan.json"), filepath.Join(planDir, "node-record.signing-input")
	prepared, err := os.ReadFile(preparedPath)
	if err != nil {
		t.Fatal(err)
	}
	pin := func(raw []byte) string { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }
	signedDir := filepath.Join(parent, "signed")
	call("sign-node-record", planPath, preparedPath, pin(prepared), privatePath, signedDir)
	recordPath := filepath.Join(signedDir, "node-record.bin")
	record, err := os.ReadFile(recordPath)
	if err != nil {
		t.Fatal(err)
	}
	facts, err := epoch.VerifyInitialClosedRecord(prepared, record)
	if err != nil || hex.EncodeToString(facts.PublicKey) != key.Public || facts.NodeID != [32]byte{2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2} {
		t.Fatal("actual signed Record differs", err)
	}
	certDir := filepath.Join(parent, "certificate")
	call("prepare-node-certificate", planPath, recordPath, pin(record), privatePath, certDir)
	certificate, err := os.ReadFile(filepath.Join(certDir, "node-certificate.pem"))
	if err != nil {
		t.Fatal(err)
	}
	block, rest := pem.Decode(certificate)
	if block == nil || len(rest) != 0 || verifyForwardingNodeCertificate(block.Bytes, facts) != nil {
		t.Fatal("actual canonical certificate profile differs")
	}
	for _, op := range []string{"sign-node-record", "prepare-node-certificate"} {
		input := preparedPath
		if op == "prepare-node-certificate" {
			input = recordPath
		}
		output := filepath.Join(parent, "wrong-pin-"+op)
		_, err := prepareForwardingNodeSigned(t.Context(), []string{op, planPath, input, strings.Repeat("00", 32), filepath.Join(parent, "unopened-private-key"), output}, nil)
		if err == nil || errors.Is(err, os.ErrNotExist) {
			t.Fatal("wrong pin reached private key read", op, err)
		}
		if _, err := os.Lstat(output); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("wrong pin created output", op, err)
		}
	}
	for _, op := range []string{"sign-node-record", "prepare-node-certificate"} {
		bad := bytes.Clone(prepared)
		if op == "prepare-node-certificate" {
			bad = bytes.Clone(record)
		}
		bad[len(bad)-1] ^= 1
		input, output := filepath.Join(parent, "changed-"+op), filepath.Join(parent, "changed-output-"+op)
		if err := os.WriteFile(input, bad, 0600); err != nil {
			t.Fatal(err)
		}
		// The supplied SHA is correct for these actual changed bytes. Public
		// preparation/signature verification, not merely the pin, must refuse
		// before attempting the deliberately missing private input.
		_, err := prepareForwardingNodeSigned(t.Context(), []string{op, planPath, input, pin(bad), filepath.Join(parent, "unopened-private-key"), output}, nil)
		if err == nil || errors.Is(err, os.ErrNotExist) {
			t.Fatal("changed public artifact reached private key read", op, err)
		}
		if _, err := os.Lstat(output); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("changed artifact created output", op, err)
		}
	}
	otherDir := filepath.Join(parent, "other-key")
	call("prepare-node-key", otherDir)
	for _, op := range []string{"sign-node-record", "prepare-node-certificate"} {
		input, raw := preparedPath, prepared
		if op == "prepare-node-certificate" {
			input, raw = recordPath, record
		}
		output := filepath.Join(parent, "wrong-key-"+op)
		if out, err := exec.CommandContext(t.Context(), command, "forwarding", op, planPath, input, pin(raw), filepath.Join(otherDir, "node-key.pem"), output).CombinedOutput(); err == nil || bytes.Contains(out, []byte(`"runtime_authority"`)) {
			t.Fatal("wrong original key accepted", op, err)
		}
		if _, err := os.Lstat(output); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("wrong key created output", op, err)
		}
	}
	if _, err := exec.CommandContext(t.Context(), command, "forwarding", "prepare-node-key", keyDir).CombinedOutput(); err == nil {
		t.Fatal("existing private key output adopted")
	}
	if after, err := os.Lstat(privatePath); err != nil || !os.SameFile(privateInfo, after) || after.Mode() != privateInfo.Mode() || after.ModTime() != privateInfo.ModTime() {
		t.Fatal("original Node key replaced", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := prepareForwardingNodeKey(ctx, filepath.Join(parent, "canceled-key"), nil); err == nil {
		t.Fatal("canceled key birth accepted")
	}
	if _, err := os.Lstat(filepath.Join(parent, "canceled-key")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("cancellation created key output", err)
	}
}
