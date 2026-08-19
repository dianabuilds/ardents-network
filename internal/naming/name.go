package naming

import (
	"errors"
	"fmt"
	"strings"
)

// SchemaVersion is the frozen wire-format version for Stage 6 canonical Service
// Names, per R-041. Any future change to the encoding requires a new
// schema_version and a new research record.
const SchemaVersion uint16 = 1

const (
	serviceLinkScheme = "ardents://"
	maxLabelLength    = 63
	maxNameLength     = 253
	maxNameDepth      = 127
)

// Name is a canonical Service Name in the Stage 6 namespace model.
type Name string

// Parse parses a canonical Service Name without any explicit scheme.
func Parse(raw string) (Name, error) {
	return parseName(raw, false, false)
}

// ParseServiceLink parses `ardents://<Service Name>` and canonicalizes the
// label case.
func ParseServiceLink(raw string) (Name, error) {
	return parseName(raw, true, true)
}

func canonicalize(raw string) (Name, error) {
	return parseName(raw, false, true)
}

func parseName(raw string, allowServiceLink bool, canonicalize bool) (Name, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return "", errors.New("invalid Service Name: empty input")
	}

	if allowServiceLink {
		lower := strings.ToLower(text)
		if !strings.HasPrefix(lower, serviceLinkScheme) {
			return "", errors.New("invalid Service Name: missing ardents:// scheme")
		}
		text = text[len(serviceLinkScheme):]
	} else if strings.Contains(text, "://") {
		return "", errors.New("invalid Service Name: must not contain URL scheme")
	}

	if canonicalize {
		text = strings.ToLower(text)
	}
	labels, err := parseLabels(text)
	if err != nil {
		return "", fmt.Errorf("invalid Service Name %q: %w", raw, err)
	}

	if len(labels) > maxNameDepth {
		return "", errors.New("namespace depth exceeds Stage 6 bound")
	}
	totalLength := 0
	for i := 0; i < len(labels); i++ {
		totalLength += len(labels[i])
		if i < len(labels)-1 {
			totalLength++
		}
	}
	if totalLength > maxNameLength {
		return "", errors.New("serialized Service Name exceeds Stage 6 bound")
	}

	return Name(joinDots(labels)), nil
}

func parseLabels(text string) ([]string, error) {
	if text == "" {
		return nil, errors.New("empty Service Name")
	}
	if strings.HasPrefix(text, ".") || strings.HasSuffix(text, ".") || strings.Contains(text, "..") {
		return nil, errors.New("name must contain dot-separated labels without empty segments")
	}
	segments := strings.Split(text, ".")
	labels := make([]string, 0, len(segments))
	for _, segment := range segments {
		if segment == "" {
			return nil, errors.New("label is empty")
		}
		if len(segment) > maxLabelLength {
			return nil, fmt.Errorf("label %q exceeds %d bytes", segment, maxLabelLength)
		}
		if segment != strings.ToLower(segment) {
			return nil, errors.New("label contains uppercase rune")
		}
		if strings.HasPrefix(segment, "-") || strings.HasSuffix(segment, "-") {
			return nil, fmt.Errorf("label %q must not start or end with -", segment)
		}
		if strings.Contains(segment, "--") {
			return nil, fmt.Errorf("label %q must not contain consecutive hyphens", segment)
		}
		for _, char := range segment {
			if char >= 128 || (!((char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '-')) {
				return nil, fmt.Errorf("label %q contains non-canonical character %q", segment, char)
			}
		}
		labels = append(labels, segment)
	}
	if len(labels) > maxNameDepth {
		return nil, errors.New("namespace depth exceeds Stage 6 bound")
	}
	if isAllDigit(labels[len(labels)-1]) {
		return nil, errRootAllDigit
	}
	return labels, nil
}

func labelsOf(name Name) ([]string, error) {
	return parseLabels(string(name))
}

func isDescendant(child, parent Name) bool {
	parsedChild, err := parseName(string(child), false, false)
	if err != nil {
		return false
	}
	parsedParent, err := parseName(string(parent), false, false)
	if err != nil {
		return false
	}
	if parsedChild == parsedParent {
		return false
	}
	return strings.HasSuffix(string(parsedChild), "."+string(parsedParent))
}

func isAllDigit(label string) bool {
	if label == "" {
		return false
	}
	for _, c := range label {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func joinDots(labels []string) string {
	out := ""
	for i, l := range labels {
		if i > 0 {
			out += "."
		}
		out += l
	}
	return out
}
