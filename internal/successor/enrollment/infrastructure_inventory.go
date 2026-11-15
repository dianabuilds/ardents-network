package enrollment

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
)

// InfrastructureInventoryName is separate from the unchanged Endpoint manifest.
const InfrastructureInventoryName = "infrastructure-enrollment.json"

// InfrastructureFacts are copied declarations. Only InfrastructureBundle also
// retains independent initial-pin and actual running-program provenance.
type InfrastructureFacts struct {
	Platform        string `json:"platform"`
	ReleaseIdentity string `json:"release_identity"`
	ReleaseVersion  int64  `json:"release_version"`
	Invocation      string `json:"invocation"`
}

type infrastructureInventory struct {
	Schema          string            `json:"schema"`
	Platform        string            `json:"platform"`
	ReleaseIdentity string            `json:"release_identity"`
	ReleaseVersion  int64             `json:"release_version"`
	Invocation      string            `json:"invocation"`
	Files           map[string]string `json:"files"`
}

type infrastructureSnapshot struct {
	facts  InfrastructureFacts
	files  map[string][]byte
	pin    string
	caller context.Context
}

// InfrastructureBundle has private construction and cannot be reconstructed
// from a public receipt, facts, candidate or supplied boolean.
type InfrastructureBundle struct{ initial *infrastructureSnapshot }

// InfrastructureCandidate supplies a frozen inventory without initial trust.
type InfrastructureCandidate struct{ candidate *infrastructureSnapshot }

// Check retains the original loader's cancellation through later handoff.
// Physical input descriptors were joined before this immutable result existed.
func (b InfrastructureBundle) Check() error    { return b.initial.check() }
func (c InfrastructureCandidate) Check() error { return c.candidate.check() }
func (s *infrastructureSnapshot) check() error {
	if s == nil || s.caller == nil {
		return ErrInput
	}
	if !matchesDigest(s.files[InfrastructureInventoryName], s.pin) {
		return ErrBinding
	}
	return s.caller.Err()
}

func (b InfrastructureBundle) Facts() (InfrastructureFacts, bool)    { return b.initial.factsCopy() }
func (c InfrastructureCandidate) Facts() (InfrastructureFacts, bool) { return c.candidate.factsCopy() }
func (b InfrastructureBundle) File(name string) ([]byte, bool)       { return b.initial.file(name) }
func (c InfrastructureCandidate) File(name string) ([]byte, bool)    { return c.candidate.file(name) }
func (s *infrastructureSnapshot) factsCopy() (InfrastructureFacts, bool) {
	if s == nil {
		return InfrastructureFacts{}, false
	}
	return s.facts, true
}
func (s *infrastructureSnapshot) file(name string) ([]byte, bool) {
	if s == nil {
		return nil, false
	}
	raw, ok := s.files[name]
	return bytes.Clone(raw), ok
}

func infrastructureNames(invocation string) []string {
	switch invocation {
	case "source":
		return []string{"ardents-node-linux-amd64", "ardents-source.service", "infrastructure-source.json", "root.json", "snapshot.json", "targets.json", "timestamp.json"}
	case "forwarding":
		return []string{"ardents-forwarding.service", "ardents-node-linux-amd64", "infrastructure-forwarding.json", "root.json", "snapshot.json", "targets.json", "timestamp.json"}
	case "introduction":
		return []string{"ardents-introduction.service", "ardents-node-linux-amd64", "infrastructure-introduction.json", "root.json", "snapshot.json", "targets.json", "timestamp.json"}
	case "descriptor":
		return []string{"ardents-descriptor.service", "ardents-node-linux-amd64", "infrastructure-descriptor.json", "root.json", "snapshot.json", "targets.json", "timestamp.json"}
	case "join":
		return []string{"ardents-join.service", "ardents-node-linux-amd64", "infrastructure-join.json", "root.json", "snapshot.json", "targets.json", "timestamp.json"}
	default:
		return nil
	}
}

func validInfrastructureFacts(f InfrastructureFacts) bool {
	// Only these exact immutable inventories have current consumers. This
	// selection grants no role dispatch, native installation or startup right.
	return f.Platform == "linux-amd64" && len(infrastructureNames(f.Invocation)) == 7 && f.ReleaseVersion > 0 && len(f.ReleaseIdentity) <= 16<<10 && strings.TrimSpace(f.ReleaseIdentity) != ""
}

// PrepareInfrastructureInventory emits public canonical bytes after all actual
// metadata exist. It signs nothing and creates neither a Bundle nor a pin grant.
func PrepareInfrastructureInventory(f InfrastructureFacts, files map[string][]byte) ([]byte, error) {
	if !validInfrastructureFacts(f) || len(files) != 7 {
		return nil, ErrInventory
	}
	d := infrastructureInventory{"ardents-infrastructure-enrollment-v1", f.Platform, f.ReleaseIdentity, f.ReleaseVersion, f.Invocation, make(map[string]string, 7)}
	for _, name := range infrastructureNames(f.Invocation) {
		raw, ok := files[name]
		if !ok || len(raw) == 0 || int64(len(raw)) > infrastructureLimit(name) {
			return nil, ErrInventory
		}
		sum := sha256.Sum256(raw)
		d.Files[name] = hex.EncodeToString(sum[:])
	}
	raw, err := json.Marshal(d)
	if err != nil || len(raw)+1 > 16<<10 {
		return nil, errors.Join(ErrInventory, err)
	}
	return append(raw, '\n'), nil
}

// ValidateInfrastructureInventory checks the canonical inventory against all
// seven bounded, immutable byte images supplied by the caller. The caller owns
// the returned declarations and digest map. They supply no initial pin, native
// custody, running-program proof, Candidate, Bundle or installation permission.
func ValidateInfrastructureInventory(raw []byte, files map[string][]byte) (InfrastructureFacts, map[string]string, error) {
	d, err := parseInfrastructureInventory(raw)
	if err != nil {
		return InfrastructureFacts{}, nil, err
	}
	facts := InfrastructureFacts{d.Platform, d.ReleaseIdentity, d.ReleaseVersion, d.Invocation}
	expected, err := PrepareInfrastructureInventory(facts, files)
	if err != nil {
		return InfrastructureFacts{}, nil, err
	}
	if !bytes.Equal(raw, expected) {
		return InfrastructureFacts{}, nil, ErrBinding
	}
	return facts, d.Files, nil
}

func parseInfrastructureInventory(raw []byte) (infrastructureInventory, error) {
	var d infrastructureInventory
	if len(raw) == 0 || len(raw) > 16<<10 || json.Unmarshal(raw, &d) != nil {
		return d, ErrInventory
	}
	f := InfrastructureFacts{d.Platform, d.ReleaseIdentity, d.ReleaseVersion, d.Invocation}
	if d.Schema != "ardents-infrastructure-enrollment-v1" || !validInfrastructureFacts(f) || len(d.Files) != 7 {
		return d, ErrInventory
	}
	for _, name := range infrastructureNames(f.Invocation) {
		if !canonicalDigest(d.Files[name]) {
			return d, ErrInventory
		}
	}
	canonical, err := json.Marshal(d)
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) {
		return d, ErrInventory
	}
	return d, nil
}

func infrastructureLimit(name string) int64 {
	switch name {
	case "infrastructure-source.json", "infrastructure-forwarding.json", "infrastructure-join.json", "infrastructure-descriptor.json", "infrastructure-introduction.json":
		return 16 << 10
	case "root.json", "snapshot.json", "targets.json", "timestamp.json":
		return 1 << 20
	default:
		return maximumFileLen
	}
}
