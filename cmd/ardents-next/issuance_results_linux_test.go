//go:build linux

package main

import "github.com/dianabuilds/ardents-network/internal/successor/admission/quota"

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/cloudflare/circl/blindsign/blindrsa"

	"github.com/dianabuilds/ardents-network/internal/successor/admission/issuance"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/issuerprofile"
)

func issuanceResultCommandPlan(t *testing.T) (issuancePlan, map[string]any, map[string]any, func([]byte)) {
	return issuanceResultCommandPlanWithSigner(t, [32]byte{3})
}

func issuanceResultCommandPlanWithSigner(t *testing.T, signer [32]byte) (issuancePlan, map[string]any, map[string]any, func([]byte)) {
	t.Helper()
	p, _ := issuanceCommandPlan(t)
	p.Binding.Signer = signer
	p.Binding.Start = time.Unix(3600, 0).UTC()
	p.Binding.End = time.Unix(7200, 0).UTC()
	if e := issuance.Initialize(t.Context(), p.Root, p.Binding); e != nil {
		t.Fatal(e)
	}
	s, e := issuance.Open(t.Context(), p.Root, p.Binding)
	if e != nil {
		t.Fatal(e)
	}
	v, e := s.Inventory()
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Close(); e != nil {
		t.Fatal(e)
	}
	raw, f, b := admissionCommandFixture(t, 2, 2, 2, 1)
	b.Keys = nil
	for _, key := range v.Keys {
		b.Keys = append(b.Keys, issuerprofile.Key{Window: key.Window, Class: key.Class, SPKI: key.SPKI})
	}
	copy(raw[277:623], b.Keys[1].SPKI)
	var subject struct {
		Algorithm asn1.RawValue
		Public    asn1.BitString
	}
	if rest, e := asn1.Unmarshal(b.Keys[1].SPKI, &subject); e != nil || len(rest) != 0 {
		t.Fatal(e)
	}
	public, e := x509.ParsePKCS1PublicKey(subject.Public.Bytes)
	if e != nil {
		t.Fatal(e)
	}
	client, e := blindrsa.NewClient(blindrsa.SHA384PSSDeterministic, public)
	if e != nil {
		t.Fatal(e)
	}
	var checks []func([]byte)
	for i := 0; i < 2; i++ {
		prepared, e := client.Prepare(rand.Reader, []byte{byte(i)})
		if e != nil {
			t.Fatal(e)
		}
		blinded, state, e := client.Blind(rand.Reader, prepared)
		if e != nil {
			t.Fatal(e)
		}
		id := sha256.Sum256(b.Keys[1].SPKI)
		raw[627+i*259] = id[31]
		copy(raw[628+i*259:884+i*259], blinded)
		checks = append(checks, func(sig []byte) {
			final, e := client.Finalize(state, sig)
			if e != nil {
				t.Fatal(e)
			}
			if e = client.Verify(prepared, final); e != nil {
				t.Fatal(e)
			}
		})
	}
	seed := make([]byte, 32)
	seed[0] = 1
	holder := ed25519.NewKeyFromSeed(seed)
	transcript := append([]byte("ardents-issuance-request-v1\x00"), raw[80:112]...)
	transcript = append(transcript, raw[236:277]...)
	transcript = append(transcript, raw[623:625]...)
	digest := sha256.Sum256(raw[625 : len(raw)-64])
	transcript = append(transcript, digest[:]...)
	copy(raw[len(raw)-64:], ed25519.Sign(holder, transcript))
	parent := filepath.Dir(p.Root)
	ledgerRoot := filepath.Join(parent, "admission")
	if e = quota.Initialize(ledgerRoot, b); e != nil {
		t.Fatal(e)
	}
	keyBinding := issuanceBindingInput{Network: admissionBindingConfig(b).Network, Issuer: admissionBindingConfig(b).Issuer, Signer: hex.EncodeToString(p.Binding.Signer[:]), Start: p.Binding.Start.Format(time.RFC3339), End: p.Binding.End.Format(time.RFC3339)}
	init := map[string]any{"admission_root": ledgerRoot, "admission_binding": admissionBindingConfig(b), "key_root": p.Root, "key_binding": keyBinding, "result_root": filepath.Join(parent, "results")}
	issue := map[string]any{}
	for k, v := range init {
		issue[k] = v
	}
	batchFile := filepath.Join(parent, "batch")
	if e = os.WriteFile(batchFile, raw, 0600); e != nil {
		t.Fatal(e)
	}
	issue["batch_file"] = batchFile
	issue["facts"] = admissionFactsConfig(f)
	issue["kind"] = "bootstrap"
	issue["response_file"] = filepath.Join(parent, "response")
	verify := func(response []byte) {
		if len(response) != 16347 || string(response[:8]) != "ARDIOR01" || response[8] != 1 || response[9] != 2 {
			t.Fatal("grammar")
		}
		for i, check := range checks {
			check(response[10+i*256 : 10+(i+1)*256])
		}
	}
	return p, init, issue, verify
}

func resultCommand(t *testing.T, operation string, plan map[string]any, collector string) (issuanceResult, error) {
	t.Helper()
	args := []string{"issuance", operation, "--config", hostingConfig(t, plan)}
	if collector != "" {
		args = append(args, collector)
	}
	command := exec.CommandContext(t.Context(), compiledCommand(t), args...)
	var out, log bytes.Buffer
	command.Stdout = &out
	command.Stderr = &log
	err := command.Run()
	var r issuanceResult
	if e := json.Unmarshal(out.Bytes(), &r); e != nil {
		t.Fatal(e, &out, &log)
	}
	if bytes.Contains(log.Bytes(), []byte(plan["key_root"].(string))) {
		t.Fatal("metadata leak")
	}
	return r, err
}
func TestConfirmedIssuanceCompiledCLI(t *testing.T) {
	p, init, issue, verify := issuanceResultCommandPlan(t)
	if r, e := resultCommand(t, "initialize-results", init, ""); e != nil || r.Outcome != "initialized-results" {
		t.Fatal(r, e)
	}
	s, e := issuance.Open(t.Context(), p.Root, p.Binding)
	if e != nil {
		t.Fatal(e)
	}
	if r, e := resultCommand(t, "issue", issue, ""); e == nil || r.Outcome != "busy" {
		t.Fatal(r, e)
	}
	_ = s.Close()
	if r, e := resultCommand(t, "issue", issue, "http://127.0.0.1:1"); e != nil || r.Outcome != "issued-offline" {
		t.Fatal(r, e)
	}
	saved, e := os.ReadFile(issue["response_file"].(string))
	if e != nil {
		t.Fatal(e)
	}
	verify(saved)
	if r, e := resultCommand(t, "issue", issue, ""); e != nil || r.Outcome != "already-issued" {
		t.Fatal(r, e)
	}
	again, _ := os.ReadFile(issue["response_file"].(string))
	if !bytes.Equal(saved, again) {
		t.Fatal("replay changed")
	}
	if e = os.WriteFile(issue["response_file"].(string), []byte("conflicting"), 0600); e != nil {
		t.Fatal(e)
	}
	if r, e := resultCommand(t, "issue", issue, ""); e == nil || r.Outcome != "storage-unavailable" {
		t.Fatal(r, e)
	}
	issue["response_file"] = filepath.Join(filepath.Dir(p.Root), "new-response")
	if r, e := resultCommand(t, "issue", issue, ""); e != nil || r.Outcome != "already-issued" {
		t.Fatal(r, e)
	}
	again, _ = os.ReadFile(issue["response_file"].(string))
	if !bytes.Equal(saved, again) {
		t.Fatal("export loss")
	}
	issue["kind"] = "admitted"
	if r, e := resultCommand(t, "issue", issue, ""); e == nil || r.Outcome != "request-conflict" {
		t.Fatal(r, e)
	}
}
func TestConfirmedIssuanceCLIOTLPAndCrash(t *testing.T) {
	for _, episode := range []string{"otlp", "kill", "interrupt"} {
		t.Run(episode, func(t *testing.T) {
			p, init, issue, verify := issuanceResultCommandPlan(t)
			if r, e := resultCommand(t, "initialize-results", init, ""); e != nil {
				t.Fatal(r, e)
			}
			var mu sync.Mutex
			seen := map[string][]byte{}
			entered := make(chan struct{}, 2)
			release := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				raw, _ := io.ReadAll(io.LimitReader(r.Body, 32769))
				mu.Lock()
				seen[r.URL.Path] = raw
				mu.Unlock()
				w.Header().Set("Content-Type", "application/x-protobuf")
				if episode != "otlp" {
					entered <- struct{}{}
					select {
					case <-release:
					case <-r.Context().Done():
					}
				}
			}))
			defer server.Close()
			defer close(release)
			command := exec.CommandContext(t.Context(), compiledCommand(t), "issuance", "issue", "--config", hostingConfig(t, issue), server.URL)
			var out bytes.Buffer
			command.Stdout = &out
			if e := command.Start(); e != nil {
				t.Fatal(e)
			}
			defer func() { _ = command.Process.Kill(); _ = command.Wait() }()
			if episode != "otlp" {
				select {
				case <-entered:
				case <-time.After(10 * time.Second):
					t.Fatal("missing committed export")
				}
				if episode == "kill" {
					_ = command.Process.Kill()
				} else {
					_ = command.Process.Signal(os.Interrupt)
				}
			}
			err := command.Wait()
			if episode != "kill" && err != nil {
				t.Fatal(err, string(out.Bytes()))
			}
			saved, e := os.ReadFile(issue["response_file"].(string))
			if e != nil {
				t.Fatal(e)
			}
			verify(saved)
			if r, e := resultCommand(t, "issue", issue, ""); e != nil || r.Outcome != "already-issued" {
				t.Fatal(r, e)
			}
			again, _ := os.ReadFile(issue["response_file"].(string))
			if !bytes.Equal(saved, again) {
				t.Fatal("crash changed")
			}
			if episode == "otlp" {
				mu.Lock()
				assertIssuanceOTLP(t, seen, p, "issuance.issue", "export", "issued-offline")
				mu.Unlock()
			}
		})
	}
}
