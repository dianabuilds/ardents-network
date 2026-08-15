package campaign

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const maximumCellReceipt = (4 << 20) + (64 << 10)

func writeCellReceipt(root string, receipt CellReceipt) (returnErr error) {
	cellRoot := filepath.Join(root, "cells", receipt.CellID)
	if err := os.MkdirAll(cellRoot, 0o700); err != nil {
		return fmt.Errorf("create qualification cell receipt root: %w", err)
	}
	attemptRoot := filepath.Join(cellRoot, receipt.AttemptID)
	if err := os.Mkdir(attemptRoot, 0o700); err != nil {
		return fmt.Errorf("reserve immutable qualification attempt: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			var pendingErr, rootErr error
			if err := os.Remove(pendingReceipt(attemptRoot)); err != nil && !errors.Is(err, os.ErrNotExist) {
				pendingErr = fmt.Errorf("remove pending qualification cell receipt: %w", err)
			}
			if err := os.Remove(attemptRoot); err != nil && !errors.Is(err, os.ErrNotExist) {
				rootErr = fmt.Errorf("remove failed qualification attempt reservation: %w", err)
			}
			returnErr = errors.Join(returnErr, pendingErr, rootErr)
		}
	}()
	if err := syncDirectory(cellRoot); err != nil {
		return fmt.Errorf("sync qualification attempt reservation: %w", err)
	}
	raw, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return fmt.Errorf("encode qualification cell receipt: %w", err)
	}
	raw = append(raw, '\n')
	if len(raw) > maximumCellReceipt {
		return errors.New("qualification cell receipt exceeds its byte bound")
	}
	pending := pendingReceipt(attemptRoot)
	final := filepath.Join(attemptRoot, "receipt.json")
	file, err := os.OpenFile(pending, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create pending qualification cell receipt: %w", err)
	}
	if _, err := file.Write(raw); err != nil {
		return errors.Join(fmt.Errorf("write pending qualification cell receipt: %w", err),
			closeReceiptFile(file))
	}
	if err := file.Sync(); err != nil {
		return errors.Join(fmt.Errorf("sync pending qualification cell receipt: %w", err),
			closeReceiptFile(file))
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close pending qualification cell receipt: %w", err)
	}
	if err := publishReceipt(pending, final, attemptRoot); err != nil {
		return fmt.Errorf("publish immutable qualification cell receipt: %w", err)
	}
	committed = true
	return nil
}

func closeReceiptFile(file *os.File) error {
	if err := file.Close(); err != nil {
		return fmt.Errorf("close pending qualification cell receipt: %w", err)
	}
	return nil
}

func pendingReceipt(attemptRoot string) string {
	return filepath.Join(attemptRoot, "receipt.json.pending")
}

func safeIdentifier(value string) bool {
	if value == "" || len(value) > 96 || value[0] == '-' || value[len(value)-1] == '-' {
		return false
	}
	for _, character := range value {
		if !(character >= 'a' && character <= 'z' || character >= '0' && character <= '9' || character == '-') {
			return false
		}
	}
	return true
}

func hexDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	return strings.IndexFunc(value, func(character rune) bool {
		return !(character >= '0' && character <= '9' || character >= 'a' && character <= 'f')
	}) == -1
}
