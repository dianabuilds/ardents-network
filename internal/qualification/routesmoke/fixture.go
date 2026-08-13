package routesmoke

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/epoch/assignment"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/qualification/byteio"
	"github.com/dianabuilds/ardents-network/internal/qualification/epochfixture"
	qualification "github.com/dianabuilds/ardents-network/internal/qualification/route"
	"github.com/dianabuilds/ardents-network/internal/route"
)

type prepared struct {
	base     qualification.Case
	manifest [32]byte
}

const (
	fixtureOwner  = "ardents-h3-route-smoke-fixture-v1\n"
	evidenceOwner = "ardents-h3-route-smoke-evidence-v1\n"
)

func prepare(input Config) (prepared, error) {
	if input.Duration < 10*time.Minute || input.Duration > 30*time.Minute || !absoluteNew(input.FixtureRoot) ||
		!absoluteNew(input.EvidenceRoot) || input.ComposeFile == "" || !filepath.IsAbs(input.ComposeFile) ||
		input.SourceRoot == "" || !filepath.IsAbs(input.SourceRoot) {
		return prepared{}, errors.New("route smoke evidence root, fixture root, Compose file, source root, and 10m..30m duration are required")
	}
	if err := validateRoots(input); err != nil {
		return prepared{}, err
	}
	if err := os.Mkdir(input.FixtureRoot, 0o700); err != nil {
		return prepared{}, err
	}
	if err := os.WriteFile(filepath.Join(input.FixtureRoot, ".ardents-route-smoke-owned"), []byte(fixtureOwner), 0o600); err != nil {
		return prepared{}, err
	}
	if err := os.Mkdir(input.EvidenceRoot, 0o700); err != nil {
		return prepared{}, err
	}
	if err := os.WriteFile(filepath.Join(input.EvidenceRoot, ".ardents-route-smoke-evidence"),
		[]byte(evidenceOwner), 0o600); err != nil {
		return prepared{}, err
	}
	for _, path := range []string{"plans", "state"} {
		if err := os.MkdirAll(filepath.Join(input.FixtureRoot, path), 0o700); err != nil {
			return prepared{}, err
		}
	}
	for _, role := range []string{"client", "initiator", "introduction", "rendezvous", "responder", "publisher"} {
		if err := os.MkdirAll(filepath.Join(input.FixtureRoot, "secrets", role), 0o700); err != nil {
			return prepared{}, err
		}
	}
	fixture, err := buildFixture(input.FixtureRoot, time.Now().UTC().Truncate(time.Second))
	return fixture, err
}

func absoluteNew(path string) bool {
	if path == "" || !filepath.IsAbs(path) {
		return false
	}
	_, err := os.Stat(path)
	return errors.Is(err, os.ErrNotExist)
}

func removeFixture(root string) error {
	if root == "" {
		return nil
	}
	if !filepath.IsAbs(root) {
		return errors.New("route smoke fixture cleanup requires an absolute path")
	}
	marker, err := os.ReadFile(filepath.Join(root, ".ardents-route-smoke-owned"))
	if errors.Is(err, os.ErrNotExist) {
		if _, statErr := os.Stat(root); errors.Is(statErr, os.ErrNotExist) {
			return nil
		}
	}
	if err != nil || string(marker) != fixtureOwner {
		return errors.New("route smoke fixture cleanup ownership is invalid")
	}
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return os.Remove(path)
		}
		if entry.IsDir() {
			return os.Chmod(path, 0o700)
		}
		return os.Chmod(path, 0o600)
	}); err != nil {
		return err
	}
	if err := os.RemoveAll(root); err != nil {
		return err
	}
	if _, err := os.Stat(root); !errors.Is(err, os.ErrNotExist) {
		return errors.New("route smoke fixture remains after cleanup")
	}
	return nil
}

func ownsFixture(root string) bool {
	marker, err := os.ReadFile(filepath.Join(root, ".ardents-route-smoke-owned"))
	return err == nil && string(marker) == fixtureOwner
}

func buildFixture(root string, now time.Time) (prepared, error) {
	roles := []string{"initiator", "introduction", "rendezvous", "responder"}
	addresses := []string{"172.31.20.11:4601", "172.31.20.12:4602", "172.31.20.13:4603", "172.31.20.14:4604"}
	identities := make([]identity, 6)
	for index := range identities {
		value, err := newIdentity(now)
		if err != nil {
			return prepared{}, err
		}
		identities[index] = value
	}
	var network, seed, selectionSeed [32]byte
	for _, value := range [][]byte{network[:], seed[:], selectionSeed[:]} {
		if _, err := rand.Read(value); err != nil {
			return prepared{}, err
		}
	}
	_, authority, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return prepared{}, err
	}
	inputs, accepted := make([][]byte, 0, 4), make([]epochfixture.Record, 0, 4)
	for index, role := range roles {
		family, err := assignedFamily(network, seed, role, roles)
		if err != nil {
			return prepared{}, err
		}
		nodeID := sha256.Sum256(append([]byte("ardents-h3-route-node-v1\x00"), identities[index].public[:]...))
		record, err := epochfixture.BuildRecord(epochfixture.RecordSpec{NetworkID: network, NodeID: nodeID, Generation: 1,
			ValidFrom: now.Add(-time.Minute), ValidUntil: now.Add(90 * time.Minute), Family: family, Endpoint: addresses[index],
			Capability: 2, Capacity: 1, PrivateKey: identities[index].private})
		if err != nil {
			return prepared{}, err
		}
		inputs, accepted = append(inputs, record.Raw), append(accepted, record)
	}
	epoch, err := epochfixture.BuildEpoch(epochfixture.EpochSpec{NetworkID: network, Number: 1,
		ValidFrom: now.Add(-time.Minute), ValidUntil: now.Add(90 * time.Minute), Inputs: inputs, Accepted: accepted,
		AssignmentSeed: seed, Profile: "h3-route-tracer-v1", Domains: roles, Authorities: []ed25519.PrivateKey{authority}})
	if err != nil {
		return prepared{}, err
	}
	public := authority.Public().(ed25519.PublicKey)
	opened, err := state.Open(state.Config{Root: filepath.Join(root, "state"), NetworkID: network,
		Authorities: map[[32]byte]ed25519.PublicKey{sha256.Sum256(public): public}, Threshold: 1,
		AcceptedProfile: "h3-route-tracer-v1", Now: now})
	if err != nil {
		return prepared{}, err
	}
	if _, err = opened.Accept(context.Background(), epoch.Raw, epoch.Inputs, epoch.Materials); err != nil {
		opened.Close()
		return prepared{}, err
	}
	view, err := opened.Current()
	_ = opened.Close()
	if err != nil {
		return prepared{}, err
	}
	plan, err := route.Select(view, route.Selection{Seed: selectionSeed, At: now})
	if err != nil {
		return prepared{}, err
	}
	publisherID := sha256.Sum256(append([]byte("ardents-h3-route-publisher-v1\x00"), identities[4].public[:]...))
	base := qualification.Case{NetworkID: network, Generation: plan.Generation, Epoch: plan.Epoch, EpochDigest: plan.Digest,
		Profile: plan.Profile, ViewRoot: plan.ViewRoot, SelectionSeed: selectionSeed, SelectionAt: plan.SelectionAt,
		ClientPin: identities[5].public, PublisherID: publisherID}
	for index, position := range plan.Positions {
		candidate := view.Candidates[index]
		base.Candidates = append(base.Candidates, qualification.Candidate{NodeID: candidate.NodeID, PublicKey: candidate.PublicKey,
			Family: candidate.Family, Endpoint: candidate.Endpoint, Domain: candidate.Domain, Capacity: candidate.Capacity,
			ValidFrom: candidate.ValidFrom.Unix(), ValidUntil: candidate.ValidUntil.Unix()})
		base.NodeIDs[index], base.PublicKeys[index], base.Families[index], base.Endpoints[index] = position.NodeID, position.PublicKey, position.Family, position.Endpoint
	}
	manifest := qualification.Commit(base)
	if err := writeFixtureFiles(root, now, public, identities, plan, publisherID, manifest); err != nil {
		return prepared{}, err
	}
	if err := byteio.WriteJSON(filepath.Join(root, "manifest.json"), map[string]any{"schema": "ardents-h3-route-smoke-fixture-v1",
		"created_at": now, "network_id": hex.EncodeToString(network[:]), "manifest_digest": hex.EncodeToString(manifest[:])}, 64<<10); err != nil {
		return prepared{}, err
	}
	return prepared{base: base, manifest: manifest}, nil
}

func assignedFamily(network, seed [32]byte, wanted string, domains []string) (string, error) {
	for index := range 10_000 {
		family := fmt.Sprintf("%s-family-%d", wanted, index)
		selected, err := assignment.Select(network, 1, seed, family, domains)
		if err != nil {
			return "", err
		}
		if selected == wanted {
			return family, nil
		}
	}
	return "", errors.New("cannot derive Route family")
}
