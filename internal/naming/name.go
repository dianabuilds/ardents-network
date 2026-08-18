package naming

import (
	"errors"
	"fmt"
	"strings"
)

const (
	serviceLinkScheme = "ardents://"
	maxLabelLength    = 63
	maxNameLength     = 255
	maxNameDepth      = 8
)

// Name is a canonical Service Name in the Stage 6 namespace model.
type Name string

// ParseError is returned when a candidate name is structurally invalid.
type ParseError struct {
	Input  string
	Reason string
}

func (e ParseError) Error() string {
	return fmt.Sprintf("invalid Service Name %q: %s", e.Input, e.Reason)
}

// Parse parses a canonical Service Name without any explicit scheme.
func Parse(raw string) (Name, error) {
	return parseName(raw, false, false)
}

// ParseServiceLink parses `ardents://<Service Name>` and canonicalizes the label case.
func ParseServiceLink(raw string) (Name, error) {
	return parseName(raw, true, true)
}

// Canonicalize canonicalizes and validates a Service Name candidate.
func Canonicalize(raw string) (Name, error) {
	return parseName(raw, false, true)
}

// Labels returns canonical label segments for a validated Service Name.
func Labels(name Name) ([]string, error) {
	labels, err := parseLabels(string(name))
	if err != nil {
		return nil, err
	}
	return append([]string(nil), labels...), nil
}

// IsDescendant reports whether child is in the namespace subtree of parent.
func IsDescendant(child, parent Name) bool {
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

func parseName(raw string, allowServiceLink bool, canonicalize bool) (Name, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return "", ParseError{Input: raw, Reason: "empty input"}
	}

	if allowServiceLink {
		lower := strings.ToLower(text)
		if !strings.HasPrefix(lower, serviceLinkScheme) {
			return "", ParseError{Input: raw, Reason: "missing ardents:// scheme"}
		}
		text = text[len(serviceLinkScheme):]
	} else if strings.Contains(text, "://") {
		return "", ParseError{Input: raw, Reason: "name must not contain URL scheme"}
	}

	labels, err := parseLabels(text)
	if err != nil {
		if !canonicalize || !strings.Contains(err.Error(), "uppercase") {
			return "", ParseError{Input: raw, Reason: err.Error()}
		}
	}
	if canonicalize {
		text = strings.ToLower(text)
		labels, err = parseLabels(text)
	}
	if err != nil {
		return "", ParseError{Input: raw, Reason: err.Error()}
	}

	if len(labels) > maxNameDepth {
		return "", ParseError{Input: raw, Reason: "namespace depth exceeds Stage 6 bound"}
	}
	totalLength := 0
	for i := 0; i < len(labels); i++ {
		totalLength += len(labels[i])
		if i < len(labels)-1 {
			totalLength++
		}
	}
	if totalLength > maxNameLength {
		return "", ParseError{Input: raw, Reason: "serialized Service Name exceeds Stage 6 bound"}
	}

	if canonicalize {
		return Name(strings.Join(labels, ".")), nil
	}
	return Name(strings.Join(labels, ".")), nil
}

func parseLabels(text string) ([]string, error) {
	if text == "" {
		return nil, errors.New("empty Service Name")
	}
	if strings.HasPrefix(text, ".") || strings.HasSuffix(text, ".") || strings.Contains(text, "..") {
		return nil, errors.New("Service Name must contain dot-separated labels without empty segments")
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
	if len(labels) == 0 || !isASCIIAlphabetic(labels[len(labels)-1][0]) {
		return nil, errors.New("root label must include only lowercase letters, digits, and hyphen")
	}
	return labels, nil
}

func isASCIIAlphabetic(b byte) bool {
	return b >= 'a' && b <= 'z'
}
