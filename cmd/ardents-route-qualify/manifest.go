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
	Evidence       string   `json:"evidence"`
	EvidenceDigest string   `json:"evidence_digest"`
	NetworkID      string   `json:"network_id"`
	Generation     string   `json:"generation"`
	Epoch          uint64   `json:"epoch"`
	EpochDigest    string   `json:"epoch_digest"`
	NodeIDs        []string `json:"node_ids"`
	PublicKeys     []string `json:"public_keys"`
	Families       []string `json:"families"`
	ClientPin      string   `json:"client_pin"`
	PublisherID    string   `json:"publisher_id"`
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
	if len(value.NodeIDs) != 4 || len(value.PublicKeys) != 4 || len(value.Families) != 4 {
		return qualification.Case{}, errors.New("route evidence manifest requires four positions")
	}
	input := qualification.Case{Generation: value.Generation, Epoch: value.Epoch, Families: [4]string(value.Families)}
	for _, field := range []struct {
		encoded     string
		destination []byte
	}{{value.EvidenceDigest, input.EvidenceDigest[:]}, {value.NetworkID, input.NetworkID[:]},
		{value.EpochDigest, input.EpochDigest[:]}, {value.ClientPin, input.ClientPin[:]},
		{value.PublisherID, input.PublisherID[:]}} {
		if err := fixedHex(field.encoded, field.destination); err != nil {
			return qualification.Case{}, err
		}
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
