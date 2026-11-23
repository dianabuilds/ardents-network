//go:build linux

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestPlatformRetiresDetachedSourceStartBeforeEffects(t *testing.T) {
	var out bytes.Buffer
	if code := runInfrastructure(context.Background(), []string{"start-source", "/absent/distribution", "/absent/trust", "closed", "private", "/absent/binding"}, &out); code != 2 || out.Len() != 0 {
		t.Fatal("retired Source startup entered Installation", code, out.String())
	}
}

func TestPlatformSourceCompletionInspectionUsesExactRetainedPlan(t *testing.T) {
	path, plan := sourceCustodyFixture(t)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	config, err := platformSourceInspectionConfig(raw)
	if err != nil {
		t.Fatal(err)
	}
	if config.Root != plan.Network.Root || config.LocalRoleStateRoot != plan.ParticipationRoot || config.ClockObservationFile != plan.Network.ClockObservationFile || config.Source.ServeAddress != "" || config.Source.Sources[0].Address != "" || config.Source.Sources[1].Address != "" || config.AutomaticRefreshInterval != 0 {
		t.Fatal("inspection widened or changed runtime plan")
	}
	plan.ParticipationRoot = plan.Network.Root
	raw, err = json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := platformSourceInspectionConfig(raw); err == nil {
		t.Fatal("overlapping semantic histories accepted")
	}
}

func platformTestLaunch(role string) platformLaunch {
	base := "/var/lib/ardents/roles/" + role
	roots := []string{base + "/state", base + "/participation", base + "/spend", base + "/hosting"}
	if role == "introduction" {
		roots = append(roots, base+"/slots")
	}
	inputs := map[string]string{"/etc/ardents/node.crt": strings.Repeat("c", 64), "/etc/ardents/node.key": strings.Repeat("d", 64)}
	if role == "source" {
		inputs["/etc/ardents/client-ca.pem"] = strings.Repeat("e", 64)
	}
	return platformLaunch{Role: role, Distribution: "/var/lib/ardents/artifacts/release-one", ReleaseRoot: "/var/lib/ardents/release-trust", Environment: "closed", Network: "private", Program: "/usr/lib/ardents/release-one/ardents-next", Configuration: "/etc/ardents/" + role + ".json", ConfigSHA256: strings.Repeat("a", 64), WritableRoots: roots, Inputs: inputs}
}

func TestPlatformLaunchRejectsExpansionAndWritableTrust(t *testing.T) {
	for name, change := range map[string]func(*platformLaunch){
		"root-escape":   func(p *platformLaunch) { p.WritableRoots[0] = "/var/lib/ardents" },
		"role-escape":   func(p *platformLaunch) { p.Role = "other" },
		"specifier":     func(p *platformLaunch) { p.Program += "%i" },
		"environment":   func(p *platformLaunch) { p.Configuration += "$X" },
		"newline":       func(p *platformLaunch) { p.Distribution += "\nExecStart=/bin/sh" },
		"overlap":       func(p *platformLaunch) { p.WritableRoots[1] = p.WritableRoots[0] + "/child" },
		"trust":         func(p *platformLaunch) { p.ReleaseRoot = p.WritableRoots[0] },
		"artifact":      func(p *platformLaunch) { p.Distribution = p.WritableRoots[0] },
		"configuration": func(p *platformLaunch) { p.Configuration = p.WritableRoots[0] + "/plan" },
		"pin":           func(p *platformLaunch) { p.ConfigSHA256 = strings.Repeat("z", 64) },
		"mutable-key": func(p *platformLaunch) {
			delete(p.Inputs, "/etc/ardents/node.key")
			p.Inputs[p.WritableRoots[0]+"/node.key"] = strings.Repeat("d", 64)
		},
	} {
		t.Run(name, func(t *testing.T) {
			p := platformTestLaunch("introduction")
			if !validPlatformLaunch(p) {
				t.Fatal("healthy declaration refused")
			}
			change(&p)
			if validPlatformLaunch(p) {
				t.Fatal("unsafe platform declaration accepted")
			}
		})
	}
}

func TestPlatformConfigurationBindsEveryCredentialAndSourceTrustInput(t *testing.T) {
	raw := []byte(`{"certificate":"/etc/ardents/node.crt","private_key":"/etc/ardents/node.key","client_root":"/etc/ardents/client-ca.pem"}`)
	paths, err := platformPlanInputs("source", raw)
	if err != nil || len(paths) != 3 || paths[2] != "/etc/ardents/client-ca.pem" {
		t.Fatal("Source trust input omitted", paths, err)
	}
	for _, bad := range []string{`{"certificate":"/etc/ardents/node.crt","private_key":"/etc/ardents/node.key"}`, `{"certificate":"/etc/ardents/node.crt","private_key":"/etc/ardents/node.key","client_root":"/var/lib/ardents/roles/source/state/ca.pem"}`} {
		if _, err := platformPlanInputs("source", []byte(bad)); err == nil {
			t.Fatal("unbound/mutable Source trust admitted")
		}
	}
}

func TestPlatformUnitRunsExistingRetainedRuntimeDirectly(t *testing.T) {
	for _, role := range []string{"source", "forwarding", "issuer", "descriptor", "introduction", "join"} {
		p := platformTestLaunch(role)
		unit := string(renderPlatformUnit(p, "/etc/ardents/launch.json", strings.Repeat("b", 64)))
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
	var out bytes.Buffer
	if runPlatform(t.Context(), []string{"template"}, &out) != 0 || out.String() != platformUnitTemplate {
		t.Fatal("signed-template byte producer differs")
	}
}

func TestPlatformPlanWriteProjectionCannotWidenRoots(t *testing.T) {
	p := platformTestLaunch("introduction")
	fields := map[string]any{"network": networkAuthorityPlan{Root: p.WritableRoots[0], Authorities: [][32]byte{{1}}}, "participation_root": p.WritableRoots[1], "spend_root": p.WritableRoots[2], "hosting_root": p.WritableRoots[3], "introduction_root": p.WritableRoots[4]}
	raw, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	roots, err := platformPlanRoots(p.Role, raw)
	if err != nil || !equalPlatformRoots(roots, p.WritableRoots) {
		t.Fatal("original domain roots differ", roots, err)
	}
	if equalPlatformRoots(roots, append(roots, "/etc")) {
		t.Fatal("extra platform write permission accepted")
	}
	duplicate := append([]byte(`{"spend_root":"/foreign",`), raw[1:]...)
	if _, err := platformPlanRoots(p.Role, duplicate); err == nil {
		t.Fatal("duplicate configuration field accepted")
	}
}
