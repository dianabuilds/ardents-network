package installation

import (
	"bytes"
	"strings"
	"testing"
)

// Independently spelled fixed contract, not produced from the renderer's
// property map. These tests check bytes, never a successful native manager.
const endpointTemplateFixture = `[Unit]
Description=Ardents protected text Endpoint
Requires=ardents-text-reader.socket ardents-text-publisher.socket
After=network-online.target ardents-text-reader.socket ardents-text-publisher.socket
Wants=network-online.target

[Service]
Type=exec
User=ardents-endpoint
Group=ardents-endpoint
SupplementaryGroups=
ExecStart=:@ARDENTS_ENDPOINT_PROGRAM@ endpoint start-installed @ARDENTS_INSTALLATION_ROOT@
WorkingDirectory=/
UMask=0077
NoNewPrivileges=yes
CapabilityBoundingSet=
AmbientCapabilities=
PrivateTmp=yes
ProtectHome=yes
ProtectSystem=strict
ReadWritePaths=@ARDENTS_WRITE_PATHS@
ProtectControlGroups=yes
ProtectKernelTunables=yes
ProtectKernelModules=yes
ProtectKernelLogs=yes
RestrictSUIDSGID=yes
LockPersonality=yes
RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6
MemoryAccounting=yes
CPUAccounting=yes
TasksAccounting=yes
LimitCORE=0
Restart=no
RemainAfterExit=no
KillMode=control-group
TimeoutStopSec=30s
StandardOutput=journal
StandardError=journal
`

func TestGenerationEndpointUnitFixedBytes(t *testing.T) {
	request, err := decodeInstallationRequest(requestFixture())
	if err != nil {
		t.Fatal(err)
	}
	directory := "/installation/generations/" + strings.Repeat("01", 32)
	want := strings.NewReplacer(
		"@ARDENTS_ENDPOINT_PROGRAM@", `"`+directory+`/ardents-linux-amd64"`,
		"@ARDENTS_INSTALLATION_ROOT@", `"/installation"`,
		"@ARDENTS_WRITE_PATHS@", `"/entry" "/permissions" "/roles" "/socket" "/state" "/tokens"`,
	).Replace(endpointTemplateFixture)
	got, err := renderEndpointUnit([]byte(endpointTemplateFixture), request, directory)
	if err != nil || !bytes.Equal(got, []byte(want)) {
		t.Fatalf("fixed bytes differ: %v\n%s", err, got)
	}
	// Selected artifact paths retain Linux grammar on every checking host.
	request.InstallationRoot = "/installation space/%n"
	directory = request.InstallationRoot + "/generations/" + strings.Repeat("01", 32)
	got, err = renderEndpointUnit([]byte(endpointTemplateFixture), request, directory)
	if err != nil || !bytes.Contains(got, []byte(`ExecStart=:"/installation space/%%n/generations/`+strings.Repeat("01", 32)+`/ardents-linux-amd64" endpoint start-installed "/installation space/%%n"`)) {
		t.Fatalf("literal paths lost: %v\n%s", err, got)
	}
}

func TestGenerationEndpointUnitRefusesChangedProtection(t *testing.T) {
	request, err := decodeInstallationRequest(requestFixture())
	if err != nil {
		t.Fatal(err)
	}
	directory := "/installation/generations/" + strings.Repeat("01", 32)
	for name, raw := range map[string]string{
		"weaker-protection":  strings.Replace(endpointTemplateFixture, "ProtectSystem=strict", "ProtectSystem=full", 1),
		"missing-protection": strings.Replace(endpointTemplateFixture, "NoNewPrivileges=yes\n", "", 1),
		"duplicate-property": endpointTemplateFixture + "KillMode=process\n",
		"unknown-property":   endpointTemplateFixture + "ExecStartPost=/bin/true\n",
		"ambiguous-marker":   endpointTemplateFixture + "# @ARDENTS_ENDPOINT_PROGRAM@\n",
		"unknown-section":    endpointTemplateFixture + "[Install]\nWantedBy=multi-user.target\n",
		"oversize":           strings.Repeat("#", 64<<10) + endpointTemplateFixture,
	} {
		t.Run(name, func(t *testing.T) {
			if body, err := renderEndpointUnit([]byte(raw), request, directory); err == nil || body != nil {
				t.Fatal("changed fixed contract accepted")
			}
		})
	}
	for _, directory := range []string{"/other/generations/" + strings.Repeat("01", 32), "/installation/generations/current", "/installation/generations/" + strings.Repeat("AB", 32)} {
		if _, err := renderEndpointUnit([]byte(endpointTemplateFixture), request, directory); err == nil {
			t.Fatal("unbound generation path accepted")
		}
	}
}

func TestGenerationWriteAllowancesRefuseImmutableAncestors(t *testing.T) {
	for _, immutable := range []string{"bundle_root", "installation_root", "release_floor_root"} {
		t.Run(immutable, func(t *testing.T) {
			raw := string(requestFixture())
			original := map[string]string{"bundle_root": "/bundle", "installation_root": "/installation", "release_floor_root": "/floors"}[immutable]
			raw = strings.Replace(raw, `"`+immutable+`":"`+original+`"`, `"`+immutable+`":"/permissions/private"`, 1)
			request, err := decodeInstallationRequest([]byte(raw))
			if err != nil {
				t.Fatalf("independent declaration precondition: %v", err)
			}
			if _, err := writableDirectories(request); err == nil {
				t.Fatal("permission parent exposes protected root")
			}
		})
	}
	request, err := decodeInstallationRequest(requestFixture())
	if err != nil {
		t.Fatal(err)
	}
	request.Headless.ApplicationSocket = "/socket-at-root"
	if _, err := writableDirectories(request); err == nil {
		t.Fatal("whole-filesystem write allowance accepted")
	}
	request.Headless.ApplicationSocket = "/control\t/socket"
	if _, err := writableDirectories(request); err == nil {
		t.Fatal("unit path control character accepted")
	}
}
