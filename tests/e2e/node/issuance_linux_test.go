//go:build linux

package node_test

import "github.com/dianabuilds/ardents-network/internal/admission/quota"

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"github.com/dianabuilds/ardents-network/internal/admission"
	"strconv"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/admission/issuance"
	"github.com/dianabuilds/ardents-network/internal/admission/issuerprofile"
	"github.com/dianabuilds/ardents-network/tests/e2e/node/processfixture"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func issuanceCommandPlan(t *testing.T) (issuancePlan, map[string]any) {
	t.Helper()
	dir := t.TempDir()
	if e := os.Chmod(dir, 0700); e != nil {
		t.Fatal(e)
	}
	start := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	b := issuance.Binding{Network: [32]byte{1}, Issuer: [32]byte{2}, Signer: [32]byte{3}, Start: start, End: start.Add(time.Hour)}
	p := issuancePlan{Root: filepath.Join(dir, "keys"), Binding: b, InventoryFile: filepath.Join(dir, "public.json")}
	input := map[string]any{"root": p.Root, "binding": issuanceBindingInput{hex.EncodeToString(b.Network[:]), hex.EncodeToString(b.Issuer[:]), hex.EncodeToString(b.Signer[:]), b.Start.Format(time.RFC3339), b.End.Format(time.RFC3339)}, "inventory_file": p.InventoryFile}
	return p, input
}
func issuanceCommand(t *testing.T, operation string, p map[string]any, collector string) (issuanceResult, error) {
	t.Helper()
	args := []string{"issuance", operation, "--config", processfixture.WriteConfig(t, p)}
	if collector != "" {
		args = append(args, collector)
	}
	command := exec.CommandContext(t.Context(), buildNodeCommand(t), args...)
	var out, log bytes.Buffer
	command.Stdout = &out
	command.Stderr = &log
	err := command.Run()
	var r issuanceResult
	if e := json.Unmarshal(out.Bytes(), &r); e != nil {
		t.Fatal(e, &out, &log)
	}
	var fields map[string]any
	_ = json.Unmarshal(out.Bytes(), &fields)
	if len(fields) != 3 {
		t.Fatal("extra output", fields)
	}
	if bytes.Contains(log.Bytes(), []byte(p["root"].(string))) {
		t.Fatal("path leaked")
	}
	return r, err
}
func TestIssuanceCompiledCLI(t *testing.T) {
	p, input := issuanceCommandPlan(t)
	if r, e := issuanceCommand(t, "initialize", input, ""); e != nil || r.Outcome != "completed" {
		t.Fatal(r, e)
	}
	before, e := os.ReadFile(p.InventoryFile)
	if e != nil {
		t.Fatal(e)
	}
	if r, e := issuanceCommand(t, "inspect", input, "http://127.0.0.1:1"); e != nil || r.Outcome != "completed" {
		t.Fatal(r, e)
	}
	after, _ := os.ReadFile(p.InventoryFile)
	if !bytes.Equal(before, after) {
		t.Fatal("inventory changed")
	}
	s, e := issuance.Open(t.Context(), p.Root, p.Binding)
	if e != nil {
		t.Fatal(e)
	}
	if r, e := issuanceCommand(t, "inspect", input, ""); e == nil || r.Outcome != "busy" {
		t.Fatal(r, e)
	}
	inventory, e := s.Inventory()
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Close(); e != nil {
		t.Fatal(e)
	}
	// Real command Admission initializes with exactly the exported SPKI bytes.
	var exported struct {
		Keys []struct {
			Window string `json:"window"`
			Class  uint8  `json:"class"`
			SPKI   string `json:"spki"`
		} `json:"keys"`
	}
	if e = json.Unmarshal(before, &exported); e != nil {
		t.Fatal(e)
	}
	binding := quota.LedgerBinding{Network: p.Binding.Network, Issuer: p.Binding.Issuer, Authority: [32]byte{4}, Profile: [32]byte{5}, Duty: 1, Start: p.Binding.Start, End: p.Binding.End}
	for i, k := range exported.Keys {
		spki, err := hex.DecodeString(k.SPKI)
		if err != nil || !bytes.Equal(spki, inventory.Keys[i].SPKI) {
			t.Fatal("export SPKI mismatch")
		}
		binding.Keys = append(binding.Keys, issuerprofile.Key{Window: inventory.Keys[i].Window, Class: k.Class, SPKI: spki})
	}
	config := processfixture.WriteConfig(t, map[string]any{"root": filepath.Join(filepath.Dir(p.Root), "admission"), "binding": admissionBindingConfig(binding)})
	command := exec.CommandContext(t.Context(), buildNodeCommand(t), "admission", "initialize", "--config", config)
	if result, e := command.CombinedOutput(); e != nil {
		t.Fatal(e, string(result))
	}
	if e = os.WriteFile(p.InventoryFile, []byte("different"), 0600); e != nil {
		t.Fatal(e)
	}
	if r, e := issuanceCommand(t, "inspect", input, ""); e == nil || r.Outcome != "storage-unavailable" {
		t.Fatal(r, e)
	}
	s, e = issuance.Open(t.Context(), p.Root, p.Binding)
	if e != nil {
		t.Fatal("export reset root", e)
	}
	same, _ := s.Inventory()
	_ = s.Close()
	if same.Digest != inventory.Digest {
		t.Fatal("changed identity")
	}
}
func TestIssuanceCLICrashReopen(t *testing.T) {
	for _, episode := range []string{"kill", "interrupt"} {
		t.Run(episode, func(t *testing.T) {
			p, input := issuanceCommandPlan(t)
			entered := make(chan struct{}, 2)
			release := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				entered <- struct{}{}
				select {
				case <-release:
				case <-r.Context().Done():
				}
			}))
			defer server.Close()
			defer close(release)
			command := exec.CommandContext(t.Context(), buildNodeCommand(t), "issuance", "initialize", "--config", processfixture.WriteConfig(t, input), server.URL)
			var out bytes.Buffer
			command.Stdout = &out
			if e := command.Start(); e != nil {
				t.Fatal(e)
			}
			defer func() { _ = command.Process.Kill(); _ = command.Wait() }()
			select {
			case <-entered:
			case <-time.After(10 * time.Second):
				t.Fatal("no committed export")
			}
			before, e := os.ReadFile(p.InventoryFile)
			if e != nil {
				t.Fatal(e)
			}
			if episode == "kill" {
				_ = command.Process.Kill()
			} else {
				_ = command.Process.Signal(os.Interrupt)
			}
			_ = command.Wait()
			if episode == "interrupt" {
				var r issuanceResult
				if json.Unmarshal(out.Bytes(), &r) != nil || r.Outcome != "completed" {
					t.Fatal("cancel after commit", out.String())
				}
			}
			if r, e := issuanceCommand(t, "inspect", input, ""); e != nil || r.Outcome != "completed" {
				t.Fatal(r, e)
			}
			after, _ := os.ReadFile(p.InventoryFile)
			if !bytes.Equal(before, after) {
				t.Fatal("crash changed inventory")
			}
		})
	}
}

func TestIssuanceOTLPFiniteAttributes(t *testing.T) {
	var mu sync.Mutex
	seen := map[string][]byte{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(io.LimitReader(r.Body, 32769))
		mu.Lock()
		seen[r.URL.Path] = raw
		mu.Unlock()
		w.Header().Set("Content-Type", "application/x-protobuf")
	}))
	defer server.Close()
	p, input := issuanceCommandPlan(t)
	if r, e := issuanceCommand(t, "initialize", input, server.URL); e != nil || r.Outcome != "completed" {
		t.Fatal(r, e)
	}
	mu.Lock()
	defer mu.Unlock()
	assertIssuanceOTLP(t, seen, p)
}

type issuanceResult struct {
	Operation string `json:"operation"`
	Phase     string `json:"phase"`
	Outcome   string `json:"outcome"`
}

type issuancePlan struct {
	Root          string
	Binding       issuance.Binding
	InventoryFile string
}

type issuanceBindingInput struct {
	Network string `json:"network"`
	Issuer  string `json:"issuer"`
	Signer  string `json:"signer"`
	Start   string `json:"start"`
	End     string `json:"end"`
}

type admissionBindingInput struct {
	Network   string              `json:"network"`
	Issuer    string              `json:"issuer"`
	Authority string              `json:"authority"`
	Profile   string              `json:"profile"`
	Duty      string              `json:"duty"`
	Start     string              `json:"start"`
	End       string              `json:"end"`
	Keys      []admissionKeyInput `json:"keys"`
}
type admissionKeyInput struct {
	Window string `json:"window"`
	Class  string `json:"class"`
	SPKI   string `json:"spki"`
}

func admissionBindingConfig(b quota.LedgerBinding) admissionBindingInput {
	input := admissionBindingInput{Network: hex.EncodeToString(b.Network[:]), Issuer: hex.EncodeToString(b.Issuer[:]), Authority: hex.EncodeToString(b.Authority[:]), Profile: hex.EncodeToString(b.Profile[:]), Duty: strconv.FormatUint(b.Duty, 10), Start: b.Start.Format(time.RFC3339), End: b.End.Format(time.RFC3339)}
	for _, key := range b.Keys {
		input.Keys = append(input.Keys, admissionKeyInput{time.Unix(int64(key.Window), 0).UTC().Format(time.RFC3339), strconv.Itoa(int(key.Class)), hex.EncodeToString(key.SPKI)})
	}
	return input
}
func admissionFactsConfig(f admission.Facts) map[string]string {
	return map[string]string{"network": hex.EncodeToString(f.Network[:]), "issuer": hex.EncodeToString(f.Issuer[:]), "authority": hex.EncodeToString(f.Authority[:]), "holder": hex.EncodeToString(f.Holder[:]), "duty": strconv.FormatUint(f.Duty, 10), "duty_not_before": f.DutyNotBefore.Format(time.RFC3339), "duty_not_after": f.DutyNotAfter.Format(time.RFC3339), "now": f.Now.Format(time.RFC3339), "class": strconv.Itoa(int(f.Class)), "count": strconv.Itoa(int(f.Count))}
}

// TestIssuanceInventoryInputRefusal exercises the actual command and ensures
// malformed declarations are refused before an inventory root is created.
func TestIssuanceInventoryInputRefusal(t *testing.T) {
	binary := buildNodeCommand(t)
	dir := t.TempDir()
	root := filepath.Join(dir, "keys")
	input := map[string]any{"root": root, "inventory_file": filepath.Join(dir, "public"), "binding": issuanceBindingInput{strings.Repeat("ab", 32), strings.Repeat("cd", 32), strings.Repeat("ef", 32), "2030-01-01T00:00:00Z", "2030-01-01T01:00:00Z"}}
	raw, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		"duplicate":  strings.Replace(string(raw), `"network":`, `"network":"`+strings.Repeat("ab", 32)+`","network":`, 1),
		"case":       strings.Replace(string(raw), `"network":`, `"Network":`, 1),
		"missing":    strings.Replace(string(raw), `"signer":"`+strings.Repeat("ef", 32)+`",`, "", 1),
		"unknown":    strings.Replace(string(raw), `"binding":{`, `"binding":{"extra":0,`, 1),
		"uppercase":  strings.Replace(string(raw), strings.Repeat("ab", 32), strings.Repeat("AB", 32), 1),
		"null":       strings.Replace(string(raw), `"binding":{`, `"extra":null,"binding":{`, 1),
		"fractional": strings.Replace(string(raw), "00:00:00Z", "00:00:00.001Z", 1),
		"timezone":   strings.Replace(string(raw), "00:00:00Z", "00:00:00+00:00", 1),
		"trailing":   string(raw) + " {}",
	}
	cases["valid"] = string(raw)
	for name, value := range cases {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.json")
			if err := os.WriteFile(path, []byte(value), 0600); err != nil {
				t.Fatal(err)
			}
			cmd := exec.CommandContext(t.Context(), binary, "issuance", "inspect", "--config", path)
			output, err := cmd.Output()
			if err == nil {
				t.Fatal("missing root accepted")
			}
			var report issuanceResult
			if err := json.Unmarshal(output, &report); err != nil {
				t.Fatalf("invalid report %q: %v", output, err)
			}
			want := "invalid-input"
			if name == "valid" {
				want = "storage-unavailable"
			}
			if report.Outcome != want {
				t.Fatalf("got outcome %q, want %q", report.Outcome, want)
			}
			if _, err := os.Stat(root); !os.IsNotExist(err) {
				t.Fatalf("inventory root changed: %v", err)
			}
		})
	}
}
