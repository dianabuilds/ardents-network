package catalog

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"

	"github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"
)

var errInvalidContentReference = errors.New("content reference is invalid")

// ContentReference is a canonical CIDv1 raw SHA2-256 content identity.
// Its zero value is invalid.
type ContentReference struct {
	digest [sha256.Size]byte
	valid  bool
}

// ParseContentReference parses only the canonical lowercase representation.
func ParseContentReference(value string) (ContentReference, error) {
	parsed, err := cid.Decode(value)
	if err != nil || parsed.Version() != 1 || parsed.Type() != cid.Raw || parsed.String() != value {
		return ContentReference{}, errInvalidContentReference
	}
	decoded, err := mh.Decode(parsed.Hash())
	if err != nil || decoded.Code != mh.SHA2_256 || decoded.Length != sha256.Size || len(decoded.Digest) != sha256.Size {
		return ContentReference{}, errInvalidContentReference
	}
	var digest [sha256.Size]byte
	copy(digest[:], decoded.Digest)
	if digest == ([sha256.Size]byte{}) {
		return ContentReference{}, errInvalidContentReference
	}
	return ContentReference{digest: digest, valid: true}, nil
}

// String returns the canonical representation, or an empty string for zero.
func (reference ContentReference) String() string {
	if !reference.valid {
		return ""
	}
	encoded, err := mh.Encode(reference.digest[:], mh.SHA2_256)
	if err != nil {
		return ""
	}
	return cid.NewCidV1(cid.Raw, encoded).String()
}

// Equal reports whether two references contain the same identity.
func (reference ContentReference) Equal(other ContentReference) bool {
	return reference == other
}

// MarshalText emits the canonical CID representation.
func (reference ContentReference) MarshalText() ([]byte, error) {
	value := reference.String()
	if value == "" {
		return nil, errInvalidContentReference
	}
	return []byte(value), nil
}

// UnmarshalText accepts only a canonical CID representation.
func (reference *ContentReference) UnmarshalText(text []byte) error {
	if reference == nil {
		return errInvalidContentReference
	}
	parsed, err := ParseContentReference(string(text))
	if err != nil {
		return err
	}
	*reference = parsed
	return nil
}

// MarshalJSON emits a JSON string through the text codec.
func (reference ContentReference) MarshalJSON() ([]byte, error) {
	text, err := reference.MarshalText()
	if err != nil {
		return nil, err
	}
	return json.Marshal(string(text))
}

// UnmarshalJSON accepts a JSON string through the text codec.
func (reference *ContentReference) UnmarshalJSON(payload []byte) error {
	trimmed := bytes.TrimSpace(payload)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) || trimmed[0] != '"' {
		return errInvalidContentReference
	}
	var text string
	if err := json.Unmarshal(trimmed, &text); err != nil {
		return errInvalidContentReference
	}
	return reference.UnmarshalText([]byte(text))
}
