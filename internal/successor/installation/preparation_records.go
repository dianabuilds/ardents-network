package installation

import (
	"errors"
	"unicode/utf8"
)

// Preserve the accepted preparation record identity. These records describe
// effects; neither their JSON nor an earlier phase authorizes recovery.
type preparationRecord struct {
	Schema           string `json:"schema"`
	GenerationDigest string `json:"generation_digest"`
	RequestDigest    string `json:"request_digest"`
	Phase            string `json:"phase"`
	UID              uint32 `json:"uid,omitempty"`
	GID              uint32 `json:"gid,omitempty"`
	OriginalError    string `json:"original_error,omitempty"`
}

func preparationRecordBytes(record preparationRecord) ([]byte, error) {
	if record.Schema != "ardents-endpoint-installation-preparation-v1" ||
		!canonicalDigest(record.GenerationDigest) || !canonicalDigest(record.RequestDigest) ||
		(record.UID == 0) != (record.GID == 0) || !utf8.ValidString(record.OriginalError) {
		return nil, ErrBinding
	}
	switch record.Phase {
	case "creating-account":
		if record.UID != 0 || record.OriginalError != "" {
			return nil, ErrBinding
		}
	case "creating-mutable-roots", "mutable-roots-prepared":
		if record.UID == 0 || record.OriginalError != "" {
			return nil, ErrBinding
		}
	case "preparation-failed":
		if record.OriginalError == "" {
			return nil, ErrBinding
		}
	default:
		return nil, ErrBinding
	}
	body, err := canonicalJSON(record)
	if err != nil || len(body) > 64<<10 {
		return nil, errors.Join(ErrBinding, err)
	}
	return body, nil
}

func preparationNext(previous, next preparationRecord) (string, error) {
	if _, err := preparationRecordBytes(next); err != nil {
		return "", err
	}
	if previous.Schema == "" {
		if next.Phase == "creating-account" {
			return "0001.json", nil
		}
		return "", ErrBinding
	}
	if _, err := preparationRecordBytes(previous); err != nil ||
		previous.GenerationDigest != next.GenerationDigest || previous.RequestDigest != next.RequestDigest ||
		previous.Phase == "preparation-failed" {
		return "", ErrBinding
	}
	if previous.UID != 0 && (previous.UID != next.UID || previous.GID != next.GID) {
		return "", ErrBinding
	}
	if next.Phase == "preparation-failed" {
		return "failure.json", nil
	}
	if previous.Phase == "creating-account" && next.Phase == "creating-mutable-roots" {
		return "0002.json", nil
	}
	if previous.Phase == "creating-mutable-roots" && next.Phase == "mutable-roots-prepared" {
		return "0003.json", nil
	}
	return "", ErrBinding
}
