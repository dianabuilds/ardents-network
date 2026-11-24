//go:build linux && network_signing_native

package node_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	sourcefixture "github.com/dianabuilds/ardents-network/tests/e2e/source"
)

func nativeSigningArguments(t *testing.T, binary string) []string {
	t.Helper()
	parent := sourcefixture.SigningWorkspace(t)
	key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{71}, 32))
	p := preparationRecordPlan(key.Public().(ed25519.PublicKey))
	now := time.Now().UTC().Truncate(time.Second)
	p["valid_from"], p["valid_until"] = now.Add(-time.Minute).Format(time.RFC3339), now.Add(time.Hour).Format(time.RFC3339)
	message := prepareNetworkSigningRecord(t, binary, p)
	var err error
	digest := sha256.Sum256(message)
	plan, prepared, keyPath := filepath.Join(parent, "plan"), filepath.Join(parent, "prepared"), filepath.Join(parent, "key")
	if err = os.WriteFile(plan, signingDeclaration(t, "node-record", p, now), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(prepared, message, 0600); err != nil {
		t.Fatal(err)
	}
	writeSigningTestKey(t, keyPath, key)
	return []string{"sign-initial-node-record", plan, prepared, preparationHex(digest[:]), keyPath, filepath.Join(parent, "signed")}
}

func changeSigningTestTime(t *testing.T, binary string, args []string, from, until, reference time.Time, reprepare bool) {
	t.Helper()
	raw, err := os.ReadFile(args[1])
	if err != nil {
		t.Fatal(err)
	}
	var plan map[string]json.RawMessage
	var record map[string]any
	if json.Unmarshal(raw, &plan) != nil || json.Unmarshal(plan["record"], &record) != nil {
		t.Fatal("test declaration")
	}
	record["valid_from"], record["valid_until"] = from.Format(time.RFC3339Nano), until.Format(time.RFC3339Nano)
	plan["record"] = preparationJSON(t, record)
	plan["reference_time"] = preparationJSON(t, reference.Format(time.RFC3339Nano))
	if reprepare {
		message := prepareNetworkSigningRecord(t, binary, record)
		var e error

		digest := sha256.Sum256(message)
		args[3] = preparationHex(digest[:])
		if e = os.WriteFile(args[2], message, 0600); e != nil {
			t.Fatal(e)
		}
	}
	if err = os.WriteFile(args[1], preparationJSON(t, plan), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestNetworkSigningStaticNativeRefusals(t *testing.T) {
	binary := buildNodeCommand(t)
	for name, arguments := range map[string]func(*testing.T, string) []string{"record": nativeSigningArguments, "certificate": nativeCertificateArguments} {
		t.Run(name, func(t *testing.T) { testSigningStaticNativeRefusals(t, binary, arguments) })
	}
}

func testSigningStaticNativeRefusals(t *testing.T, binary string, arguments func(*testing.T, string) []string) {
	for _, fault := range []string{"symlink", "hardlink", "owner", "mode", "special mode", "type", "encoding", "wrong key", "oversize", "prepared pin", "QUIC", "stale", "future", "expired", "not yet live", "oversized interval", "fractional interval"} {
		t.Run(fault, func(t *testing.T) {
			args := arguments(t, binary)
			var e error
			switch fault {
			case "symlink":
				e = os.Rename(args[4], args[4]+"-original")
				if e == nil {
					e = os.Symlink(args[4]+"-original", args[4])
				}
			case "hardlink":
				e = os.Link(args[4], args[4]+"-other")
			case "owner":
				e = os.Chown(args[4], 65534, 65534)
			case "mode":
				e = os.Chmod(args[4], 0644)
			case "special mode":
				e = os.Chmod(args[4], 0600|os.ModeSetuid)
			case "type":
				e = os.Remove(args[4])
				if e == nil {
					e = os.Mkdir(args[4], 0700)
				}
			case "encoding":
				e = os.WriteFile(args[4], []byte("PRIVATE KEY must not be echoed"), 0600)
			case "wrong key":
				writeSigningTestKey(t, args[4], ed25519.NewKeyFromSeed(bytes.Repeat([]byte{98}, 32)))
			case "oversize":
				e = os.WriteFile(args[4], bytes.Repeat([]byte{1}, (16<<10)+1), 0600)
			case "prepared pin":
				args[3] = preparationIdentifier(99)
			case "QUIC":
				raw, err := os.ReadFile(args[1])
				if err != nil {
					t.Fatal(err)
				}
				raw = bytes.ReplaceAll(raw, []byte("ardents-carrier-tcp-tls-v2"), []byte("ardents-carrier-quic-v2"))
				e = os.WriteFile(args[1], raw, 0600)
			case "stale", "future", "expired", "not yet live", "oversized interval", "fractional interval":
				now := time.Now().UTC().Truncate(time.Second)
				from, until, ref := now.Add(-time.Minute), now.Add(time.Hour), now
				switch fault {
				case "stale":
					from, ref = now.Add(-10*time.Minute), now.Add(-6*time.Minute)
				case "future":
					ref = now.Add(2 * time.Minute)
				case "expired":
					from, until, ref = now.Add(-2*time.Minute), now.Add(-time.Second), now.Add(-30*time.Second)
				case "not yet live":
					from, ref = now.Add(40*time.Second), now.Add(50*time.Second)
				case "oversized interval":
					until = from.Add(7*24*time.Hour + time.Second)
				case "fractional interval":
					from = from.Add(100 * time.Millisecond)
				}
				changeSigningTestTime(t, binary, args, from, until, ref, fault != "fractional interval")
			}
			if e != nil {
				t.Fatal(e)
			}
			cmd := exec.CommandContext(t.Context(), binary, append([]string{"network"}, args...)...)
			var out bytes.Buffer
			cmd.Stdout = &out
			if err := cmd.Run(); err == nil || out.Len() != 0 {
				t.Fatal("static refusal accepted or leaked diagnostic", err, out.String())
			}
			if _, e = os.Lstat(args[5]); !os.IsNotExist(e) {
				t.Fatal("invalid input created output", e)
			}
			control := arguments(t, binary)
			if output, err := exec.CommandContext(t.Context(), binary, append([]string{"network"}, control...)...).CombinedOutput(); err != nil {
				t.Fatal("healthy control", err, string(output))
			}
		})
	}
}

func nativeCertificateArguments(t *testing.T, binary string) []string {
	t.Helper()
	return sourcefixture.CertificateArguments(t, binary, sourcefixture.SigningWorkspace(t))
}

func TestNetworkSigningNodeCertificateReceipt(t *testing.T) {
	binary := buildNodeCommand(t)
	args := nativeCertificateArguments(t, binary)
	full, err := os.OpenFile("/dev/full", os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer full.Close()
	cmd := exec.CommandContext(t.Context(), binary, append([]string{"network"}, args...)...)
	cmd.Stdout = full
	if err := cmd.Run(); err == nil {
		t.Fatal("failed receipt accepted")
	}
	if err := exec.CommandContext(t.Context(), binary, append([]string{"network"}, args...)...).Run(); err == nil {
		t.Fatal("failed receipt output adopted")
	}
}

func prepareNetworkSigningRecord(t *testing.T, binary string, record any) []byte {
	t.Helper()
	parent := sourcefixture.SigningWorkspace(t)
	input, output := filepath.Join(parent, "plan"), filepath.Join(parent, "prepared")
	if err := os.WriteFile(input, preparationJSON(t, record), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(t.Context(), binary, "network", "prepare-initial-node-record", input, output)
	if raw, err := cmd.CombinedOutput(); err != nil {
		t.Fatal("prepare signing input", err, string(raw))
	}
	message, err := os.ReadFile(filepath.Join(output, "node-record.signing-input"))
	if err != nil {
		t.Fatal(err)
	}
	return message
}
