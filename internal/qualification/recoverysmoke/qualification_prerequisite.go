package recoverysmoke

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/dianabuilds/ardents-network/internal/qualification/byteio"
	"github.com/dianabuilds/ardents-network/internal/qualification/recovery"
	"github.com/dianabuilds/ardents-network/internal/qualification/service"
)

type qualificationPrerequisite struct {
	Stage, SourceCommit, EvidenceDigest string
	raw                                 []byte
}

func loadQualificationPrerequisites(input config, sourceCommit string) ([]qualificationPrerequisite, error) {
	s41Raw, err := byteio.ReadFile(input.S41Evidence, 4<<20)
	if err != nil {
		return nil, fmt.Errorf("read S4.1 prerequisite evidence: %w", err)
	}
	var s41 recovery.Evidence
	decoder := json.NewDecoder(bytes.NewReader(s41Raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&s41); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return nil, errors.Join(err, errors.New("S4.1 prerequisite evidence is malformed"))
	}
	if verdict := recovery.Verify(s41); verdict.Verdict != "pass" || s41.SourceCommit != sourceCommit ||
		s41.Claim != "S4.1 local development evidence only" {
		return nil, errors.New("S4.1 prerequisite does not pass for the current source")
	}
	values := []qualificationPrerequisite{{Stage: "S4.1", SourceCommit: sourceCommit,
		EvidenceDigest: digestPrerequisite(s41Raw), raw: s41Raw}}
	if input.Slice == "s4.3" {
		s42Raw, readErr := byteio.ReadFile(input.S42Evidence, 52<<20)
		if readErr != nil {
			return nil, fmt.Errorf("read S4.2 prerequisite evidence: %w", readErr)
		}
		var envelope recovery.Evidence
		if err := decodeQualificationJSON(s42Raw, &envelope); err != nil {
			return nil, errors.Join(err, errors.New("S4.2 prerequisite evidence is malformed"))
		}
		manifestSourceCommit, err := sourceCommitFromManifest(envelope.AttemptManifest)
		if err != nil || recovery.Verify(envelope).Verdict != "pass" || manifestSourceCommit != sourceCommit {
			return nil, errors.Join(err, errors.New("S4.2 prerequisite does not pass for the current source"))
		}
		values = append(values, qualificationPrerequisite{Stage: "S4.2", SourceCommit: sourceCommit,
			EvidenceDigest: digestPrerequisite(s42Raw), raw: s42Raw})
	}
	s3Raw, err := byteio.ReadFile(input.Stage3Evidence, 4<<20)
	if err != nil {
		return nil, fmt.Errorf("read Stage 3 prerequisite evidence: %w", err)
	}
	stage3Verdict := service.Verify(s3Raw)
	var stage3 struct {
		SourceCommit string `json:"source_commit"`
	}
	if err := json.Unmarshal(s3Raw, &stage3); err != nil || stage3Verdict.Verdict != "pass" ||
		stage3.SourceCommit != sourceCommit {
		return nil, errors.Join(err, errors.New("Stage 3 prerequisite does not pass for the current source"))
	}
	return append(values, qualificationPrerequisite{Stage: "Stage 3", SourceCommit: sourceCommit,
		EvidenceDigest: stage3Verdict.EvidenceDigest, raw: s3Raw}), nil
}

func retainQualificationPrerequisites(root string, values []qualificationPrerequisite) error {
	directory := filepath.Join(root, "prerequisites")
	if err := os.Mkdir(directory, 0o700); err != nil {
		return fmt.Errorf("create qualification prerequisite directory: %w", err)
	}
	for _, value := range values {
		name := map[string]string{"S4.1": "s4.1-evidence.json", "S4.2": "s4.2-evidence.json",
			"Stage 3": "stage3-evidence.json"}[value.Stage]
		if name == "" || digestPrerequisite(value.raw) != value.EvidenceDigest && value.Stage != "Stage 3" {
			return errors.New("qualification prerequisite retention input is inconsistent")
		}
		path := filepath.Join(directory, name)
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			return fmt.Errorf("create immutable prerequisite %s: %w", value.Stage, err)
		}
		_, writeErr := file.Write(value.raw)
		syncErr := file.Sync()
		closeErr := file.Close()
		if err := errors.Join(writeErr, syncErr, closeErr); err != nil {
			return fmt.Errorf("retain prerequisite %s: %w", value.Stage, err)
		}
	}
	return nil
}

func decodeQualificationJSON(raw []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("qualification evidence contains multiple JSON values")
	}
	return nil
}

func sourceCommitFromManifest(raw []byte) (string, error) {
	var manifest struct{ SourceCommit string }
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := decoder.Decode(&manifest); err != nil {
		return "", err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return "", errors.New("qualification manifest contains multiple JSON values")
	}
	return manifest.SourceCommit, nil
}

func digestPrerequisite(raw []byte) string {
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}
