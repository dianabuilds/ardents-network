//go:build ignore

package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Fixtures is one closed-alpha State fixture bundle produced by Prebake.
// It contains the network identity, the alpha corpus authority keypair, the
// canonical epoch bytes, the matching materialization, and the on-disk root
// directory layout that the source server plans and the source client plan
// reference. It is the single source of truth for everything else the pilot
// writes into the evidence directory.
type Fixtures struct {
	NetworkID         [32]byte
	NetworkIDHex      string
	AuthorityPublic   ed25519.PublicKey
	AuthorityID       [32]byte
	AuthorityPrivate  ed25519.PrivateKey
	Generation        string
	EpochNumber       uint64
	EpochDigest       [32]byte
	EpochRaw          []byte
	Materialization   []byte
	Inputs            [][]byte
	Records           []Record
	RootDir           string
	LocalRoleStateDir string
	Now               time.Time
}

// Prebake builds one fresh State fixture bundle under root and returns the
// in-memory summary. The fixture is sized for the closed-alpha pilot only:
// one authority, one epoch, six consumer node records, no rejected inputs.
// It is intentionally not a public or hostile fixture.
func Prebake(root string, now time.Time) (Fixtures, error) {
	if root == "" {
		return Fixtures{}, errors.New("pilot: prebake root is empty")
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return Fixtures{}, fmt.Errorf("pilot: prebake root: %w", err)
	}
	network := sha256.Sum256([]byte("ardents-pilot-multi-node-2026-09-04"))
	authoritySeed := bytes.Repeat([]byte{0xa9}, ed25519.SeedSize)
	authorityPrivate := ed25519.NewKeyFromSeed(authoritySeed)
	authorityPublic := authorityPrivate.Public().(ed25519.PublicKey)
	authorityID := sha256.Sum256(authorityPublic)

	records := make([]Record, 0, 6)
	rawInputs := make([][]byte, 0, 6)
	for index := 0; index < 6; index++ {
		marker := byte(0x21 + index)
		family := fmt.Sprintf("pilot-family-%d", index)
		endpoint := fmt.Sprintf("node-%d:4101", index+1) // never dialed, identifies the record's owner
		nodeSeed := bytes.Repeat([]byte{marker}, ed25519.SeedSize)
		nodePrivate := ed25519.NewKeyFromSeed(nodeSeed)
		nodeID := sha256.Sum256([]byte{0x4e, marker})
		record, err := BuildRecord(network, nodeID, 1,
			now.Add(-30*time.Second).Unix(), now.Add(30*time.Minute).Unix(),
			family, endpoint, 1, 8, nodePrivate)
		if err != nil {
			return Fixtures{}, fmt.Errorf("pilot: build record %d: %w", index, err)
		}
		records = append(records, record)
		rawInputs = append(rawInputs, record.Raw)
	}
	sort.Slice(records, func(i, j int) bool {
		return bytes.Compare(records[i].NodeID[:], records[j].NodeID[:]) < 0
	})

	epoch, err := BuildEpoch(network, 1, [32]byte{},
		now.Add(-30*time.Second).Unix(), now.Add(30*time.Minute).Unix(),
		rawInputs, records, map[uint32]uint16{},
		sha256.Sum256([]byte("pilot-assignment-seed-1")),
		[]string{"alpha"}, []ed25519.PrivateKey{authorityPrivate})
	if err != nil {
		return Fixtures{}, fmt.Errorf("pilot: build epoch: %w", err)
	}
	if len(epoch.Materials) == 0 {
		return Fixtures{}, errors.New("pilot: built epoch has no materialization")
	}
	materialization := epoch.Materials[0]
	generation := fmt.Sprintf("%x", epoch.Digest)
	generationDir := filepath.Join(root, "generations", generation, "inputs")
	if err := os.MkdirAll(generationDir, 0o700); err != nil {
		return Fixtures{}, fmt.Errorf("pilot: mkdir generation: %w", err)
	}
	if err := os.WriteFile(filepath.Join(root, "current"), []byte(generation+"\n"), 0o600); err != nil {
		return Fixtures{}, fmt.Errorf("pilot: write current: %w", err)
	}
	if err := os.WriteFile(filepath.Join(root, "generations", generation, "epoch.bin"), epoch.Raw, 0o600); err != nil {
		return Fixtures{}, fmt.Errorf("pilot: write epoch: %w", err)
	}
	for index, raw := range rawInputs {
		name := filepath.Join(root, "generations", generation, "inputs", fmt.Sprintf("%04d.bin", index))
		if err := os.WriteFile(name, raw, 0o600); err != nil {
			return Fixtures{}, fmt.Errorf("pilot: write input %d: %w", index, err)
		}
	}
	localRoleDir := filepath.Join(root, "local-roles")
	if err := os.MkdirAll(localRoleDir, 0o700); err != nil {
		return Fixtures{}, fmt.Errorf("pilot: mkdir local roles: %w", err)
	}
	return Fixtures{
		NetworkID:         network,
		NetworkIDHex:      fmt.Sprintf("%x", network[:]),
		AuthorityPublic:   authorityPublic,
		AuthorityID:       authorityID,
		AuthorityPrivate:  authorityPrivate,
		Generation:        generation,
		EpochNumber:       epoch.Number,
		EpochDigest:       epoch.Digest,
		EpochRaw:          epoch.Raw,
		Materialization:   materialization,
		Inputs:            rawInputs,
		Records:           records,
		RootDir:           root,
		LocalRoleStateDir: localRoleDir,
		Now:               now,
	}, nil
}
