package naming

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// Wire errors. Unexported because they are test-internal; production
// callers compare with the package's Parse / EncodeWire / DecodeWire
// return values through fmt.Errorf wrapping and standard errors.Is /
// errors.As patterns.
var (
	errWireTooShort    = errors.New("naming wire: input shorter than schema_version prefix")
	errWireBadVersion  = errors.New("naming wire: schema_version does not match frozen Stage 6 profile")
	errWireLabelLength = errors.New("naming wire: declared label length exceeds Stage 6 bound")
	errWireLabelTrunc  = errors.New("naming wire: input truncated before label body completes")
	errWireEmpty       = errors.New("naming wire: decoded name is empty")
	errWireBadAllDigit = errors.New("naming wire: root label is all-digit (R-041)")
	errRootAllDigit    = errors.New("root label must not be all-digit (R-041)")
)

// EncodeWire serializes a canonical Service Name into the frozen Stage 6
// length-prefixed wire format. The output is exactly
//
//	uint16(schema_version) || (uint8(len(label)) || label_bytes)+
//
// ordered from leaf to root, per R-041.
func EncodeWire(name Name) ([]byte, error) {
	if name == "" {
		return nil, errors.New("naming wire: empty name")
	}
	labels, err := labelsOf(name)
	if err != nil {
		return nil, err
	}
	out := make([]byte, 0, 2+sumLabelLens(labels))
	var version [2]byte
	binary.BigEndian.PutUint16(version[:], SchemaVersion)
	out = append(out, version[:]...)
	for _, label := range labels {
		if len(label) == 0 || len(label) > 255 {
			return nil, fmt.Errorf("naming wire: label %q has invalid length", label)
		}
		out = append(out, byte(len(label)))
		out = append(out, label...)
	}
	return out, nil
}

// DecodeWire parses a Stage 6 wire-format name. It rejects any
// schema_version other than the frozen one (R-041), any non-canonical
// label, an all-digit root label, and any structural inconsistency.
func DecodeWire(in []byte) (Name, error) {
	if len(in) < 2 {
		return "", errWireTooShort
	}
	version := binary.BigEndian.Uint16(in[:2])
	if version != SchemaVersion {
		return "", fmt.Errorf("%w: got %d, want %d", errWireBadVersion, version, SchemaVersion)
	}
	labels := make([]string, 0, 4)
	i := 2
	for i < len(in) {
		if i+1 > len(in) {
			return "", errWireLabelTrunc
		}
		n := int(in[i])
		i++
		if n == 0 || n > maxLabelLength {
			return "", fmt.Errorf("%w: %d", errWireLabelLength, n)
		}
		if i+n > len(in) {
			return "", errWireLabelTrunc
		}
		labels = append(labels, string(in[i:i+n]))
		i += n
	}
	if len(labels) == 0 {
		return "", errWireEmpty
	}
	canonical := joinDots(labels)
	if _, err := parseLabels(canonical); err != nil {
		if errors.Is(err, errRootAllDigit) {
			return "", errWireBadAllDigit
		}
		return "", fmt.Errorf("naming wire: %w", err)
	}
	return Name(canonical), nil
}

func sumLabelLens(labels []string) int {
	n := 0
	for _, l := range labels {
		n += 1 + len(l)
	}
	return n
}
