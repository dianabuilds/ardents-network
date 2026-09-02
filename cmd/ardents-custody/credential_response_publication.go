package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const maximumServiceCredentialResponseBytes = 1024

// publishStableCustodyPublicFile makes one immutable public Credential response
// visible only after a complete, flushed staging file exists. A repeated exact
// custody operation reopens an already-published response rather than replacing
// it.
func publishStableCustodyPublicFile(path string, body []byte) (resultErr error) {
	if len(body) == 0 || len(body) > maximumServiceCredentialResponseBytes {
		return errors.New("service Credential response is invalid")
	}
	destination, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve service Credential response destination: %w", err)
	}
	parent := filepath.Dir(destination)
	staging, err := os.CreateTemp(parent, ".ardents-service-credential-response-")
	if err != nil {
		return fmt.Errorf("create service Credential response staging file: %w", err)
	}
	stagingPath := staging.Name()
	stagingOpen := true
	defer func() {
		if stagingOpen {
			resultErr = errors.Join(resultErr, staging.Close())
		}
		if err := os.Remove(stagingPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			resultErr = errors.Join(resultErr, fmt.Errorf("remove service Credential response staging file: %w", err))
			return
		}
		if err := syncStableCustodyPublicDirectory(parent); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("flush service Credential response directory: %w", err))
		}
	}()
	if err := staging.Chmod(0o600); err != nil {
		return fmt.Errorf("protect service Credential response staging file: %w", err)
	}
	written, err := staging.Write(body)
	if err != nil {
		return fmt.Errorf("write service Credential response staging file: %w", err)
	}
	if written != len(body) {
		return fmt.Errorf("write service Credential response staging file: %w", io.ErrShortWrite)
	}
	if err := staging.Sync(); err != nil {
		return fmt.Errorf("flush service Credential response staging file: %w", err)
	}
	if err := staging.Close(); err != nil {
		return fmt.Errorf("close service Credential response staging file: %w", err)
	}
	stagingOpen = false
	if err := os.Link(stagingPath, destination); err != nil {
		if !errors.Is(err, os.ErrExist) {
			return fmt.Errorf("publish service Credential response: %w", err)
		}
	}
	existing, readErr := readStableCustodyPublicFile(destination)
	if readErr != nil || !bytes.Equal(existing, body) {
		return errors.New("service Credential response destination conflicts")
	}
	return nil
}

func readStableCustodyPublicFile(path string) ([]byte, error) {
	pathInfo, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !pathInfo.Mode().IsRegular() || pathInfo.Size() <= 0 || pathInfo.Size() > maximumServiceCredentialResponseBytes {
		return nil, errors.New("service Credential response is invalid")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	fileInfo, statErr := file.Stat()
	if statErr != nil || !fileInfo.Mode().IsRegular() || fileInfo.Size() <= 0 ||
		fileInfo.Size() > maximumServiceCredentialResponseBytes || !os.SameFile(pathInfo, fileInfo) {
		closeErr := file.Close()
		return nil, errors.Join(statErr, closeErr, errors.New("service Credential response is invalid"))
	}
	body, readErr := io.ReadAll(io.LimitReader(file, maximumServiceCredentialResponseBytes+1))
	closeErr := file.Close()
	if readErr != nil || closeErr != nil || len(body) == 0 || len(body) > maximumServiceCredentialResponseBytes {
		return nil, errors.Join(readErr, closeErr, errors.New("service Credential response is invalid"))
	}
	return body, nil
}
