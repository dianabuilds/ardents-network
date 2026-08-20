//go:build ignore

package profile

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

const (
	schemaVersion       = "1"
	profileID           = "ardents-h3-stage-7-evidence-v1"
	logicalCellCount    = 91
	maximumCampaignJSON = 16 << 20
	maximumEvidence     = uint64(1 << 30)
	streamBufferBytes   = 64 << 10
)

type digestReference struct {
	Ordinal uint16 `json:"ordinal"`
	SHA256  string `json:"sha256"`
}

type campaign struct {
	Schema          string            `json:"schema"`
	Profile         string            `json:"profile"`
	RunID           string            `json:"run_id"`
	SourceCommit    string            `json:"source_commit"`
	SourceClean     bool              `json:"source_clean"`
	MaximumEvidence uint64            `json:"maximum_evidence_bytes"`
	Hosts           []digestReference `json:"hosts"`
	Cells           []digestReference `json:"cells"`
}

type attemptFacts struct {
	StructureValid  bool
	ObserverValid   bool
	SecretsClean    bool
	BehaviorValid   bool
	CleanupObserved bool
	CleanupValid    bool
}

func canonical(value any) ([]byte, error) { return json.Marshal(value) }

func admitCampaign(raw []byte) (campaign, error) {
	var admitted campaign
	if len(raw) == 0 || len(raw) > maximumCampaignJSON {
		return admitted, errors.New("campaign size is invalid")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&admitted); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return campaign{}, errors.New("campaign JSON is not one strict value")
	}
	reencoded, err := canonical(admitted)
	if err != nil || !bytes.Equal(raw, reencoded) {
		return campaign{}, errors.New("campaign JSON is not canonical")
	}
	if err := validateCampaign(admitted); err != nil {
		return campaign{}, err
	}
	return admitted, nil
}

func validateCampaign(value campaign) error {
	if value.Schema != schemaVersion || value.Profile != profileID || !value.SourceClean {
		return errors.New("campaign profile or source state is invalid")
	}
	if !validDigest(value.RunID) || !validDigest(value.SourceCommit) ||
		value.MaximumEvidence != maximumEvidence {
		return errors.New("campaign identity or bound is invalid")
	}
	if len(value.Hosts) != 2 || len(value.Cells) != logicalCellCount {
		return errors.New("campaign inventory is incomplete")
	}
	if err := validateReferences(value.Hosts); err != nil {
		return fmt.Errorf("host references: %w", err)
	}
	if err := validateReferences(value.Cells); err != nil {
		return fmt.Errorf("cell references: %w", err)
	}
	return nil
}

func streamDigest(reader io.Reader, expectedBytes uint64) (string, error) {
	if expectedBytes > maximumEvidence {
		return "", errors.New("evidence length exceeds the precommitted bound")
	}
	hasher := sha256.New()
	limited := io.LimitReader(reader, int64(expectedBytes)+1)
	count, err := io.CopyBuffer(hasher, limited, make([]byte, streamBufferBytes))
	if err != nil {
		return "", fmt.Errorf("stream evidence: %w", err)
	}
	if uint64(count) != expectedBytes {
		return "", errors.New("evidence length differs from its commitment")
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func validateReferences(references []digestReference) error {
	for index, reference := range references {
		if int(reference.Ordinal) != index || !validDigest(reference.SHA256) {
			return errors.New("references are not contiguous canonical commitments")
		}
	}
	return nil
}

func validDigest(value string) bool {
	if len(value) != sha256.Size*2 || value != strings.ToLower(value) {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size
}

func safeRelativePath(value string) bool {
	if len(value) == 0 || len(value) > 240 || strings.ContainsAny(value, "\\:%") ||
		strings.HasPrefix(value, "/") {
		return false
	}
	for _, segment := range strings.Split(value, "/") {
		if !safeSegment(segment) {
			return false
		}
	}
	return true
}

func safeSegment(segment string) bool {
	if len(segment) == 0 || len(segment) > 64 || segment == "." || segment == ".." ||
		strings.HasSuffix(segment, ".") {
		return false
	}
	base := strings.SplitN(strings.ToLower(segment), ".", 2)[0]
	if base == "con" || base == "prn" || base == "aux" || base == "nul" ||
		(len(base) == 4 && (strings.HasPrefix(base, "com") || strings.HasPrefix(base, "lpt")) &&
			base[3] >= '1' && base[3] <= '9') {
		return false
	}
	for _, character := range segment {
		if (character < 'a' || character > 'z') && (character < '0' || character > '9') &&
			character != '.' && character != '_' && character != '-' {
			return false
		}
	}
	return true
}

func reduce(facts []attemptFacts) string {
	if len(facts) == 0 {
		return "invalid"
	}
	for _, fact := range facts {
		if !fact.StructureValid || !fact.ObserverValid || !fact.SecretsClean || !fact.CleanupObserved {
			return "invalid"
		}
	}
	for _, fact := range facts {
		if !fact.BehaviorValid || !fact.CleanupValid {
			return "fail"
		}
	}
	return "pass"
}
