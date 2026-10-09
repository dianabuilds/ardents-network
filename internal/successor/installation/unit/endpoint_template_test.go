package unit

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

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

func TestEndpointTemplateFixedBytesAndOriginalInput(t *testing.T) {
	directory := "/installation/generations/" + strings.Repeat("01", 32)
	raw := []byte(endpointTemplateFixture)
	prepared, err := PrepareEndpointTemplate(raw, "/installation", directory)
	if err != nil {
		t.Fatal(err)
	}
	clear(raw)
	want := strings.NewReplacer("@ARDENTS_ENDPOINT_PROGRAM@", `"`+directory+`/ardents-linux-amd64"`, "@ARDENTS_INSTALLATION_ROOT@", `"/installation"`, "@ARDENTS_WRITE_PATHS@", `"/entry" "/permissions" "/roles" "/socket" "/state" "/tokens"`).Replace(endpointTemplateFixture)
	got, err := prepared.Render([]string{"/entry", "/permissions", "/roles", "/socket", "/state", "/tokens"})
	if err != nil || !bytes.Equal(got, []byte(want)) {
		t.Fatalf("fixed original template bytes differ: %v", err)
	}
	if body, err := (EndpointTemplate{}).Render(nil); body != nil || !errors.Is(err, ErrInput) {
		t.Fatal("unchecked template rendered", err)
	}
}

func TestEndpointTemplateRefusesChangedProtection(t *testing.T) {
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
			if _, err := PrepareEndpointTemplate([]byte(raw), "/installation", directory); err == nil {
				t.Fatal("changed fixed contract accepted")
			}
		})
	}
	for _, directory := range []string{"/other/generations/" + strings.Repeat("01", 32), "/installation/generations/current", "/installation/generations/" + strings.Repeat("AB", 32)} {
		if _, err := PrepareEndpointTemplate([]byte(endpointTemplateFixture), "/installation", directory); err == nil {
			t.Fatal("unbound generation path accepted")
		}
	}
}
