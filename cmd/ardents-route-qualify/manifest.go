package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"

	qualification "github.com/dianabuilds/ardents-network/internal/qualification/route"
)

type manifest struct {
	Evidence           string              `json:"evidence"`
	EvidenceDigest     string              `json:"evidence_digest"`
	ManifestDigest     string              `json:"manifest_digest"`
	NetworkID          string              `json:"network_id"`
	Generation         string              `json:"generation"`
	Epoch              uint64              `json:"epoch"`
	EpochDigest        string              `json:"epoch_digest"`
	Profile            string              `json:"profile"`
	ViewRoot           string              `json:"view_root"`
	SelectionSeed      string              `json:"selection_seed"`
	SelectionAt        int64               `json:"selection_at"`
	Candidates         []manifestCandidate `json:"candidates"`
	ExcludedIdentities []string            `json:"excluded_identities"`
	ExcludedFamilies   []string            `json:"excluded_families"`
	ExcludedDomains    []string            `json:"excluded_domains"`
	NodeIDs            []string            `json:"node_ids"`
	PublicKeys         []string            `json:"public_keys"`
	Families           []string            `json:"families"`
	Endpoints          []string            `json:"endpoints"`
	ClientPin          string              `json:"client_pin"`
	PublisherID        string              `json:"publisher_id"`
	SourceID           string              `json:"source_id"`
	BuildDigest        string              `json:"build_digest"`
	ExitedPIDs         []int               `json:"exited_pids"`
	ExitedRuntimeIDs   []string            `json:"exited_runtime_ids"`
	ContainerIDs       []string            `json:"container_ids"`
	CleanupVerified    bool                `json:"cleanup_verified"`
}

func readCase(path string) (qualification.Case, error) {
	raw, err := boundedFile(path, 64<<10)
	if err != nil {
		return qualification.Case{}, err
	}
	var value manifest
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return qualification.Case{}, err
	}
	if len(value.NodeIDs) != 4 || len(value.PublicKeys) != 4 || len(value.Families) != 4 ||
		len(value.Endpoints) != 4 || len(value.Candidates) == 0 || len(value.Candidates) > 64 ||
		len(value.ExitedPIDs) != 6 || len(value.ExitedRuntimeIDs) != 6 || len(value.ContainerIDs) != 6 {
		return qualification.Case{}, errors.New("route evidence manifest requires four positions")
	}
	input := qualification.Case{Generation: value.Generation, Epoch: value.Epoch, Profile: value.Profile,
		SelectionAt: value.SelectionAt, Families: [4]string(value.Families), Endpoints: [4]string(value.Endpoints),
		ExcludedFamilies: value.ExcludedFamilies, ExcludedDomains: value.ExcludedDomains, SourceID: value.SourceID,
		ExitedPIDs: [6]int(value.ExitedPIDs), ExitedRuntimeIDs: [6]string(value.ExitedRuntimeIDs),
		ContainerIDs: [6]string(value.ContainerIDs), CleanupVerified: value.CleanupVerified}
	for _, field := range []struct {
		encoded     string
		destination []byte
	}{{value.EvidenceDigest, input.EvidenceDigest[:]}, {value.ManifestDigest, input.ManifestDigest[:]},
		{value.NetworkID, input.NetworkID[:]}, {value.EpochDigest, input.EpochDigest[:]},
		{value.ViewRoot, input.ViewRoot[:]}, {value.SelectionSeed, input.SelectionSeed[:]},
		{value.ClientPin, input.ClientPin[:]}, {value.BuildDigest, input.BuildDigest[:]},
		{value.PublisherID, input.PublisherID[:]}} {
		if err := fixedHex(field.encoded, field.destination); err != nil {
			return qualification.Case{}, err
		}
	}
	if err := resolveCandidates(&input, value); err != nil {
		return qualification.Case{}, err
	}
	for index := range 4 {
		if err := fixedHex(value.NodeIDs[index], input.NodeIDs[index][:]); err != nil {
			return qualification.Case{}, err
		}
		if err := fixedHex(value.PublicKeys[index], input.PublicKeys[index][:]); err != nil {
			return qualification.Case{}, err
		}
	}
	input.RawEvidence, err = boundedFile(value.Evidence, 256<<10)
	return input, err
}

func boundedFile(path string, maximum int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, maximum+1))
	if err != nil || len(raw) == 0 || int64(len(raw)) > maximum {
		return nil, errors.New("evidence input is empty or outside its bound")
	}
	return raw, nil
}

func fixedHex(encoded string, destination []byte) error {
	if len(encoded) != len(destination)*2 {
		return errors.New("manifest hexadecimal field has wrong length")
	}
	_, err := hex.Decode(destination, []byte(encoded))
	return err
}
