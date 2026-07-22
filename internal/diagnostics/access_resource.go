package diagnostics

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"strconv"

	identitycontract "ardents/api/ardents/identity/v1"
)

const (
	diagnosticSubjectDomain = "ardents:diagnostic-subject-resource:v1\x00"
	MaxRecentEventsPage     = 1000
)

var diagnosticScopes = map[string]struct{}{
	"boot": {}, "configuration": {}, "data": {}, "diagnostics": {},
	"discovery": {}, "identity_access": {}, "network": {}, "node": {},
	"operator_access": {}, "policy": {}, "service": {}, "transport": {}, "workload": {},
}

func SubjectAccessResourceID(scope, resourceID string) (string, error) {
	if _, known := diagnosticScopes[scope]; !known || !validDiagnosticResourceID(resourceID) {
		return "", errors.New("diagnostic subject is invalid")
	}
	hash := sha256.New()
	_, _ = hash.Write([]byte(diagnosticSubjectDomain))
	for _, value := range []string{scope, resourceID} {
		var length [4]byte
		binary.BigEndian.PutUint32(length[:], uint32(len(value)))
		_, _ = hash.Write(length[:])
		_, _ = hash.Write([]byte(value))
	}
	return "dsr1_" + base64.RawURLEncoding.EncodeToString(hash.Sum(nil)), nil
}

func ValidateRecentEventsPage(limit int32, cursor string) error {
	if limit < 0 || limit > MaxRecentEventsPage {
		return errors.New("recent-events limit is invalid")
	}
	if cursor == "" {
		return nil
	}
	value, err := strconv.ParseInt(cursor, 10, 64)
	if err != nil || value <= 0 || strconv.FormatInt(value, 10) != cursor {
		return errors.New("recent-events cursor is invalid")
	}
	return nil
}

func validDiagnosticResourceID(value string) bool {
	if len(value) > identitycontract.MaxCanonicalResourceIDBytes {
		return false
	}
	for _, b := range []byte(value) {
		if b < 0x21 || b > 0x7e {
			return false
		}
	}
	return true
}
