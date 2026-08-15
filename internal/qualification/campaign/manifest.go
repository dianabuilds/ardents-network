package campaign

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const maximumCampaignManifest = 2 << 20

// PublishManifest durably creates one immutable campaign manifest.
func PublishManifest(root string, raw json.RawMessage) (returnErr error) {
	if root == "" || len(raw) == 0 || len(raw) > maximumCampaignManifest || !json.Valid(raw) {
		return errors.New("qualification campaign manifest is invalid")
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return fmt.Errorf("create qualification campaign root: %w", err)
	}
	pending := filepath.Join(root, "campaign-manifest.json.pending")
	final := filepath.Join(root, "campaign-manifest.json")
	if existing, err := readCampaignManifest(final); err == nil {
		if string(existing) == string(raw) {
			return nil
		}
		return errors.New("qualification campaign manifest is immutable")
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect qualification campaign manifest: %w", err)
	}
	file, err := os.OpenFile(pending, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create pending qualification campaign manifest: %w", err)
	}
	published := false
	defer func() {
		if !published {
			var closeErr error
			if file != nil {
				closeErr = file.Close()
			}
			returnErr = errors.Join(returnErr, closeErr, removePendingManifest(pending))
		}
	}()
	if _, err := file.Write(raw); err != nil {
		return fmt.Errorf("write qualification campaign manifest: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync qualification campaign manifest: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close qualification campaign manifest: %w", err)
	}
	file = nil
	if err := publishManifest(pending, final, root); err != nil {
		return fmt.Errorf("publish qualification campaign manifest: %w", err)
	}
	published = true
	return nil
}

func readCampaignManifest(path string) (raw []byte, returnErr error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { returnErr = errors.Join(returnErr, file.Close()) }()
	raw, err = io.ReadAll(io.LimitReader(file, maximumCampaignManifest+1))
	if err != nil || len(raw) > maximumCampaignManifest {
		return nil, errors.Join(err, errors.New("existing campaign manifest exceeds its byte bound"))
	}
	return raw, nil
}

func removePendingManifest(path string) error {
	err := os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
