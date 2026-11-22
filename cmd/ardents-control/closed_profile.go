package main

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/closedprofile"
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
	view, err := closedprofile.Verify(raw, closedprofile.Context{StateGeneration: input.StateGeneration, NetworkID: input.NetworkID, EpochDigest: input.EpochDigest, Epoch: input.Epoch, Authority: authority, Now: at.UTC()})
	if err != nil {
		return errors.New("closed profile verification failed")
	}
	return json.NewEncoder(output).Encode(struct {
		Schema, Digest, Authority string
		Epoch                     uint64
		NotBefore, NotAfter       string
	}{"ardents-closed-profile-inspection-v1", hex.EncodeToString(view.Digest[:]), hex.EncodeToString(view.AuthorityKey[:]),
		view.Epoch, view.NotBefore.Format(time.RFC3339), view.NotAfter.Format(time.RFC3339)})
}

func readClosedProfilePlan(path string) (closedprofile.Input, error) {
	raw, err := readControlFile(path, maximumClosedProfilePlanBytes)
	if err != nil {
		return closedprofile.Input{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var plan closedProfilePlan
	if err := decoder.Decode(&plan); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return closedprofile.Input{}, errors.New("closed profile plan is invalid")
	}
	input := closedprofile.Input{Epoch: plan.Epoch}
	for _, field := range []struct {
		text        string
		destination *[32]byte
	}{
		{plan.NetworkID, &input.NetworkID}, {plan.StateGeneration, &input.StateGeneration}, {plan.EpochDigest, &input.EpochDigest},
		{plan.IssuerNodeID, &input.IssuerNodeID}, {plan.IssuanceAuthorityKey, &input.IssuanceAuthorityKey},
	} {
		value, decodeErr := decodeIdentifier(field.text)
		if decodeErr != nil {
			return closedprofile.Input{}, errors.New("closed profile plan identity is invalid")
		}
		*field.destination = value
	}
	input.NotBefore, err = time.Parse(time.RFC3339, plan.NotBefore)
	if err == nil {
		input.NotAfter, err = time.Parse(time.RFC3339, plan.NotAfter)
	}
	if err != nil {
		return closedprofile.Input{}, errors.New("closed profile plan validity is invalid")
	}
	for _, node := range plan.Nodes {
		id, idErr := decodeIdentifier(node.NodeID)
		digest, digestErr := decodeIdentifier(node.RecordDigest)
		if idErr != nil || digestErr != nil {
			return closedprofile.Input{}, errors.New("closed profile plan Node is invalid")
		}
		input.Nodes = append(input.Nodes, closedprofile.NodeInput{NodeID: id, RecordDigest: digest, RoleDomain: node.RoleDomain, Subrole: node.Subrole, DutyGeneration: node.DutyGeneration})
	}
	for _, key := range plan.TokenKeys {
		window, windowErr := time.Parse(time.RFC3339, key.WindowStart)
		spki, spkiErr := base64.RawStdEncoding.DecodeString(key.SPKI)
		if windowErr != nil || spkiErr != nil || base64.RawStdEncoding.EncodeToString(spki) != key.SPKI {
			return closedprofile.Input{}, errors.New("closed profile plan token key is invalid")
		}
		input.TokenKeys = append(input.TokenKeys, closedprofile.TokenKeyInput{WindowStart: window.UTC(), Class: key.Class, SPKI: spki})
	}
	return input, nil
}
