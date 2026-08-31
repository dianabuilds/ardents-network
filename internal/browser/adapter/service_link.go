package browseradapter

import (
	"errors"
	"strings"
)

const (
	alphaHostSuffix = ".ard"
	alphaLinkPrefix = "ardents-alpha://"
	maximumLabelLen = 63
	maximumNameLen  = 253
	maximumDepth    = 127
)

// serviceLinkForHostname is the Browser-owned adapter for the frozen alpha
// Service Link text contract. It returns only canonical text for the local
// Application Interface and owns no resolution or Network naming behavior.
func serviceLinkForHostname(hostname string) (string, error) {
	if !strings.HasSuffix(hostname, alphaHostSuffix) || len(hostname) <= len(alphaHostSuffix) {
		return "", errors.New("browser Adapter Service Name is invalid")
	}
	name := strings.TrimSuffix(hostname, alphaHostSuffix)
	if len(name) > maximumNameLen || strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".") || strings.Contains(name, "..") {
		return "", errors.New("browser Adapter Service Name is invalid")
	}
	labels := strings.Split(name, ".")
	if len(labels) > maximumDepth {
		return "", errors.New("browser Adapter Service Name is invalid")
	}
	for _, label := range labels {
		if !canonicalLabel(label) {
			return "", errors.New("browser Adapter Service Name is invalid")
		}
	}
	if allDigits(labels[len(labels)-1]) {
		return "", errors.New("browser Adapter Service Name is invalid")
	}
	return alphaLinkPrefix + name, nil
}

func canonicalLabel(label string) bool {
	if label == "" || len(label) > maximumLabelLen || strings.HasPrefix(label, "-") ||
		strings.HasSuffix(label, "-") || strings.Contains(label, "--") {
		return false
	}
	for _, value := range []byte(label) {
		if !((value >= 'a' && value <= 'z') || (value >= '0' && value <= '9') || value == '-') {
			return false
		}
	}
	return true
}

func allDigits(label string) bool {
	for _, value := range []byte(label) {
		if value < '0' || value > '9' {
			return false
		}
	}
	return label != ""
}
