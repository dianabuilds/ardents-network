package infrastructure

import (
	"bytes"
	"encoding/json"
	"net/netip"
	"path/filepath"
	"strings"
)

// SourceRequest describes existing inputs and separate, still absent native
// destinations. It is a declaration, never permission to create or start them.
type SourceRequest struct {
	Schema              string `json:"schema"`
	Invocation          string `json:"invocation"`
	Profile             string `json:"profile"`
	InstallationRoot    string `json:"installation_root"`
	StateRoot           string `json:"state_root"`
	ParticipationRoot   string `json:"participation_root"`
	CredentialDirectory string `json:"credential_directory"`
	Plan                string `json:"plan"`
	Certificate         string `json:"certificate"`
	PrivateKey          string `json:"private_key"`
	ClientCA            string `json:"client_ca"`
	Clock               string `json:"clock"`
	Address             string `json:"address"`
}

// DecodeSourceRequest requires an independent canonical byte oracle: duplicate,
// unknown, reordered or omitted fields cannot survive exact re-encoding.
func DecodeSourceRequest(raw []byte) (SourceRequest, error) {
	var r SourceRequest
	if len(raw) == 0 || len(raw) > 16<<10 || json.Unmarshal(raw, &r) != nil {
		return r, ErrInput
	}
	canonical, err := json.Marshal(r)
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) || r.Schema != "ardents-source-preflight-v1" || r.Invocation != "source" || r.Profile != "ardents-source-process-v1" {
		return SourceRequest{}, ErrInput
	}
	address, err := netip.ParseAddrPort(r.Address)
	if err != nil || address.String() != r.Address || address.Port() < 1024 || address.Addr().IsUnspecified() || address.Addr().IsMulticast() {
		return SourceRequest{}, ErrInput
	}
	paths := r.paths()
	for i, p := range paths {
		if !sourcePath(p) {
			return SourceRequest{}, ErrInput
		}
		for _, other := range paths[:i] {
			if contains(p, other) || contains(other, p) {
				return SourceRequest{}, ErrBinding
			}
		}
	}
	// The fixed release template hides these host locations. Runtime resources
	// must remain visible as the exact admitted host inodes, not private tmp
	// replacements or inaccessible homes. Root-only original delivery inputs are
	// separate: Installation reads them before delivering sealed runtime copies.
	for _, path := range []string{r.InstallationRoot, r.StateRoot, r.ParticipationRoot, r.CredentialDirectory, r.Clock} {
		for _, hidden := range []string{"/tmp", "/var/tmp", "/home", "/root", "/run/user"} {
			if contains(hidden, path) {
				return SourceRequest{}, ErrBinding
			}
		}
	}
	return r, nil
}

func (r SourceRequest) paths() []string {
	return []string{r.InstallationRoot, r.StateRoot, r.ParticipationRoot, r.CredentialDirectory, r.Plan, r.Certificate, r.PrivateKey, r.ClientCA, r.Clock}
}
func sourcePath(p string) bool {
	if !filepath.IsAbs(p) || filepath.Clean(p) != p || p == "/" {
		return false
	}
	for _, c := range p {
		if c < 32 || c > 126 || strings.ContainsRune("%\\\"'*?[]{}$`", c) {
			return false
		}
	}
	return true
}
func contains(parent, child string) bool {
	return parent == child || strings.HasPrefix(child, parent+"/")
}

// SourceTemplate is the exact release resource. Future rendering is restricted
// to one executable/binding and the two independently admitted writable roots.
// It deliberately has no Endpoint policy switch or arbitrary service selector.
const SourceTemplate = `[Unit]
Description=Ardents Direct-Origin Source
After=network-online.target
Wants=network-online.target
[Service]
Type=exec
User=ardents-source
Group=ardents-source
SupplementaryGroups=
ExecStart=:@ARDENTS_SOURCE_PROGRAM@ source start-installed @ARDENTS_SOURCE_BINDING@
WorkingDirectory=/
UMask=0077
Restart=no
RemainAfterExit=no
KillMode=control-group
TimeoutStopSec=30s
NoNewPrivileges=yes
CapabilityBoundingSet=
AmbientCapabilities=
PrivateTmp=yes
ProtectHome=yes
ProtectSystem=strict
ReadWritePaths=@ARDENTS_SOURCE_WRITE_PATHS@
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
CPUQuota=100%
CPUQuotaPeriodSec=100ms
MemoryHigh=805306368
MemoryMax=1073741824
TasksMax=256
LimitNOFILE=1024
LimitCORE=0
Environment=GOMAXPROCS=1 GOMEMLIMIT=536870912
StandardOutput=journal
StandardError=journal
`

// RenderSourceUnit validates the complete closed template before substituting
// exact future paths. The returned bytes attest no installed/loaded policy.
func RenderSourceUnit(template []byte, r SourceRequest, generation string) ([]byte, error) {
	raw, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	if _, err := DecodeSourceRequest(append(raw, '\n')); err != nil {
		return nil, err
	}
	if string(template) != SourceTemplate || len(generation) != 64 || strings.Trim(generation, "0123456789abcdef") != "" {
		return nil, ErrBinding
	}
	quote := func(p string) string { return "\"" + p + "\"" }
	return []byte(strings.NewReplacer(
		"@ARDENTS_SOURCE_PROGRAM@", quote(filepath.Join(r.InstallationRoot, "generations", generation, ProgramName)),
		"@ARDENTS_SOURCE_BINDING@", quote(filepath.Join(r.InstallationRoot, "binding.json")),
		"@ARDENTS_SOURCE_WRITE_PATHS@", quote(r.StateRoot)+" "+quote(r.ParticipationRoot),
	).Replace(SourceTemplate)), nil
}
