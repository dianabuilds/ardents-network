package routesmoke

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/epoch/assignment"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/qualification/epochfixture"
	qualification "github.com/dianabuilds/ardents-network/internal/qualification/route"
	"github.com/dianabuilds/ardents-network/internal/route"
)

type recoveryPrepared struct {
	base       qualification.Case
	manifest   [32]byte
	publisher  [32]byte
	candidates []route.Position
}

func buildRecoveryFixture(root string, now time.Time) (recoveryPrepared, error) {
	roles := []string{"initiator", "introduction", "rendezvous", "responder"}
	var network, epochSeed [32]byte
	for _, value := range [][]byte{network[:], epochSeed[:]} {
		if _, err := rand.Read(value); err != nil {
			return recoveryPrepared{}, err
		}
	}
	_, authority, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return recoveryPrepared{}, err
	}
	identities := make(map[[32]byte]identity, 12)
	inputs, accepted := make([][]byte, 0, 12), make([]epochfixture.Record, 0, 12)
	for roleIndex, role := range roles {
		for candidateIndex := range 3 {
			value, identityErr := newIdentity(now)
			if identityErr != nil {
				return recoveryPrepared{}, identityErr
			}
			nodeID := sha256.Sum256(append([]byte("ardents-h3-recovery-node-v1\x00"), value.public[:]...))
			family, familyErr := recoveryFamily(network, epochSeed, role, candidateIndex, roles)
			if familyErr != nil {
				return recoveryPrepared{}, familyErr
			}
			endpoint := fmt.Sprintf("172.31.20.%d:%d", 11+roleIndex+candidateIndex*10, 4601+roleIndex)
			record, recordErr := epochfixture.BuildRecord(epochfixture.RecordSpec{NetworkID: network,
				NodeID: nodeID, Generation: 1, ValidFrom: now.Add(-time.Minute), ValidUntil: now.Add(90 * time.Minute),
				Family: family, Endpoint: endpoint, Capability: 2, Capacity: 1, PrivateKey: value.private})
			if recordErr != nil {
				return recoveryPrepared{}, recordErr
			}
			identities[nodeID] = value
			inputs, accepted = append(inputs, record.Raw), append(accepted, record)
		}
	}
	epoch, err := epochfixture.BuildEpoch(epochfixture.EpochSpec{NetworkID: network, Number: 1,
		ValidFrom: now.Add(-time.Minute), ValidUntil: now.Add(90 * time.Minute), Inputs: inputs, Accepted: accepted,
		AssignmentSeed: epochSeed, Profile: "h3-route-tracer-v1", Domains: roles,
		Authorities: []ed25519.PrivateKey{authority}})
	if err != nil {
		return recoveryPrepared{}, err
	}
	public := authority.Public().(ed25519.PublicKey)
	opened, err := state.Open(state.Config{Root: filepath.Join(root, "state"), NetworkID: network,
		Authorities: map[[32]byte]ed25519.PublicKey{sha256.Sum256(public): public}, Threshold: 1,
		AcceptedProfile: "h3-route-tracer-v1", Now: now})
	if err != nil {
		return recoveryPrepared{}, err
	}
	if _, err = opened.Accept(context.Background(), epoch.Raw, epoch.Inputs, epoch.Materials); err != nil {
		return recoveryPrepared{}, errors.Join(err, opened.Close())
	}
	view, err := opened.Current()
	if closeErr := opened.Close(); err != nil || closeErr != nil {
		return recoveryPrepared{}, errors.Join(err, closeErr)
	}
	selectionSeed, plans, err := alignedRecoverySelection(view, network, epochSeed, now)
	if err != nil {
		return recoveryPrepared{}, err
	}
	publisher, err := newIdentity(now)
	if err != nil {
		return recoveryPrepared{}, err
	}
	client, err := newIdentity(now)
	if err != nil {
		return recoveryPrepared{}, err
	}
	publisherID := sha256.Sum256(append([]byte("ardents-h3-route-publisher-v1\x00"), publisher.public[:]...))
	base := recoveryQualificationCase(view, plans[0], selectionSeed, client, publisherID)
	manifest := qualification.Commit(base)
	if err := writeRecoveryFixtureFiles(root, now, public, plans, identities, client, publisher, publisherID, manifest); err != nil {
		return recoveryPrepared{}, err
	}
	candidates := make([]route.Position, 0, 12)
	for _, plan := range plans {
		candidates = append(candidates, plan.Positions...)
	}
	return recoveryPrepared{base: base, manifest: manifest, publisher: publisher.public, candidates: candidates}, nil
}

func alignedRecoverySelection(view state.Snapshot, network, epochSeed [32]byte,
	at time.Time) ([32]byte, []route.Plan, error) {
	return alignedRecoverySelectionWith(view, network, epochSeed, at, route.Select)
}

func alignedRecoverySelectionWith(view state.Snapshot, network, epochSeed [32]byte,
	at time.Time, selectRoute func(state.Snapshot, route.Selection) (route.Plan, error)) ([32]byte, []route.Plan, error) {
	base := sha256.Sum256(append(append([]byte("ardents-h3-recovery-selection-v1\x00"), network[:]...), epochSeed[:]...))
	for attempt := uint64(0); attempt < 100_000; attempt++ {
		input := append([]byte(nil), base[:]...)
		counter := make([]byte, 8)
		binary.BigEndian.PutUint64(counter, attempt)
		seed := sha256.Sum256(append(input, counter...))
		plans := make([]route.Plan, 0, 3)
		excluded := make([][32]byte, 0, 12)
		aligned := true
		for generation := range 3 {
			plan, err := selectRoute(view, route.Selection{Seed: seed, At: at, ExcludedIdentities: excluded})
			if err != nil {
				return [32]byte{}, nil, fmt.Errorf("select recovery generation %d during alignment attempt %d: %w",
					generation, attempt, err)
			}
			if !recoveryPlanAligned(plan, generation) {
				aligned = false
				break
			}
			plans = append(plans, plan)
			for _, position := range plan.Positions {
				excluded = append(excluded, position.NodeID)
			}
		}
		if aligned {
			return seed, plans, nil
		}
	}
	return [32]byte{}, nil, errors.New("cannot align finite recovery candidates with the frozen Compose topology")
}

func recoveryPlanAligned(plan route.Plan, generation int) bool {
	if len(plan.Positions) != 4 {
		return false
	}
	for index, position := range plan.Positions {
		want := fmt.Sprintf("172.31.20.%d:%d", 11+index+generation*10, 4601+index)
		if position.Endpoint != want {
			return false
		}
	}
	return true
}

func recoveryFamily(network, seed [32]byte, role string, candidate int, domains []string) (string, error) {
	for index := range 10_000 {
		family := fmt.Sprintf("h3r-%s-%d-%04x", role, candidate+1, index)
		selected, err := assignment.Select(network, 1, seed, family, domains)
		if err != nil {
			return "", err
		}
		if selected == role {
			return family, nil
		}
	}
	return "", fmt.Errorf("cannot derive recovery Route family for %s", role)
}

func recoveryQualificationCase(view state.Snapshot, plan route.Plan, selectionSeed [32]byte,
	client identity, publisherID [32]byte) qualification.Case {
	base := qualification.Case{NetworkID: plan.NetworkID, Generation: plan.Generation, Epoch: plan.Epoch,
		EpochDigest: plan.Digest, Profile: plan.Profile, ViewRoot: plan.ViewRoot, SelectionSeed: selectionSeed,
		SelectionAt: plan.SelectionAt, ClientPin: client.public, PublisherID: publisherID}
	for index := 0; index < int(view.CandidateCount); index++ {
		candidate := view.Candidates[index]
		base.Candidates = append(base.Candidates, qualification.Candidate{NodeID: candidate.NodeID,
			PublicKey: candidate.PublicKey, Family: candidate.Family, Endpoint: candidate.Endpoint,
			Domain: candidate.Domain, Capacity: candidate.Capacity, ValidFrom: candidate.ValidFrom.Unix(),
			ValidUntil: candidate.ValidUntil.Unix()})
	}
	for index, position := range plan.Positions {
		base.NodeIDs[index], base.PublicKeys[index], base.Families[index], base.Endpoints[index] =
			position.NodeID, position.PublicKey, position.Family, position.Endpoint
	}
	return base
}
