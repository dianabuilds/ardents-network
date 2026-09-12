package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
)

const maximumClosedProfilePlanBytes = 64 << 10

type closedProfilePlan struct {
	NetworkID, StateGeneration, EpochDigest, IssuerNodeID, IssuanceAuthorityKey string
	Epoch                                                                       uint64
	NotBefore, NotAfter                                                         string
	Nodes                                                                       []closedProfilePlanNode
	TokenKeys                                                                   []closedProfilePlanKey
}

type closedProfilePlanNode struct {
	NodeID, RecordDigest string
	RoleDomain, Subrole  uint8
	DutyGeneration       uint64
}

type closedProfilePlanKey struct {
	WindowStart string
	Class       uint8
	SPKI        string
}

func prepareClosedProfile(arguments []string, output io.Writer) error {
	flags := flag.NewFlagSet("prepare-closed-profile", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var planPath, destination string
	flags.StringVar(&planPath, "plan", "", "bounded public closed profile plan")
	flags.StringVar(&destination, "output", "", "new unsigned profile destination")
	if err := flags.Parse(arguments); err != nil || flags.NArg() != 0 || destination == "" {
		return errors.New("prepare closed profile arguments are invalid")
	}
	input, err := readClosedProfilePlan(planPath)
	if err != nil {
		return err
	}
	body, err := state.PrepareClosedProfile(input)
	if err != nil {
		return err
	}
	if err := writeNewClosedProfile(destination, body); err != nil {
		return err
	}
	return json.NewEncoder(output).Encode(closedProfileReport("ardents-closed-profile-prepared-v1", body))
}

func signClosedProfile(arguments []string, output io.Writer) error {
	flags := flag.NewFlagSet("sign-closed-profile", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var planPath, keyPath, destination string
	flags.StringVar(&planPath, "plan", "", "bounded public closed profile plan")
	flags.StringVar(&keyPath, "authority-key", "", "owner-only State authority PKCS#8 PEM")
	flags.StringVar(&destination, "output", "", "new signed profile destination")
	if err := flags.Parse(arguments); err != nil || flags.NArg() != 0 || destination == "" {
		return errors.New("sign closed profile arguments are invalid")
	}
	input, err := readClosedProfilePlan(planPath)
	if err != nil {
		return err
	}
	signer, err := readClosedProfileSigner(keyPath)
	if err != nil {
		return err
	}
	defer zeroPrivateKey(signer)
	raw, err := state.SignClosedProfile(input, signer)
	if err != nil {
		return err
	}
	if err := writeNewClosedProfile(destination, raw); err != nil {
		return err
	}
	return json.NewEncoder(output).Encode(closedProfileReport("ardents-closed-profile-signed-v1", raw))
}

func inspectClosedProfile(arguments []string, output io.Writer) error {
	flags := flag.NewFlagSet("inspect-closed-profile", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var planPath, profilePath, authorityText, atText string
	flags.StringVar(&planPath, "plan", "", "bounded public closed profile plan")
	flags.StringVar(&profilePath, "profile", "", "signed ARDCPR03 profile")
	flags.StringVar(&authorityText, "authority", "", "pinned State authority public key in lowercase hex")
	flags.StringVar(&atText, "at", "", "verification time in RFC3339")
	if err := flags.Parse(arguments); err != nil || flags.NArg() != 0 {
		return errors.New("inspect closed profile arguments are invalid")
	}
	input, err := readClosedProfilePlan(planPath)
	if err != nil {
		return err
	}
	authority, err := decodePublicKey(authorityText)
	if err != nil {
		return errors.New("closed profile authority is invalid")
	}
	at, err := time.Parse(time.RFC3339, atText)
	if err != nil {
		return errors.New("closed profile inspection time is invalid")
	}
	raw, err := readControlFile(profilePath, maximumClosedProfilePlanBytes)
	if err != nil {
		return err
	}
	view, err := state.InspectClosedProfile(raw, input.StateGeneration, input.NetworkID, input.EpochDigest, input.Epoch, authority, at.UTC())
	if err != nil {
		return errors.New("closed profile was not accepted")
	}
	return json.NewEncoder(output).Encode(struct {
		Schema, Digest, Authority string
		Epoch                     uint64
		NotBefore, NotAfter       string
	}{"ardents-closed-profile-inspection-v1", hex.EncodeToString(view.Digest[:]), hex.EncodeToString(view.IssuanceAuthorityKey[:]),
		view.Epoch, view.NotBefore.Format(time.RFC3339), view.NotAfter.Format(time.RFC3339)})
}

func readClosedProfilePlan(path string) (state.ClosedProfileInput, error) {
	raw, err := readControlFile(path, maximumClosedProfilePlanBytes)
	if err != nil {
		return state.ClosedProfileInput{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var plan closedProfilePlan
	if err := decoder.Decode(&plan); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return state.ClosedProfileInput{}, errors.New("closed profile plan is invalid")
	}
	input := state.ClosedProfileInput{Epoch: plan.Epoch}
	for _, field := range []struct {
		text        string
		destination *[32]byte
	}{
		{plan.NetworkID, &input.NetworkID}, {plan.StateGeneration, &input.StateGeneration}, {plan.EpochDigest, &input.EpochDigest},
		{plan.IssuerNodeID, &input.IssuerNodeID}, {plan.IssuanceAuthorityKey, &input.IssuanceAuthorityKey},
	} {
		value, decodeErr := decodeIdentifier(field.text)
		if decodeErr != nil {
			return state.ClosedProfileInput{}, errors.New("closed profile plan identity is invalid")
		}
		*field.destination = value
	}
	input.NotBefore, err = time.Parse(time.RFC3339, plan.NotBefore)
	if err == nil {
		input.NotAfter, err = time.Parse(time.RFC3339, plan.NotAfter)
	}
	if err != nil {
		return state.ClosedProfileInput{}, errors.New("closed profile plan validity is invalid")
	}
	for _, node := range plan.Nodes {
		id, idErr := decodeIdentifier(node.NodeID)
		digest, digestErr := decodeIdentifier(node.RecordDigest)
		if idErr != nil || digestErr != nil {
			return state.ClosedProfileInput{}, errors.New("closed profile plan Node is invalid")
		}
		input.Nodes = append(input.Nodes, state.ClosedProfileNodeInput{NodeID: id, RecordDigest: digest, RoleDomain: node.RoleDomain, Subrole: node.Subrole, DutyGeneration: node.DutyGeneration})
	}
	for _, key := range plan.TokenKeys {
		window, windowErr := time.Parse(time.RFC3339, key.WindowStart)
		spki, spkiErr := base64.RawStdEncoding.DecodeString(key.SPKI)
		if windowErr != nil || spkiErr != nil || base64.RawStdEncoding.EncodeToString(spki) != key.SPKI {
			return state.ClosedProfileInput{}, errors.New("closed profile plan token key is invalid")
		}
		input.TokenKeys = append(input.TokenKeys, state.ClosedProfileTokenKeyInput{WindowStart: window.UTC(), Class: key.Class, SPKI: spki})
	}
	return input, nil
}

func readClosedProfileSigner(path string) (ed25519.PrivateKey, error) {
	raw, err := readControlFile(path, 4096)
	if err != nil {
		return nil, errors.New("closed profile signer is unavailable")
	}
	block, rest := pem.Decode(raw)
	if block == nil || len(rest) != 0 || block.Type != "PRIVATE KEY" || len(block.Headers) != 0 {
		return nil, errors.New("closed profile signer is unavailable")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	private, ok := key.(ed25519.PrivateKey)
	if err != nil || !ok || len(private) != ed25519.PrivateKeySize {
		return nil, errors.New("closed profile signer is unavailable")
	}
	return append(ed25519.PrivateKey(nil), private...), nil
}

func writeNewClosedProfile(path string, raw []byte) error {
	if path == "" || len(raw) == 0 || len(raw) > maximumClosedProfilePlanBytes {
		return errors.New("closed profile destination is invalid")
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create closed profile: %w", err)
	}
	if _, err := file.Write(raw); err != nil {
		_ = file.Close()
		return fmt.Errorf("write closed profile: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return fmt.Errorf("flush closed profile: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close closed profile: %w", err)
	}
	persisted, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(persisted, raw) {
		_ = os.Remove(path)
		return errors.New("reopen closed profile failed")
	}
	return nil
}

func closedProfileReport(schema string, raw []byte) struct{ Schema, Digest string } {
	digest := sha256.Sum256(raw)
	return struct{ Schema, Digest string }{schema, hex.EncodeToString(digest[:])}
}

func zeroPrivateKey(key ed25519.PrivateKey) {
	for index := range key {
		key[index] = 0
	}
}
