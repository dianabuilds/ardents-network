package infrastructure

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// This literal fixes the byte grammar independently of the production encoder.
const sourceRequestOracle = `{"schema":"ardents-source-preflight-v1","invocation":"source","profile":"ardents-source-process-v1","installation_root":"/var/lib/install","state_root":"/var/lib/state","participation_root":"/var/lib/participation","credential_directory":"/var/lib/credentials","plan":"/opt/inputs/plan","certificate":"/opt/inputs/cert","private_key":"/opt/inputs/key","client_ca":"/opt/inputs/ca","clock":"/var/lib/clock","address":"127.0.0.1:4567"}` + "\n"

// Independent complete release-template byte oracle, including order and LF.
// Production policy edits cannot silently rewrite this accepted vector.
const sourceTemplateOracle = "[Unit]\n" +
	"Description=Ardents Direct-Origin Source\nAfter=network-online.target\nWants=network-online.target\n" +
	"[Service]\nType=exec\nUser=ardents-source\nGroup=ardents-source\nSupplementaryGroups=\n" +
	"ExecStart=:@ARDENTS_SOURCE_PROGRAM@ source start-installed @ARDENTS_SOURCE_BINDING@\n" +
	"WorkingDirectory=/\nUMask=0077\nRestart=no\nRemainAfterExit=no\nKillMode=control-group\nTimeoutStopSec=30s\n" +
	"NoNewPrivileges=yes\nCapabilityBoundingSet=\nAmbientCapabilities=\nPrivateTmp=yes\nProtectHome=yes\nProtectSystem=strict\n" +
	"ReadWritePaths=@ARDENTS_SOURCE_WRITE_PATHS@\nProtectControlGroups=yes\nProtectKernelTunables=yes\nProtectKernelModules=yes\nProtectKernelLogs=yes\n" +
	"RestrictSUIDSGID=yes\nLockPersonality=yes\nRestrictAddressFamilies=AF_UNIX AF_INET AF_INET6\n" +
	"MemoryAccounting=yes\nCPUAccounting=yes\nTasksAccounting=yes\nCPUQuota=100%\nCPUQuotaPeriodSec=100ms\n" +
	"MemoryHigh=805306368\nMemoryMax=1073741824\nTasksMax=256\nLimitNOFILE=1024\nLimitCORE=0\n" +
	"Environment=GOMAXPROCS=1 GOMEMLIMIT=536870912\nStandardOutput=journal\nStandardError=journal\n"

func TestSourceRequestClosedGrammarAndFutureBoundaries(t *testing.T) {
	r, err := DecodeSourceRequest([]byte(sourceRequestOracle))
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{
		strings.Replace(sourceRequestOracle, `"source"`, `"forwarding"`, 1),
		strings.Replace(sourceRequestOracle, `"source"`, `"source","extra":true`, 1),
		strings.Replace(sourceRequestOracle, `"source"`, `"source","invocation":"source"`, 1),
		strings.Replace(sourceRequestOracle, "127.0.0.1:4567", "127.0.0.1:443", 1),
		strings.Replace(sourceRequestOracle, "127.0.0.1:4567", "example.test:4567", 1),
		strings.Replace(sourceRequestOracle, "/var/lib/state", "/var/lib/install/state", 1),
		strings.Replace(sourceRequestOracle, "/var/lib/state", "/var/lib/../state", 1),
		strings.Replace(sourceRequestOracle, "/var/lib/state", "/var/lib/%S", 1),
		strings.TrimSuffix(sourceRequestOracle, "\n"),
	} {
		if _, err := DecodeSourceRequest([]byte(raw)); err == nil {
			t.Fatal("ambiguous/wrong role/path/address admitted", raw)
		}
	}
	if r.StateRoot != "/var/lib/state" || r.PrivateKey != "/opt/inputs/key" {
		t.Fatal("input/destination projection changed")
	}
}

func TestSourceTemplateExactPolicyAndRendering(t *testing.T) {
	if SourceTemplate != sourceTemplateOracle {
		t.Fatal("Source release template differs from independent complete byte vector")
	}
	r, err := DecodeSourceRequest([]byte(sourceRequestOracle))
	if err != nil {
		t.Fatal(err)
	}
	generation := strings.Repeat("a", 64)
	raw, err := RenderSourceUnit([]byte(SourceTemplate), r, generation)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{
		`ExecStart=:"/var/lib/install/generations/` + generation + `/ardents-node-linux-amd64" source start-installed "/var/lib/install/binding.json"`,
		`ReadWritePaths="/var/lib/state" "/var/lib/participation"`,
		"User=ardents-source", "Restart=no", "CPUQuota=100%", "CPUQuotaPeriodSec=100ms", "MemoryHigh=805306368", "MemoryMax=1073741824", "LimitNOFILE=1024", "TasksMax=256",
		"Environment=GOMAXPROCS=1 GOMEMLIMIT=536870912", "CapabilityBoundingSet=", "ProtectSystem=strict",
	} {
		if !bytes.Contains(raw, []byte(line+"\n")) {
			t.Fatal("independent closed policy oracle missing", line)
		}
	}
	if bytes.Contains(raw, []byte("@ARDENTS_")) || bytes.Contains(raw, []byte("/opt/inputs")) {
		t.Fatal("rendered unit exposes input source or unresolved marker")
	}
	// Every deleted, changed, doubled or additional policy line refuses. This
	// includes hooks, supplementary groups, resource changes and Endpoint units.
	for _, line := range strings.Split(strings.TrimSuffix(SourceTemplate, "\n"), "\n") {
		for _, bad := range []string{strings.Replace(SourceTemplate, line+"\n", "", 1), strings.Replace(SourceTemplate, line+"\n", line+"x\n", 1), SourceTemplate + line + "\n"} {
			if _, err := RenderSourceUnit([]byte(bad), r, generation); err == nil {
				t.Fatal("altered template accepted", line)
			}
		}
	}
	for _, bad := range []string{"", "../generation", strings.Repeat("A", 64)} {
		if _, err := RenderSourceUnit([]byte(SourceTemplate), r, bad); err == nil {
			t.Fatal("unbound generation accepted")
		}
	}
	copy := r
	copy.Profile = "endpoint"
	encoded, _ := json.Marshal(copy)
	if _, err := DecodeSourceRequest(append(encoded, '\n')); err == nil {
		t.Fatal("foreign protection profile accepted")
	}
}
