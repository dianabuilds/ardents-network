//go:build linux

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/dianabuilds/ardents-network/internal/entry"
)

type routeMember struct {
	entry.ClosedSetMember
	subrole uint8
}

func selectRoute(root string, networkID [32]byte, domain uint8, members []routeMember, now time.Time) (entry.ClosedSetMember, entry.ClosedSetMember, error) {
	if err := os.MkdirAll(root, 0o700); err != nil {
		return entry.ClosedSetMember{}, entry.ClosedSetMember{}, err
	}
	view := entry.ClosedSetView{NetworkID: networkID, Now: now}
	for _, member := range members {
		if member.Domain == domain && member.subrole == 1 {
			view.Candidates = append(view.Candidates, member.ClosedSetMember)
		}
	}
	owner, err := entry.OpenClosedSets(entry.ClosedSetConfig{Root: root, NetworkID: networkID,
		Current: func() (entry.ClosedSetView, error) { return view, nil }})
	if err != nil {
		return entry.ClosedSetMember{}, entry.ClosedSetMember{}, err
	}
	pair, pairErr := owner.Members(domain)
	closeErr := owner.Close()
	if pairErr != nil || closeErr != nil {
		return entry.ClosedSetMember{}, entry.ClosedSetMember{}, errors.Join(pairErr, closeErr)
	}
	interiors := make([]entry.ClosedSetMember, 0, 2)
	for _, member := range members {
		if member.Domain == domain && member.subrole == 2 &&
			!memberConflict(pair[0], member.ClosedSetMember) && !memberConflict(pair[1], member.ClosedSetMember) {
			interiors = append(interiors, member.ClosedSetMember)
		}
	}
	slices.SortFunc(interiors, func(first, second entry.ClosedSetMember) int {
		return bytes.Compare(first.NodeID[:], second.NodeID[:])
	})
	var pairs [][2]entry.ClosedSetMember
	for _, first := range interiors {
		for _, second := range interiors {
			if !memberConflict(first, second) {
				pairs = append(pairs, [2]entry.ClosedSetMember{first, second})
			}
		}
	}
	if len(pairs) == 0 {
		return entry.ClosedSetMember{}, entry.ClosedSetMember{}, errors.New("fixture has no eligible Interior pair")
	}
	input := []byte{domain}
	for _, member := range pair {
		input = append(input, member.NodeID[:]...)
		input = append(input, member.PublicKey[:]...)
	}
	digest := sha256.Sum256(input)
	selected := pairs[binary.BigEndian.Uint64(digest[:8])%uint64(len(pairs))]
	return pair[0], selected[0], nil
}

func memberConflict(first, second entry.ClosedSetMember) bool {
	return first.NodeID == second.NodeID || first.PublicKey == second.PublicKey || first.FamilyID == second.FamilyID
}

func copyEntryRoot(source, destination string) error {
	if err := os.MkdirAll(destination, 0o700); err != nil {
		return err
	}
	entries, err := os.ReadDir(source)
	if err != nil {
		return err
	}
	for _, item := range entries {
		if item.IsDir() || item.Name() == ".ardents-entry-state-lock" {
			continue
		}
		body, err := os.ReadFile(filepath.Join(source, item.Name()))
		if err != nil {
			return err
		}
		if err := writePrivateKeyBytes(destination, item.Name(), body); err != nil {
			return err
		}
	}
	return nil
}
