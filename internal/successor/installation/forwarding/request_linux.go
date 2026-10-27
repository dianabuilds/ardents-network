package forwarding

import (
	"bytes"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure"
)

var (
	ErrInput   = errors.New("forwarding installation: invalid input")
	ErrBinding = errors.New("forwarding installation: binding differs")
)

// Request declares original delivery inputs and four independent
// future semantic roots. It grants no account adoption, creation or startup.
// The fixed host/provider Hosting path must survive subsequent invocations.
type Request struct {
	Schema              string `json:"schema"`
	Invocation          string `json:"invocation"`
	Profile             string `json:"profile"`
	InstallationRoot    string `json:"installation_root"`
	StateRoot           string `json:"state_root"`
	ParticipationRoot   string `json:"participation_root"`
	SpendRoot           string `json:"spend_root"`
	HostingRoot         string `json:"hosting_root"`
	CredentialDirectory string `json:"credential_directory"`
	Plan                string `json:"plan"`
	Certificate         string `json:"certificate"`
	PrivateKey          string `json:"private_key"`
	Clock               string `json:"clock"`
}

func DecodeRequest(raw []byte) (Request, error) {
	var r Request
	if len(raw) == 0 || len(raw) > 16<<10 || json.Unmarshal(raw, &r) != nil {
		return Request{}, ErrInput
	}
	canonical, err := json.Marshal(r)
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) || r.Schema != "ardents-forwarding-installation-v1" || r.Invocation != "forwarding" || r.Profile != "ardents-forwarding-process-v1" {
		return Request{}, ErrInput
	}
	paths := []string{r.InstallationRoot, r.StateRoot, r.ParticipationRoot, r.SpendRoot, r.HostingRoot, r.CredentialDirectory, r.Plan, r.Certificate, r.PrivateKey, r.Clock}
	for i, path := range paths {
		if !validPath(path) {
			return Request{}, ErrInput
		}
		for _, previous := range paths[:i] {
			if contains(path, previous) || contains(previous, path) {
				return Request{}, ErrBinding
			}
		}
	}
	for _, reserved := range r.reservedSiblingPaths() {
		if !validPath(reserved) {
			return Request{}, ErrBinding
		}
		for _, path := range paths {
			if contains(reserved, path) || contains(path, reserved) {
				return Request{}, ErrBinding
			}
		}
	}
	for _, path := range []string{r.InstallationRoot, r.StateRoot, r.ParticipationRoot, r.SpendRoot, r.HostingRoot, r.CredentialDirectory, r.Clock} {
		for _, hidden := range []string{"/tmp", "/var/tmp", "/home", "/root", "/run/user"} {
			if contains(hidden, path) {
				return Request{}, ErrBinding
			}
		}
	}
	// Linux pathname Unix sockets reserve one of sun_path's 108 bytes for NUL.
	// Admit the whole retained lifetime namespace before any installation effect;
	// the first-start path is shorter than later restart/retirement paths.
	for n := attemptNumber(0); n < attemptLimit; n++ {
		if len(filepath.Join(n.directory(r.InstallationRoot), "start-completion.socket")) > 107 || len(filepath.Join(n.retirement(r.InstallationRoot), "stop-completion.socket")) > 107 {
			return Request{}, ErrInput
		}
	}
	return r, nil
}

// The whole finite journal namespace is reserved before initial effects. A
// delivery input, live clock, Release root or semantic root cannot occupy a
// later attempt's journal merely because that attempt has not been created.
func (r Request) reservedSiblingPaths() []string {
	paths := []string{r.InstallationRoot + ".forwarding-initial-journal", r.InstallationRoot + ".forwarding-initial-pending"}
	paths = append(paths, r.InstallationRoot+".forwarding-hosting-journal", r.InstallationRoot+".forwarding-hosting-pending", r.InstallationRoot+".forwarding-hosting")
	for n := attemptNumber(0); n < attemptLimit; n++ {
		paths = append(paths, n.journal(r.InstallationRoot), n.retirementJournal(r.InstallationRoot))
	}
	return paths
}

func (r Request) ownedPaths() []string {
	return append(r.paths(), r.reservedSiblingPaths()...)
}

// Template is a distinct closed release resource. It uses the one
// common Node account and exactly four writable semantic roots. It admits no
// alternate role, boot/restart policy, Source account or configurable shell.
const Template = `[Unit]
Description=Ardents Forwarding Node
After=network-online.target
Wants=network-online.target
[Service]
Type=exec
User=ardents-node
Group=ardents-node
SupplementaryGroups=
ExecStart=:@ARDENTS_FORWARDING_PROGRAM@ forwarding start-installed @ARDENTS_FORWARDING_BINDING@
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
ReadWritePaths=@ARDENTS_FORWARDING_WRITE_PATHS@
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

// RenderUnit binds only the fixed program/binding and four exact write
// paths. Rendering neither publishes a unit nor attests actual loaded protection.
func RenderUnit(template []byte, r Request, generation string) ([]byte, error) {
	raw, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	if _, err := DecodeRequest(append(raw, '\n')); err != nil {
		return nil, err
	}
	if string(template) != Template || len(generation) != 64 || strings.Trim(generation, "0123456789abcdef") != "" {
		return nil, ErrBinding
	}
	quote := func(path string) string { return "\"" + path + "\"" }
	return []byte(strings.NewReplacer(
		"@ARDENTS_FORWARDING_PROGRAM@", quote(filepath.Join(r.InstallationRoot, "generations", generation, infrastructure.ProgramName)),
		"@ARDENTS_FORWARDING_BINDING@", quote(filepath.Join(r.InstallationRoot, "binding.json")),
		"@ARDENTS_FORWARDING_WRITE_PATHS@", quote(r.StateRoot)+" "+quote(r.ParticipationRoot)+" "+quote(r.SpendRoot)+" "+quote(r.HostingRoot),
	).Replace(Template)), nil
}

func validPath(path string) bool {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || path == "/" || len(path) > 4096 || len(strings.Split(path, "/")) > 128 {
		return false
	}
	for _, c := range path {
		if c < 32 || c > 126 || strings.ContainsRune("%\\\"'*?[]{}$`", c) {
			return false
		}
	}
	return true
}

func contains(parent, child string) bool {
	return parent == child || strings.HasPrefix(child, parent+"/")
}
