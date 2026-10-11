package durable

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/publication"
)

// Publish persists only this original reservation's exact verified public
// record. It grants no live registration, Store acknowledgement or readiness.
// The floor must already be consumed; a failed record never refunds it.
func (generation *Generation) Publish(ctx context.Context, raw []byte) (result error) {
	if generation == nil || generation.root == nil || ctx == nil {
		return errors.New("publication generation unavailable")
	}
	if len(raw) > 512 {
		return errors.New("publication public record exceeds bound")
	}
	raw = bytes.Clone(raw)
	root := generation.root
	root.mu.Lock()
	defer root.mu.Unlock()
	if err := generation.check(ctx); err != nil {
		return err
	}
	proof, err := publication.VerifyPublish(raw, root.config.Target, root.config.Network, time.Now())
	if err != nil {
		return err
	}
	const credentialStart = len("ardents-service-publication-v3\x00")
	if sha256.Sum256(raw[credentialStart:credentialStart+222]) != generation.credential.Digest() || !generation.advanced {
		return errors.Join(errors.New("publication exact consumed record required"), err)
	}
	if len(generation.record) != 0 {
		if !bytes.Equal(raw, generation.record) {
			return errors.New("publication immutable record changed")
		}
		err := root.checkRecord(raw)
		root.failure = errors.Join(root.failure, err)
		return err
	}
	defer func() {
		if result != nil {
			root.failure = errors.Join(root.failure, result)
		}
	}()
	name := fmt.Sprintf("%016x", root.floor)
	parent := filepath.Join(root.path, "generations")
	path := filepath.Join(parent, name)
	stage := filepath.Join(parent, ".publication-staging-"+name)
	if entries, err := scan(parent, 1); err != nil || len(entries) != 0 || root.current {
		return errors.Join(errors.New("publication generation inventory not empty"), err)
	}
	if err := os.Mkdir(stage, 0700); err != nil {
		return err
	}
	if err := root.flush(parent); err != nil {
		return err
	}
	if err := root.check(ctx); err != nil {
		return err
	}
	if err := exclusive(filepath.Join(stage, "publication.bin"), raw); err != nil {
		return err
	}
	if err := root.flush(stage); err != nil {
		return err
	}
	if err := root.check(ctx); err != nil {
		return err
	}
	if err := os.Rename(stage, path); err != nil {
		return err
	}
	if err := root.flush(parent); err != nil {
		return err
	}
	if err := root.replaceLeaf(ctx, "current", []byte(name+"\n")); err != nil {
		return err
	}
	if err := generation.check(ctx); err != nil {
		return err
	}
	generation.record = bytes.Clone(raw)
	root.current, root.predecessor = true, proof.Delegation()
	return nil
}

func (root *Root) checkRecord(raw []byte) error {
	name := fmt.Sprintf("%016x", root.floor)
	pointer, pointerErr := readRegular(filepath.Join(root.path, "current"), 17)
	record, recordErr := readRegular(filepath.Join(root.path, "generations", name, "publication.bin"), 512)
	if pointerErr != nil || recordErr != nil || string(pointer) != name+"\n" || !bytes.Equal(record, raw) || !root.current {
		return errors.Join(errors.New("publication original record unavailable"), pointerErr, recordErr)
	}
	return nil
}
