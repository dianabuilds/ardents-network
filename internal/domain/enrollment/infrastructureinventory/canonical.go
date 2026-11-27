package infrastructureinventory

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/canonicaldigest"
)

const schema = "ardents-infrastructure-enrollment-v1"

var (
	ErrInventory = errors.New("invalid infrastructure inventory")
	ErrBinding   = errors.New("infrastructure inventory differs from supplied bytes")
)

// Facts are the declarations carried in the public inventory.
type Facts struct {
	Platform        string `json:"platform"`
	ReleaseIdentity string `json:"release_identity"`
	ReleaseVersion  int64  `json:"release_version"`
	Invocation      string `json:"invocation"`
}

type document struct {
	Schema          string            `json:"schema"`
	Platform        string            `json:"platform"`
	ReleaseIdentity string            `json:"release_identity"`
	ReleaseVersion  int64             `json:"release_version"`
	Invocation      string            `json:"invocation"`
	Files           map[string]string `json:"files"`
}

func validFacts(f Facts) bool {
	// Only these exact immutable inventories have current consumers. This
	// selection grants no role dispatch, native installation or startup right.
	return f.Platform == "linux-amd64" && len(Files(f.Invocation)) == fileCount &&
		f.ReleaseVersion > 0 && len(f.ReleaseIdentity) <= MaximumSize && strings.TrimSpace(f.ReleaseIdentity) != ""
}

// Prepare emits public canonical bytes after all actual
// metadata exist. It signs nothing and creates neither a Bundle nor a pin grant.
func Prepare(f Facts, files map[string][]byte) ([]byte, error) {
	if !validFacts(f) || len(files) != fileCount {
		return nil, ErrInventory
	}
	d := document{schema, f.Platform, f.ReleaseIdentity, f.ReleaseVersion, f.Invocation, make(map[string]string, fileCount)}
	for _, file := range Files(f.Invocation) {
		raw, ok := files[file.Name]
		if !ok || len(raw) == 0 || int64(len(raw)) > file.MaximumSize {
			return nil, ErrInventory
		}
		sum := sha256.Sum256(raw)
		d.Files[file.Name] = hex.EncodeToString(sum[:])
	}
	raw, err := json.Marshal(d)
	if err != nil || len(raw)+1 > MaximumSize {
		return nil, errors.Join(ErrInventory, err)
	}
	return append(raw, '\n'), nil
}

// Validate checks the canonical inventory against all
// seven bounded, immutable byte images supplied by the caller. The caller owns
// the returned declarations and digest map. They supply no initial pin, native
// custody, running-program proof, Candidate, Bundle or installation permission.
func Validate(raw []byte, files map[string][]byte) (Facts, map[string]string, error) {
	facts, digests, err := Parse(raw)
	if err != nil {
		return Facts{}, nil, err
	}
	expected, err := Prepare(facts, files)
	if err != nil {
		return Facts{}, nil, err
	}
	if !bytes.Equal(raw, expected) {
		return Facts{}, nil, ErrBinding
	}
	return facts, digests, nil
}

// Parse returns canonical declarations and digests without verifying companion bytes.
func Parse(raw []byte) (Facts, map[string]string, error) {
	var d document
	if len(raw) == 0 || len(raw) > MaximumSize || json.Unmarshal(raw, &d) != nil {
		return Facts{}, nil, ErrInventory
	}
	f := Facts{d.Platform, d.ReleaseIdentity, d.ReleaseVersion, d.Invocation}
	if d.Schema != schema || !validFacts(f) || len(d.Files) != fileCount {
		return Facts{}, nil, ErrInventory
	}
	for _, file := range Files(f.Invocation) {
		if !canonicaldigest.Valid(d.Files[file.Name]) {
			return Facts{}, nil, ErrInventory
		}
	}
	canonical, err := json.Marshal(d)
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) {
		return Facts{}, nil, ErrInventory
	}
	return f, d.Files, nil
}
