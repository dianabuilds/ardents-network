package launch

import (
	"bytes"
	"encoding/json"
	"errors"
	"path"
	"slices"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/admission/issuance"
	"github.com/dianabuilds/ardents-network/internal/canonicaldigest"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/pathgrammar"
	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
	"github.com/dianabuilds/ardents-network/internal/platform/systemd"
)

// Declaration is operator configuration, not a domain identity or a
// serialized authorization. Its digest is part of the loaded pre-start argv.
type Declaration struct {
	Role          string            `json:"role"`
	Distribution  string            `json:"distribution"`
	ReleaseRoot   string            `json:"release_root"`
	Environment   string            `json:"environment"`
	Network       string            `json:"network"`
	Program       string            `json:"program"`
	Configuration string            `json:"configuration"`
	ConfigSHA256  string            `json:"config_sha256"`
	WritableRoots []string          `json:"writable_roots"`
	Inputs        map[string]string `json:"inputs"`
}

var ErrInput = errors.New("platform launch refused")

func below(parent, value string) bool {
	return pathgrammar.LiteralPOSIXPath(value) && strings.HasPrefix(value, parent+"/")
}

func independentRoots(roots ...string) bool {
	for i, root := range roots {
		for _, previous := range roots[:i] {
			if root == previous || strings.HasPrefix(root, previous+"/") || strings.HasPrefix(previous, root+"/") {
				return false
			}
		}
	}
	return true
}

func valid(p Declaration) bool {
	switch p.Role {
	case "source", "forwarding", "issuer", "descriptor", "introduction", "join":
		return validInfrastructure(p)
	default:
		return false
	}
}

func validInfrastructure(p Declaration) bool {
	if !below("/usr/lib/ardents", p.Program) || !below("/etc/ardents", p.Configuration) || !pathgrammar.LiteralPOSIXPath(p.Distribution) || !pathgrammar.LiteralPOSIXPath(p.ReleaseRoot) || !independentRoots(p.Distribution, p.ReleaseRoot) || p.Environment == "" || p.Network == "" {
		return false
	}
	if !canonicaldigest.Valid(p.ConfigSHA256) || len(p.WritableRoots) < 2 || len(p.WritableRoots) > 8 {
		return false
	}
	wanted := 2
	if p.Role == "source" {
		wanted = 3
	}
	if len(p.Inputs) != wanted {
		return false
	}
	for path, digest := range p.Inputs {
		if !below("/etc/ardents", path) || !canonicaldigest.Valid(digest) {
			return false
		}
	}
	for _, root := range p.WritableRoots {
		if !below("/var/lib/ardents/roles/"+p.Role, root) || !independentRoots(root, p.Distribution, p.ReleaseRoot) {
			return false
		}
	}
	return independentRoots(p.WritableRoots...)
}

func credentialPaths(role string, raw []byte) ([]string, error) {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil {
		return nil, ErrInput
	}
	names := []string{"certificate", "private_key"}
	if role == "source" {
		names = append(names, "client_root")
	}
	paths := make([]string, 0, len(names))
	seen := map[string]bool{}
	for _, name := range names {
		var path string
		if json.Unmarshal(fields[name], &path) != nil || !below("/etc/ardents", path) || seen[path] {
			return nil, ErrInput
		}
		seen[path] = true
		paths = append(paths, path)
	}
	return paths, nil
}

func equalRoots(a, b []string) bool {
	return slices.Equal(a, b)
}

// The runtime owns complete plan and domain admission.
// This projection limits platform write access to exactly the declared domain
// roots. No root is opened, initialized, inspected as clean or repaired here.
func writableRoots(role string, raw []byte) ([]string, error) {
	var fields map[string]json.RawMessage
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if len(raw) == 0 || len(raw) > 64<<10 || artifacts.UniqueJSON(decoder) != nil || json.Unmarshal(raw, &fields) != nil || fields == nil {
		return nil, ErrInput
	}
	var network state.AuthorityDeclaration
	if artifacts.DecodeObject(fields["network"], &network) != nil {
		return nil, ErrInput
	}
	roots := []string{network.Root}
	names := []string{"participation_root", "spend_root", "hosting_root"}
	switch role {
	case "source":
		names = names[:1]
	case "introduction":
		names = append(names, "introduction_root")
	case "descriptor":
		names = append(names, "descriptor_root")
	case "issuer":
		issuanceRaw := fields["issuance"]
		prepared, err := issuance.DecodeInstalledResultSetup(issuanceRaw)
		if err != nil {
			return nil, err
		}
		// Key bytes remain read-only; only their existing lock file needs
		// a writable open. Quota/debit and results retain their own histories.
		for _, name := range names {
			var root string
			if json.Unmarshal(fields[name], &root) != nil {
				return nil, ErrInput
			}
			roots = append(roots, root)
		}
		return append(roots, prepared.AdmissionRoot, prepared.ResultRoot, path.Join(prepared.KeyRoot, "issuer.lock")), nil
	case "forwarding", "join":
	default:
		return nil, ErrInput
	}
	for _, name := range names {
		var root string
		if json.Unmarshal(fields[name], &root) != nil {
			return nil, ErrInput
		}
		roots = append(roots, root)
	}
	return roots, nil
}

func (p Declaration) unit(manifest, pin string) systemd.InfrastructureUnit {
	return systemd.InfrastructureUnit{
		Role: p.Role, Distribution: p.Distribution, Program: p.Program, Configuration: p.Configuration,
		Manifest: manifest, ManifestSHA256: pin, WritableRoots: p.WritableRoots,
	}
}
