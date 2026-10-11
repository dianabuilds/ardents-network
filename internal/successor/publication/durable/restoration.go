package durable

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/publication"
)

func (root *Root) restore(flush func(string) error) error {
	entries, err := scan(root.path, 5)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		switch entry.Name() {
		case markerName, lockName, "floor", "current":
			if !entry.Type().IsRegular() {
				return errors.New("publication root leaf invalid")
			}
		case "generations":
			if !entry.IsDir() {
				return errors.New("publication generations invalid")
			}
		default:
			return errors.New("publication root contains foreign residue")
		}
	}
	floor, exists, err := readFloor(root.path)
	if err != nil {
		return err
	}
	root.floor = floor
	if exists {
		if err = resyncFile(filepath.Join(root.path, "floor")); err != nil {
			return err
		}
		if err = flush(root.path); err != nil {
			return err
		}
	}
	generations := filepath.Join(root.path, "generations")
	entries, err = scan(generations, 1)
	if err != nil {
		return err
	}
	pointer, pointerErr := readRegular(filepath.Join(root.path, "current"), 17)
	hasPointer := pointerErr == nil
	if pointerErr != nil && !errors.Is(pointerErr, os.ErrNotExist) {
		return pointerErr
	}
	if !exists && (len(entries) != 0 || hasPointer) {
		return errors.New("publication history lacks floor")
	}
	if len(entries) == 0 {
		if hasPointer {
			return errors.New("publication pointer lacks record")
		}
		return nil
	}
	name := fmt.Sprintf("%016x", floor)
	stage := !hasPointer && entries[0].Name() == ".publication-staging-"+name
	if !entries[0].IsDir() || (entries[0].Name() != name && !stage) || (hasPointer && string(pointer) != name+"\n") {
		return errors.New("publication generation and pointer disagree with floor")
	}
	path := filepath.Join(generations, entries[0].Name())
	leaves, err := scan(path, 1)
	if err == nil && len(leaves) == 0 && !hasPointer {
		// A canonical floor and exact empty generation directory can only be
		// retired as unavailable residue. No signature or readiness is restored.
		if err = flush(path); err != nil {
			return err
		}
		if err = os.Remove(path); err != nil {
			return err
		}
		return flush(generations)
	}
	if err != nil || len(leaves) != 1 || leaves[0].Name() != "publication.bin" {
		return errors.Join(errors.New("publication immutable record inventory invalid"), err)
	}
	raw, err := readRegular(filepath.Join(path, "publication.bin"), 512)
	// Retained evidence is checked at its signed NotBefore, not considered live.
	const domain = "ardents-service-publication-v3\x00"
	if err != nil || len(raw) < len(domain)+122 {
		return errors.Join(errors.New("publication record invalid"), err)
	}
	before := binary.BigEndian.Uint64(raw[len(domain)+106 : len(domain)+114])
	if before > 1<<63-1 {
		return errors.New("publication retained time invalid")
	}
	proof, err := publication.VerifyPublish(raw, root.config.Target, root.config.Network, time.Unix(int64(before), 0))
	if err != nil || proof.Delegation().Generation != floor {
		return errors.Join(errors.New("publication retained proof invalid"), err)
	}
	root.predecessor = proof.Delegation()
	if !hasPointer {
		// An exact committed orphan is unavailable, never restored as readiness.
		if err = os.Remove(filepath.Join(path, "publication.bin")); err != nil {
			return err
		}
		if err = flush(path); err != nil {
			return err
		}
		if err = os.Remove(path); err != nil {
			return err
		}
		return flush(generations)
	}
	if err = resyncFile(filepath.Join(path, "publication.bin")); err != nil {
		return err
	}
	for _, directory := range []string{path, generations} {
		if err = flush(directory); err != nil {
			return err
		}
	}
	if err = resyncFile(filepath.Join(root.path, "current")); err != nil {
		return err
	}
	if err = flush(root.path); err != nil {
		return err
	}
	root.current = true
	return nil
}

func readFloor(path string) (uint64, bool, error) {
	raw, err := readRegular(filepath.Join(path, "floor"), 21)
	if errors.Is(err, os.ErrNotExist) {
		return 0, false, nil
	}
	if err != nil || len(raw) == 0 || raw[len(raw)-1] != '\n' {
		return 0, false, errors.Join(errors.New("publication floor malformed"), err)
	}
	value, err := strconv.ParseUint(string(raw[:len(raw)-1]), 10, 64)
	if err != nil || value == 0 || string(raw) != strconv.FormatUint(value, 10)+"\n" {
		return 0, false, errors.New("publication floor malformed")
	}
	return value, true, nil
}
