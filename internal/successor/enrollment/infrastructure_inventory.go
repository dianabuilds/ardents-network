//go:build linux

package enrollment

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
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

func infrastructureNames() []string {
	return []string{"ardents-node-linux-amd64", "ardents-source.service", "infrastructure-source.json", "root.json", "snapshot.json", "targets.json", "timestamp.json"}
}

func validInfrastructureFacts(f InfrastructureFacts) bool {
	// Other invocation contracts have no accepting path in this component.
	return f.Platform == "linux-amd64" && f.Invocation == "source" && f.ReleaseVersion > 0 && len(f.ReleaseIdentity) <= 16<<10 && strings.TrimSpace(f.ReleaseIdentity) != ""
}

// PrepareInfrastructureInventory emits public canonical bytes after all actual
// metadata exist. It signs nothing and creates neither a Bundle nor a pin grant.
func PrepareInfrastructureInventory(f InfrastructureFacts, files map[string][]byte) ([]byte, error) {
	if !validInfrastructureFacts(f) || len(files) != 7 {
		return nil, ErrInventory
	}
	d := infrastructureInventory{"ardents-infrastructure-enrollment-v1", f.Platform, f.ReleaseIdentity, f.ReleaseVersion, f.Invocation, make(map[string]string, 7)}
	for _, name := range infrastructureNames() {
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

func parseInfrastructureInventory(raw []byte) (infrastructureInventory, error) {
	var d infrastructureInventory
	if len(raw) == 0 || len(raw) > 16<<10 || json.Unmarshal(raw, &d) != nil {
		return d, ErrInventory
	}
	f := InfrastructureFacts{d.Platform, d.ReleaseIdentity, d.ReleaseVersion, d.Invocation}
	if d.Schema != "ardents-infrastructure-enrollment-v1" || !validInfrastructureFacts(f) || len(d.Files) != 7 {
		return d, ErrInventory
	}
	for _, name := range infrastructureNames() {
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
	case "infrastructure-source.json":
		return 16 << 10
	case "root.json", "snapshot.json", "targets.json", "timestamp.json":
		return 1 << 20
	default:
		return maximumFileLen
	}
}

// VerifyInfrastructure checks the independent pin before parsing, then the
// seven original bounded files and the actual executable of this process.
// Self-verification does not establish trust in its first execution.
func VerifyInfrastructure(ctx context.Context, root, pin string) (InfrastructureBundle, error) {
	if !canonicalDigest(pin) {
		return InfrastructureBundle{}, ErrInput
	}
	s, err := loadInfrastructure(ctx, root, pin)
	if err != nil {
		return InfrastructureBundle{}, err
	}
	return InfrastructureBundle{initial: s}, nil
}

func ReadInfrastructureCandidate(ctx context.Context, root string) (InfrastructureCandidate, error) {
	s, err := loadInfrastructure(ctx, root, "")
	if err != nil {
		return InfrastructureCandidate{}, err
	}
	return InfrastructureCandidate{candidate: s}, nil
}

func checkInfrastructureDirectory(root *os.Root) (err error) {
	d, err := root.Open(".")
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, d.Close()) }()
	entries, err := d.ReadDir(9)
	if err != nil && err != io.EOF {
		return err
	}
	if len(entries) != 8 {
		return ErrInventory
	}
	wanted := map[string]bool{InfrastructureInventoryName: true}
	for _, name := range infrastructureNames() {
		wanted[name] = true
	}
	for _, e := range entries {
		if !wanted[e.Name()] {
			return ErrInventory
		}
	}
	return nil
}

func loadInfrastructure(ctx context.Context, path, pin string) (result *infrastructureSnapshot, err error) {
	if ctx == nil || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, ErrInput
	}
	if err := infrastructurePlatform(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ancestors := map[string]os.FileInfo{}
	for p := path; ; p = filepath.Dir(p) {
		info, e := os.Lstat(p)
		if e != nil {
			return nil, e
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return nil, ErrInventory
		}
		ancestors[p] = info
		if filepath.Dir(p) == p {
			break
		}
	}
	original, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !original.IsDir() || original.Mode()&os.ModeSymlink != 0 {
		return nil, ErrInventory
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = errors.Join(err, root.Close(), ctx.Err())
		if err != nil {
			result = nil
		}
	}()
	opened, err := root.Lstat(".")
	if err != nil {
		return nil, err
	}
	if !os.SameFile(original, opened) {
		return nil, ErrBinding
	}
	read := func(name string, limit int64) ([]byte, os.FileInfo, error) {
		before, e := root.Lstat(name)
		if e != nil {
			return nil, nil, e
		}
		raw, after, e := readBundleFile(ctx, root, name, limit)
		if e != nil {
			return nil, nil, e
		}
		if !sameInfrastructureFile(before, after) {
			return nil, nil, ErrBinding
		}
		return raw, after, nil
	}
	raw, info, err := read(InfrastructureInventoryName, 16<<10)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(raw)
	observedPin := hex.EncodeToString(sum[:])
	if pin != "" && pin != observedPin {
		return nil, ErrPin
	}
	d, err := parseInfrastructureInventory(raw)
	if err != nil {
		return nil, err
	}
	if err := checkInfrastructureDirectory(root); err != nil {
		return nil, err
	}
	files := map[string][]byte{InfrastructureInventoryName: raw}
	identities := map[string]os.FileInfo{InfrastructureInventoryName: info}
	for _, name := range infrastructureNames() {
		body, fi, err := read(name, infrastructureLimit(name))
		if err != nil {
			return nil, err
		}
		if !matchesDigest(body, d.Files[name]) {
			return nil, ErrBinding
		}
		files[name], identities[name] = body, fi
	}
	if pin != "" {
		if err := verifyInfrastructureProcess(ctx, identities["ardents-node-linux-amd64"], files["ardents-node-linux-amd64"]); err != nil {
			return nil, err
		}
	}
	if err := checkInfrastructureDirectory(root); err != nil {
		return nil, err
	}
	for name, before := range identities {
		after, err := root.Lstat(name)
		if err != nil {
			return nil, err
		}
		if !sameInfrastructureFile(before, after) {
			return nil, ErrBinding
		}
		if err := verifyOwnedFile(after); err != nil {
			return nil, err
		}
	}
	after, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !os.SameFile(original, after) || original.Mode() != after.Mode() {
		return nil, ErrBinding
	}
	if pin != "" {
		if err := verifyInfrastructureProcess(ctx, identities["ardents-node-linux-amd64"], files["ardents-node-linux-amd64"]); err != nil {
			return nil, err
		}
	}
	for name, before := range ancestors {
		after, e := os.Lstat(name)
		if e != nil {
			return nil, e
		}
		if !os.SameFile(before, after) || before.Mode() != after.Mode() {
			return nil, ErrBinding
		}
	}
	return &infrastructureSnapshot{InfrastructureFacts{d.Platform, d.ReleaseIdentity, d.ReleaseVersion, d.Invocation}, files, observedPin, ctx}, nil
}
