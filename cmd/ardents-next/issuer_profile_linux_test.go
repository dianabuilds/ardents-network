//go:build linux

package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/nodeidentity"
)

func profileCommand(t *testing.T, operation string, plan map[string]any, collector string) (issuanceResult, error) {
	t.Helper()
	args := append(strings.Split(operation, "."), "--config", hostingConfig(t, plan))
	if collector != "" {
		args = append(args, collector)
	}
	cmd := exec.CommandContext(t.Context(), compiledCommand(t), args...)
	var out, log bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &log
	err := cmd.Run()
	var r issuanceResult
	if e := json.Unmarshal(out.Bytes(), &r); e != nil {
		t.Fatal(e, &out, &log)
	}
	for _, v := range plan {
		if s, ok := v.(string); ok && filepath.IsAbs(s) && bytes.Contains(log.Bytes(), []byte(s)) {
			t.Fatal("path leaked")
		}
	}
	return r, err
}
func TestIssuerProfileCLICompleteTokenCycle(t *testing.T) {
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{11}, 32))
	signer := [32]byte(private.Public().(ed25519.PublicKey))
	p, init, issue, verify := issuanceResultCommandPlanWithSigner(t, signer)
	base := filepath.Dir(p.Root)
	source := filepath.Join(base, "node.pem")
	der, e := x509.MarshalPKCS8PrivateKey(private)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(source, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0600); e != nil {
		t.Fatal(e)
	}
	keyBinding := init["key_binding"].(issuanceBindingInput)
	identityBinding := map[string]any{"network": keyBinding.Network, "node": keyBinding.Issuer, "signer": keyBinding.Signer}
	identityRoot := filepath.Join(base, "identity")
	importPlan := map[string]any{"root": identityRoot, "binding": identityBinding, "key_file": source}
	r, e := profileCommand(t, "node-identity.import", importPlan, "http://127.0.0.1:1")
	if e != nil || r.Outcome != "identity-imported" {
		t.Fatal(r, e)
	}
	profileFile := filepath.Join(base, "profile.bin")
	profilePlan := map[string]any{"identity_root": identityRoot, "identity_binding": identityBinding, "key_root": p.Root, "key_binding": keyBinding, "profile_root": filepath.Join(base, "profile"), "profile_file": profileFile}
	r, e = profileCommand(t, "issuance.initialize-profile", profilePlan, "")
	if e != nil || r.Outcome != "profile-initialized" {
		t.Fatal(r, e)
	}
	for _, root := range []string{identityRoot, p.Root, profilePlan["profile_root"].(string)} {
		bad := map[string]any{}
		for k, v := range profilePlan {
			bad[k] = v
		}
		destination := filepath.Join(root, "export.bin")
		bad["profile_file"] = destination
		refused, err := profileCommand(t, "issuance.inspect-profile", bad, "")
		if err == nil || refused.Outcome != "invalid-input" {
			t.Fatal("root export accepted", refused, err)
		}
		if _, err = os.Stat(destination); !os.IsNotExist(err) {
			t.Fatal("export changed root")
		}
	}
	original, e := os.ReadFile(profileFile)
	if e != nil {
		t.Fatal(e)
	}
	if !ed25519.Verify(signer[:], append([]byte("ardents-closed-issuer-keys-v1\x00"), original[:len(original)-64]...), original[len(original)-64:]) {
		t.Fatal("independent signature")
	}
	r, e = profileCommand(t, "issuance.inspect-profile", profilePlan, "")
	if e != nil || r.Outcome != "profile-verified" {
		t.Fatal(r, e)
	}
	if e = os.WriteFile(profileFile, []byte("conflict"), 0600); e != nil {
		t.Fatal(e)
	}
	r, e = profileCommand(t, "issuance.inspect-profile", profilePlan, "")
	if e == nil || r.Outcome != "storage-unavailable" {
		t.Fatal("export conflict", r, e)
	}
	profileFile = filepath.Join(base, "profile-again.bin")
	profilePlan["profile_file"] = profileFile
	r, e = profileCommand(t, "issuance.inspect-profile", profilePlan, "")
	if e != nil {
		t.Fatal(r, e)
	}
	again, e := os.ReadFile(profileFile)
	if e != nil || !bytes.Equal(original, again) {
		t.Fatal("profile changed")
	}
	raw, e := json.Marshal(init["admission_binding"])
	if e != nil {
		t.Fatal(e)
	}
	var fields map[string]any
	if e = json.Unmarshal(raw, &fields); e != nil {
		t.Fatal(e)
	}
	delete(fields, "keys")
	bindingFile := filepath.Join(base, "binding.json")
	bindingPlan := map[string]any{"profile_file": profileFile, "expected_profile": keyBinding, "binding": fields, "binding_file": bindingFile}
	var mu sync.Mutex
	payloads := map[string][]byte{}
	collector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		mu.Lock()
		payloads[r.URL.Path] = raw
		mu.Unlock()
		w.WriteHeader(200)
	}))
	defer collector.Close()
	r, e = profileCommand(t, "admission.prepare-binding", bindingPlan, collector.URL)
	if e != nil || r.Outcome != "binding-prepared" {
		t.Fatal(r, e)
	}
	badExport := map[string]any{}
	for k, v := range bindingPlan {
		badExport[k] = v
	}
	destination := filepath.Join(identityRoot, "binding.json")
	badExport["binding_file"] = destination
	refused, err := profileCommand(t, "admission.prepare-binding", badExport, "")
	if err == nil || refused.Outcome != "storage-unavailable" {
		t.Fatal("binding corrupted identity", refused, err)
	}
	if _, err = os.Stat(destination); !os.IsNotExist(err) {
		t.Fatal("binding export effect")
	}
	mu.Lock()
	captured := map[string][]byte{}
	for k, v := range payloads {
		captured[k] = v
	}
	mu.Unlock()
	assertIssuanceOTLP(t, captured, p, "admission.prepare-binding", "export", "binding-prepared")
	raw, e = os.ReadFile(bindingFile)
	if e != nil {
		t.Fatal(e)
	}
	var bound admissionBindingInput
	if e = json.Unmarshal(raw, &bound); e != nil {
		t.Fatal(e)
	}
	expected := init["admission_binding"].(admissionBindingInput)
	if bound.Profile != expected.Profile || bound.Authority != expected.Authority || bound.Duty != expected.Duty || len(bound.Keys) != len(expected.Keys) {
		t.Fatal("State facts replaced")
	}
	ledgerRoot := filepath.Join(base, "prepared-admission")
	initializePlan := map[string]any{"root": ledgerRoot, "binding": json.RawMessage(raw)}
	r, e = profileCommand(t, "admission.initialize", initializePlan, "")
	if e != nil || r.Outcome != "initialized-offline" {
		t.Fatal(r, e)
	}
	init["admission_root"] = ledgerRoot
	init["admission_binding"] = bound
	issue["admission_root"] = ledgerRoot
	issue["admission_binding"] = bound
	r, e = resultCommand(t, "initialize-results", init, "")
	if e != nil {
		t.Fatal(r, e)
	}
	r, e = resultCommand(t, "issue", issue, "")
	if e != nil || r.Outcome != "issued-offline" {
		t.Fatal(r, e)
	}
	response, e := os.ReadFile(issue["response_file"].(string))
	if e != nil {
		t.Fatal(e)
	}
	verify(response)
	r, e = resultCommand(t, "issue", issue, "")
	if e != nil || r.Outcome != "already-issued" {
		t.Fatal(r, e)
	}
	repeated, e := os.ReadFile(issue["response_file"].(string))
	if e != nil || !bytes.Equal(response, repeated) {
		t.Fatal("response changed")
	}
	identity, e := nodeidentity.Open(t.Context(), identityRoot, nodeidentity.Binding{Network: p.Binding.Network, Node: p.Binding.Issuer, Signer: signer})
	if e != nil {
		t.Fatal(e)
	}
	r, e = profileCommand(t, "issuance.inspect-profile", profilePlan, "")
	if e == nil || r.Outcome != "busy" {
		t.Fatal(r, e)
	}
	if e = identity.Close(); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	var out, log bytes.Buffer
	if code := run(ctx, []string{"issuance", "inspect-profile", "--config", hostingConfig(t, profilePlan)}, &out, &log); code != 130 {
		t.Fatal("cancel", code, &out)
	}
	if hex.EncodeToString(signer[:]) != keyBinding.Signer {
		t.Fatal("fixture pin")
	}
}
func TestProfileCommandStrictInput(t *testing.T) {
	for _, raw := range []string{`{"root":"/unused","root":"/again","binding":{},"key_file":"/missing"}`, `{"Root":"/unused","binding":{},"key_file":"/missing"}`, `{"root":null,"binding":{},"key_file":"/missing"}`, `{"root":"/unused","binding":{},"key_file":"/missing","extra":1}`} {
		path := filepath.Join(t.TempDir(), "config")
		if e := os.WriteFile(path, []byte(raw), 0600); e != nil {
			t.Fatal(e)
		}
		var out, log bytes.Buffer
		if code := run(t.Context(), []string{"node-identity", "import", "--config", path}, &out, &log); code != 2 {
			t.Fatal(code, &out)
		}
	}
}
