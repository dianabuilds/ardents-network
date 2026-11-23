package launch

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/network/state"
)

func platformTestLaunch(role string) Declaration {
	base := "/var/lib/ardents/roles/" + role
	roots := []string{base + "/state", base + "/participation", base + "/spend", base + "/hosting"}
	if role == "introduction" {
		roots = append(roots, base+"/slots")
	}
	inputs := map[string]string{"/etc/ardents/node.crt": strings.Repeat("c", 64), "/etc/ardents/node.key": strings.Repeat("d", 64)}
	if role == "source" {
		inputs["/etc/ardents/client-ca.pem"] = strings.Repeat("e", 64)
	}
	return Declaration{Role: role, Distribution: "/var/lib/ardents/artifacts/release-one", ReleaseRoot: "/var/lib/ardents/release-trust", Environment: "closed", Network: "private", Program: "/usr/lib/ardents/release-one/ardents-next", Configuration: "/etc/ardents/" + role + ".json", ConfigSHA256: strings.Repeat("a", 64), WritableRoots: roots, Inputs: inputs}
}

func TestPlatformLaunchRejectsExpansionAndWritableTrust(t *testing.T) {
	for name, change := range map[string]func(*Declaration){
		"root-escape":   func(p *Declaration) { p.WritableRoots[0] = "/var/lib/ardents" },
		"role-escape":   func(p *Declaration) { p.Role = "other" },
		"specifier":     func(p *Declaration) { p.Program += "%i" },
		"environment":   func(p *Declaration) { p.Configuration += "$X" },
		"newline":       func(p *Declaration) { p.Distribution += "\nExecStart=/bin/sh" },
		"overlap":       func(p *Declaration) { p.WritableRoots[1] = p.WritableRoots[0] + "/child" },
		"trust":         func(p *Declaration) { p.ReleaseRoot = p.WritableRoots[0] },
		"artifact":      func(p *Declaration) { p.Distribution = p.WritableRoots[0] },
		"configuration": func(p *Declaration) { p.Configuration = p.WritableRoots[0] + "/plan" },
		"pin":           func(p *Declaration) { p.ConfigSHA256 = strings.Repeat("z", 64) },
		"mutable-key": func(p *Declaration) {
			delete(p.Inputs, "/etc/ardents/node.key")
			p.Inputs[p.WritableRoots[0]+"/node.key"] = strings.Repeat("d", 64)
		},
	} {
		t.Run(name, func(t *testing.T) {
			p := platformTestLaunch("introduction")
			if !valid(p) {
				t.Fatal("healthy declaration refused")
			}
			change(&p)
			if valid(p) {
				t.Fatal("unsafe platform declaration accepted")
			}
		})
	}
}

func TestPlatformConfigurationBindsEveryCredentialAndSourceTrustInput(t *testing.T) {
	raw := []byte(`{"certificate":"/etc/ardents/node.crt","private_key":"/etc/ardents/node.key","client_root":"/etc/ardents/client-ca.pem"}`)
	paths, err := credentialPaths("source", raw)
	if err != nil || len(paths) != 3 || paths[2] != "/etc/ardents/client-ca.pem" {
		t.Fatal("Source trust input omitted", paths, err)
	}
	for _, bad := range []string{`{"certificate":"/etc/ardents/node.crt","private_key":"/etc/ardents/node.key"}`, `{"certificate":"/etc/ardents/node.crt","private_key":"/etc/ardents/node.key","client_root":"/var/lib/ardents/roles/source/state/ca.pem"}`} {
		if _, err := credentialPaths("source", []byte(bad)); err == nil {
			t.Fatal("unbound/mutable Source trust admitted")
		}
	}
}

func TestPlatformUnitRunsExistingRetainedRuntimeDirectly(t *testing.T) {
	for _, role := range []string{"source", "forwarding", "issuer", "descriptor", "introduction", "join"} {
		p := platformTestLaunch(role)
		unit := string(p.unit("/etc/ardents/launch.json", strings.Repeat("b", 64)).Render())
		for _, required := range []string{"ExecStartPre=+:/var/lib/ardents/artifacts/release-one/ardents-node-linux-amd64 platform check /etc/ardents/launch.json ", "Restart=no\n", "KillMode=control-group\n", "TimeoutStopSec=infinity\n", "SendSIGKILL=no\n", "NoNewPrivileges=yes\n", "CapabilityBoundingSet=\n"} {
			if !strings.Contains(unit, required) {
				t.Fatalf("%s lacks %q", role, required)
			}
		}
		command := " node " + role + " --retained-config " + p.Configuration
		if role == "source" {
			command = " source serve-retained " + p.Configuration
		}
		if !strings.Contains(unit, "ExecStart=:"+p.Program+command+"\n") || strings.Contains(unit, "start-installed") || strings.Contains(unit, "@") {
			t.Fatal("unit delegates to Installation or leaves substitution", role, unit)
		}
	}
}

func TestPlatformPlanWriteProjectionCannotWidenRoots(t *testing.T) {
	p := platformTestLaunch("introduction")
	fields := map[string]any{"network": state.AuthorityDeclaration{Root: p.WritableRoots[0], Authorities: [][32]byte{{1}}}, "participation_root": p.WritableRoots[1], "spend_root": p.WritableRoots[2], "hosting_root": p.WritableRoots[3], "introduction_root": p.WritableRoots[4]}
	raw, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	roots, err := writableRoots(p.Role, raw)
	if err != nil || !equalRoots(roots, p.WritableRoots) {
		t.Fatal("original domain roots differ", roots, err)
	}
	if equalRoots(roots, append(roots, "/etc")) {
		t.Fatal("extra platform write permission accepted")
	}
	duplicate := append([]byte(`{"spend_root":"/foreign",`), raw[1:]...)
	if _, err := writableRoots(p.Role, duplicate); err == nil {
		t.Fatal("duplicate configuration field accepted")
	}
}

func TestIssuerProjectionRetainsKeyLeaseOnly(t *testing.T) {
	root := "/var/lib/ardents/roles/issuer/"
	pin := strings.Repeat("a", 64)
	keyBinding := map[string]any{
		"network": pin, "issuer": pin, "signer": pin,
		"start": "2026-11-23T12:00:00Z", "end": "2026-11-23T13:00:00Z",
	}
	ledgerBinding := map[string]any{
		"network": pin, "issuer": pin, "authority": pin, "profile": pin, "duty": "1",
		"start": keyBinding["start"], "end": keyBinding["end"],
	}
	var inventory []map[string]any
	for _, class := range []string{"1", "2", "3"} {
		inventory = append(inventory, map[string]any{"window": keyBinding["start"], "class": class, "spki": strings.Repeat("a", 692)})
	}
	ledgerBinding["keys"] = inventory
	issuance := map[string]any{
		"admission_root": root + "quota", "admission_binding": ledgerBinding,
		"key_root": root + "keys", "key_binding": keyBinding, "result_root": root + "results",
	}
	plan := map[string]any{
		"network": state.AuthorityDeclaration{Root: root + "state", Authorities: [][32]byte{{1}}}, "participation_root": root + "duty",
		"spend_root": root + "spend", "hosting_root": root + "hosting", "issuance": issuance,
	}
	check := func() ([]string, error) {
		t.Helper()
		raw, err := json.Marshal(plan)
		if err != nil {
			t.Fatal(err)
		}
		return writableRoots("issuer", raw)
	}
	want := []string{root + "state", root + "duty", root + "spend", root + "hosting", root + "quota", root + "results", root + "keys/issuer.lock"}
	if got, err := check(); err != nil || !equalRoots(got, want) {
		t.Fatalf("issuer write grants = %v, %v", got, err)
	}
	issuance["result_root"] = root + "keys/nested"
	if _, err := check(); err == nil {
		t.Fatal("result storage within immutable key root was admitted")
	}
	issuance["result_root"] = root + "results"
	ledgerBinding["keys"] = nil
	if _, err := check(); err == nil {
		t.Fatal("issuer configuration without the complete binding was admitted")
	}
}
