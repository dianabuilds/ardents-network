package recoverysmoke

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/dianabuilds/ardents-network/internal/qualification/byteio"
	"github.com/dianabuilds/ardents-network/internal/qualification/campaign"
	"github.com/dianabuilds/ardents-network/internal/qualification/recovery"
)

type replacementCampaignIndex struct {
	Schema, ManifestDigest string
	Attempts               []replacementCampaignAttempt
}

type replacementCampaignAttempt struct {
	CellID, AttemptID, ReceiptPath, ReceiptDigest, VerifierPath string
	Receipt                                                     json.RawMessage
	Verifier                                                    recovery.Result
}

func buildReplacementCampaignIndex(root string, manifestRaw json.RawMessage,
	finalPaths []string) (json.RawMessage, []string, error) {
	var manifest struct {
		Schema string
		Cells  []replacementCampaignCell
	}
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
		return nil, nil, fmt.Errorf("decode retained replacement campaign manifest: %w", err)
	}
	expected := 10
	if manifest.Schema == stressCampaignManifestSchema {
		expected = 3
	}
	if len(manifest.Cells) != expected || len(finalPaths) == 0 || len(finalPaths) > len(manifest.Cells) {
		return nil, nil, errors.New("replacement campaign final receipt cardinality is invalid")
	}
	index := replacementCampaignIndex{Schema: "ardents-qualification-campaign-index-v1",
		ManifestDigest: digestCampaignJSON(manifestRaw)}
	artifacts := make([]string, 0, len(manifest.Cells)*3)
	for cellIndex, cell := range manifest.Cells[:len(finalPaths)] {
		cellRoot := filepath.Join(root, "cells", cell.CellID)
		entries, err := os.ReadDir(cellRoot)
		if err != nil {
			return nil, nil, fmt.Errorf("read replacement cell receipt history %q: %w", cell.CellID, err)
		}
		for attemptIndex, entry := range entries {
			attemptID := fmt.Sprintf("attempt-%04d", attemptIndex+1)
			if !entry.IsDir() || entry.Name() != attemptID {
				return nil, nil, errors.New("replacement campaign receipt history is not contiguous")
			}
			attemptRoot := filepath.Join(cellRoot, attemptID)
			receiptPath := filepath.Join(attemptRoot, "receipt.json")
			raw, err := byteio.ReadFile(receiptPath, 5<<20)
			if err != nil {
				return nil, nil, fmt.Errorf("read durable replacement receipt: %w", err)
			}
			var receipt campaign.CellReceipt
			if err := decodeCampaignJSON(raw, &receipt); err != nil {
				return nil, nil, fmt.Errorf("decode durable replacement receipt: %w", err)
			}
			entryValue := replacementCampaignAttempt{CellID: receipt.CellID, AttemptID: receipt.AttemptID,
				ReceiptPath: relativeCampaignPath(root, receiptPath), ReceiptDigest: digestCampaignJSON(raw),
				Receipt: append(json.RawMessage(nil), raw...)}
			artifacts = append(artifacts, receiptPath)
			if receipt.Candidate == "pass" || receipt.Candidate == "fail" {
				verifierPath := filepath.Join(attemptRoot, "verifier.json")
				verifierRaw, err := byteio.ReadFile(verifierPath, 64<<10)
				if err != nil {
					return nil, nil, fmt.Errorf("read replacement attempt verifier result: %w", err)
				}
				var result recovery.Result
				if err := decodeCampaignJSON(verifierRaw, &result); err != nil {
					return nil, nil, fmt.Errorf("decode replacement attempt verifier result: %w", err)
				}
				entryValue.VerifierPath, entryValue.Verifier = relativeCampaignPath(root, verifierPath), result
				artifacts = append(artifacts, verifierPath)
			}
			index.Attempts = append(index.Attempts, entryValue)
		}
		if len(entries) == 0 || filepath.Clean(finalPaths[cellIndex]) !=
			filepath.Join(cellRoot, fmt.Sprintf("attempt-%04d", len(entries)), "receipt.json") {
			return nil, nil, errors.New("replacement campaign final receipt does not match its immutable history")
		}
	}
	raw, err := json.Marshal(index)
	if err != nil {
		return nil, nil, fmt.Errorf("encode replacement campaign index: %w", err)
	}
	return raw, artifacts, nil
}

func digestCampaignJSON(raw []byte) string {
	var compact bytes.Buffer
	if json.Compact(&compact, raw) != nil {
		return digestPrerequisite(raw)
	}
	return digestPrerequisite(compact.Bytes())
}

func decodeCampaignJSON(raw []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("JSON contains multiple values")
	}
	return nil
}

func relativeCampaignPath(root, path string) string {
	value, err := filepath.Rel(root, path)
	if err != nil {
		return ""
	}
	return filepath.ToSlash(value)
}
