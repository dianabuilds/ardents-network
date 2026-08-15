package campaign

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// NextAttempt returns either the next retryable attempt identity or the
// retained terminal candidate receipt. It never permits retry after candidate
// pass or fail.
func NextAttempt(root, cellID string) (string, *CellReceipt, error) {
	if root == "" || !safeIdentifier(cellID) {
		return "", nil, errors.New("qualification attempt location is invalid")
	}
	cellRoot := filepath.Join(root, "cells", cellID)
	entries, err := os.ReadDir(cellRoot)
	if errors.Is(err, os.ErrNotExist) {
		return "attempt-0001", nil, nil
	}
	if err != nil {
		return "", nil, fmt.Errorf("read qualification attempt history: %w", err)
	}
	for index, entry := range entries {
		expected := fmt.Sprintf("attempt-%04d", index+1)
		if !entry.IsDir() || entry.Name() != expected {
			return "", nil, errors.New("qualification attempt history is not contiguous and immutable")
		}
		receipt, err := readAttemptReceipt(filepath.Join(cellRoot, expected, "receipt.json"))
		if err != nil {
			return "", nil, fmt.Errorf("read %s qualification receipt: %w", expected, err)
		}
		if receipt.Schema != "ardents-qualification-cell-receipt-v1" || receipt.CellID != cellID ||
			receipt.AttemptID != expected {
			return "", nil, errors.New("qualification attempt receipt identity is inconsistent")
		}
		if receipt.Candidate == candidatePass || receipt.Candidate == candidateFail {
			return "", &receipt, nil
		}
		if receipt.Candidate != candidateNotRun || receipt.Observation != observationInvalid {
			return "", nil, errors.New("qualification attempt has an unsupported retry state")
		}
	}
	if len(entries) >= 9999 {
		return "", nil, errors.New("qualification attempt history exhausted its bound")
	}
	return fmt.Sprintf("attempt-%04d", len(entries)+1), nil, nil
}

func readAttemptReceipt(path string) (receipt CellReceipt, returnErr error) {
	file, err := os.Open(path)
	if err != nil {
		return CellReceipt{}, err
	}
	defer func() { returnErr = errors.Join(returnErr, file.Close()) }()
	raw, err := io.ReadAll(io.LimitReader(file, maximumCellReceipt+1))
	if err != nil || len(raw) == 0 || len(raw) > maximumCellReceipt {
		return CellReceipt{}, errors.Join(err, errors.New("qualification receipt is invalid or oversized"))
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&receipt); err != nil {
		return CellReceipt{}, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return CellReceipt{}, errors.New("qualification receipt contains multiple values")
	}
	return receipt, nil
}
