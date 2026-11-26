package provisioning

import (
	"bytes"
	"context"
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/admission/issuance"
	"github.com/dianabuilds/ardents-network/internal/domain/node"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/processresource"
)

func writePreparePlanNetwork(t *testing.T, base string) {
	t.Helper()
	network := node.Network{Root: filepath.Join(base, "state"), NetworkID: [32]byte{1}, Authorities: [][32]byte{{2}}, Threshold: 1, ProfileAuthority: [32]byte{3}, ClockObservationFile: filepath.Join(base, "clock")}
	raw, err := json.Marshal(network)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "network.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
}

func writePreparePlanIssuance(t *testing.T, base string) {
	t.Helper()
	hex32, spki := strings.Repeat("cd", 32), strings.Repeat("ab", 346)
	start, end := "2026-01-01T00:00:00Z", "2026-01-02T00:00:00Z"
	var keys []map[string]any
	for range 3 {
		keys = append(keys, map[string]any{"window": start, "class": "1", "spki": spki})
	}
	sub := map[string]any{
		"admission_root":    filepath.Join(base, "admission"),
		"admission_binding": map[string]any{"network": hex32, "issuer": hex32, "authority": hex32, "profile": hex32, "duty": "1", "start": start, "end": end, "keys": keys},
		"key_root":          filepath.Join(base, "keys"),
		"key_binding":       map[string]any{"network": hex32, "issuer": hex32, "signer": hex32, "start": start, "end": end},
		"result_root":       filepath.Join(base, "results"),
	}
	raw, err := json.Marshal(sub)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "issuance.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
}

func preparePlanTestArgs(role, base string, extra map[string]string) []string {
	flags := map[string]string{
		"network": filepath.Join(base, "network.json"), "node-id": strings.Repeat("ab", 32),
		"participation-root": filepath.Join(base, "participation"), "spend-root": filepath.Join(base, "spend"), "hosting-root": filepath.Join(base, "hosting"),
		"certificate": filepath.Join(base, "cert.pem"), "private-key": filepath.Join(base, "key.pem"),
		"work": "33554432:33554432", "termination": "65536:65536", "retirement-wait-ms": "5000", "output": filepath.Join(base, "plan.json"),
	}
	maps.Copy(flags, extra)
	args := []string{role}
	for key, value := range flags {
		args = append(args, "--"+key, value)
	}
	return args
}

// decodedPreparePlanFacts decodes the produced plan through the runtime codec and
// returns the facts the runtime admission gate consumes, proving the written plan
// is admissible by the same closed grammar and per-role policy the runtime uses.
func decodedPreparePlanFacts(t *testing.T, role string, raw []byte) (*node.Network, string, []string, uint32) {
	t.Helper()
	switch role {
	case "forwarding":
		p, err := node.DecodeForwardingPlan(raw)
		if err != nil {
			t.Fatal("produced Forwarding plan does not decode:", err)
		}
		return p.Network, p.ProcessProfile, []string{p.ParticipationRoot, p.SpendRoot, p.HostingRoot}, p.RetirementWaitMS
	case "join":
		p, err := node.DecodeJoinPlan(raw)
		if err != nil {
			t.Fatal("produced JOIN plan does not decode:", err)
		}
		return p.Network, p.ProcessProfile, []string{p.ParticipationRoot, p.SpendRoot, p.HostingRoot}, p.RetirementWaitMS
	case "descriptor":
		p, err := node.DecodeDescriptorPlan(raw)
		if err != nil {
			t.Fatal("produced Descriptor plan does not decode:", err)
		}
		return p.Network, p.ProcessProfile, []string{p.ParticipationRoot, p.SpendRoot, p.HostingRoot, p.DescriptorRoot}, p.RetirementWaitMS
	case "introduction":
		p, err := node.DecodeIntroductionPlan(raw)
		if err != nil {
			t.Fatal("produced Introduction plan does not decode:", err)
		}
		return p.Network, p.ProcessProfile, []string{p.ParticipationRoot, p.SpendRoot, p.HostingRoot, p.IntroductionRoot}, p.RetirementWaitMS
	case "issuer":
		p, err := node.DecodeIssuerPlan(raw)
		if err != nil {
			t.Fatal("produced Issuer plan does not decode:", err)
		}
		prepared, err := issuance.DecodeResultSetupDeclaration(p.Issuance)
		if err != nil {
			t.Fatal("produced Issuer issuance sub-plan does not decode:", err)
		}
		return p.Network, p.ProcessProfile, []string{p.ParticipationRoot, p.SpendRoot, p.HostingRoot, prepared.AdmissionRoot, prepared.KeyRoot, prepared.ResultRoot}, p.RetirementWaitMS
	}
	t.Fatal("unknown role")
	return nil, "", nil, 0
}

func TestNodePreparePlanProducesRuntimeAdmissiblePlan(t *testing.T) {
	roles := []struct{ name, profile string }{
		{"forwarding", processresource.Profile},
		{"join", processresource.JoinProfile},
		{"descriptor", processresource.DescriptorProfile},
		{"introduction", processresource.IntroductionProfile},
		{"issuer", processresource.IssuerProfile},
	}
	for _, role := range roles {
		t.Run(role.name, func(t *testing.T) {
			base := t.TempDir()
			writePreparePlanNetwork(t, base)
			extra := map[string]string{}
			switch role.name {
			case "descriptor":
				extra["descriptor-root"] = filepath.Join(base, "store")
			case "introduction":
				extra["introduction-root"] = filepath.Join(base, "registry")
			case "issuer":
				writePreparePlanIssuance(t, base)
				extra["issuance"] = filepath.Join(base, "issuance.json")
				extra["bootstrap-work"] = "131072:131072"
			}
			args := preparePlanTestArgs(role.name, base, extra)
			var out bytes.Buffer
			if code := RunPreparePlan(t.Context(), args, &out); code != 0 {
				t.Fatalf("prepare-plan refused valid inputs: %d %s", code, &out)
			}
			if !bytes.Contains(out.Bytes(), []byte(`"outcome":"prepared"`)) {
				t.Fatal("prepare-plan did not report prepared:", out.String())
			}
			raw, err := os.ReadFile(filepath.Join(base, "plan.json"))
			if err != nil {
				t.Fatal("prepare-plan wrote no plan:", err)
			}
			network, profile, roots, wait := decodedPreparePlanFacts(t, role.name, raw)
			if profile != role.profile || wait != 5000 || network == nil {
				t.Fatalf("produced plan facts diverge: profile=%q wait=%d", profile, wait)
			}
			if !(*state.AuthorityDeclaration)(network).ValidShape() || !network.IndependentRoots(roots...) {
				t.Fatal("produced plan is not accepted by the runtime admission gate")
			}
		})
	}
}

func TestNodePreparePlanRefusesBeforeEffect(t *testing.T) {
	cases := []struct {
		name   string
		role   string
		mutate func(base string, flags map[string]string) map[string]string
	}{
		{"unknown-role", "source", func(_ string, f map[string]string) map[string]string { return f }},
		{"missing-output", "forwarding", func(_ string, f map[string]string) map[string]string { delete(f, "output"); return f }},
		{"unknown-flag", "forwarding", func(_ string, f map[string]string) map[string]string { f["bogus"] = "1"; return f }},
		{"foreign-role-flag", "forwarding", func(b string, f map[string]string) map[string]string {
			f["descriptor-root"] = filepath.Join(b, "store")
			return f
		}},
		{"short-node-id", "forwarding", func(_ string, f map[string]string) map[string]string { f["node-id"] = "abcd"; return f }},
		{"relative-root", "forwarding", func(_ string, f map[string]string) map[string]string { f["spend-root"] = "relative/spend"; return f }},
		{"zero-retirement", "forwarding", func(_ string, f map[string]string) map[string]string { f["retirement-wait-ms"] = "0"; return f }},
		{"overlong-retirement", "forwarding", func(_ string, f map[string]string) map[string]string { f["retirement-wait-ms"] = "5001"; return f }},
		{"overlapping-roots", "forwarding", func(b string, f map[string]string) map[string]string {
			f["spend-root"] = filepath.Join(b, "participation")
			return f
		}},
		{"bad-traffic", "forwarding", func(_ string, f map[string]string) map[string]string { f["work"] = "123"; return f }},
		{"issuer-missing-issuance", "issuer", func(_ string, f map[string]string) map[string]string { delete(f, "issuance"); return f }},
		{"descriptor-missing-root", "descriptor", func(_ string, f map[string]string) map[string]string { delete(f, "descriptor-root"); return f }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			base := t.TempDir()
			writePreparePlanNetwork(t, base)
			flags := map[string]string{
				"network": filepath.Join(base, "network.json"), "node-id": strings.Repeat("ab", 32),
				"participation-root": filepath.Join(base, "participation"), "spend-root": filepath.Join(base, "spend"), "hosting-root": filepath.Join(base, "hosting"),
				"certificate": filepath.Join(base, "cert.pem"), "private-key": filepath.Join(base, "key.pem"),
				"work": "33554432:33554432", "termination": "65536:65536", "retirement-wait-ms": "5000", "output": filepath.Join(base, "plan.json"),
			}
			if tc.role == "descriptor" {
				flags["descriptor-root"] = filepath.Join(base, "store")
			}
			if tc.role == "issuer" {
				writePreparePlanIssuance(t, base)
				flags["issuance"] = filepath.Join(base, "issuance.json")
				flags["bootstrap-work"] = "131072:131072"
			}
			flags = tc.mutate(base, flags)
			args := []string{tc.role}
			for key, value := range flags {
				args = append(args, "--"+key, value)
			}
			var out bytes.Buffer
			if code := RunPreparePlan(t.Context(), args, &out); code != 2 {
				t.Fatalf("invalid prepare-plan input accepted: %d %s", code, &out)
			}
			if _, err := os.Stat(filepath.Join(base, "plan.json")); !os.IsNotExist(err) {
				t.Fatal("refused prepare-plan created an output file")
			}
		})
	}
}

func TestNodePreparePlanRefusesDuplicateFlagTokenAndClobber(t *testing.T) {
	base := t.TempDir()
	writePreparePlanNetwork(t, base)
	args := preparePlanTestArgs("forwarding", base, nil)
	// A repeated --work token (distinct from a repeated map key) must refuse.
	args = append(args, "--work", "1:1")
	var out bytes.Buffer
	if code := RunPreparePlan(t.Context(), args, &out); code != 2 {
		t.Fatal("duplicate flag token accepted:", code)
	}
	// An existing output file is never clobbered.
	fresh := preparePlanTestArgs("forwarding", base, nil)
	if err := os.WriteFile(filepath.Join(base, "plan.json"), []byte("existing"), 0600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if code := RunPreparePlan(t.Context(), fresh, &out); code != 2 {
		t.Fatal("prepare-plan clobbered an existing output:", code)
	}
	if raw, _ := os.ReadFile(filepath.Join(base, "plan.json")); string(raw) != "existing" {
		t.Fatal("existing output was modified")
	}
}

func TestNodePreparePlanCanceledRefusesBeforeEffect(t *testing.T) {
	base := t.TempDir()
	writePreparePlanNetwork(t, base)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	args := preparePlanTestArgs("forwarding", base, nil)
	var out bytes.Buffer
	if code := RunPreparePlan(ctx, args, &out); code != 130 {
		t.Fatal("canceled prepare-plan did not refuse before effect:", code)
	}
	if _, err := os.Stat(filepath.Join(base, "plan.json")); !os.IsNotExist(err) {
		t.Fatal("canceled prepare-plan created an output file")
	}
}
