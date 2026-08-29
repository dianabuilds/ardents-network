package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/dianabuilds/ardents-network/internal/node"
)

func nodeEventEmitter(output *os.File, directory string) func(context.Context, node.Event) error {
	base := node.EventEmitter(output)
	var lock sync.Mutex
	return func(ctx context.Context, event node.Event) error {
		lock.Lock()
		defer lock.Unlock()
		if err := base(ctx, event); err != nil || directory == "" {
			return err
		}
		name := ""
		switch event.Kind {
		case "lifecycle":
			name = "lifecycle.json"
		case "resource", "resource-sample":
			name = "resource.json"
		default:
			return nil
		}
		raw, err := json.Marshal(event)
		if err != nil {
			return err
		}
		return replaceDiagnostic(filepath.Join(directory, name), append(raw, '\n'))
	}
}

func replaceDiagnostic(path string, raw []byte) error {
	temporary := path + ".new"
	if err := os.Remove(temporary); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	file, err := os.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	written, writeErr := file.Write(raw)
	if writeErr == nil && written != len(raw) {
		writeErr = errors.New("short Node diagnostic write")
	}
	writeErr = errors.Join(writeErr, file.Sync(), file.Close())
	if writeErr != nil {
		_ = os.Remove(temporary)
		return writeErr
	}
	if err := os.Rename(temporary, path); err == nil {
		return nil
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		_ = os.Remove(temporary)
		return err
	}
	return os.Rename(temporary, path)
}
