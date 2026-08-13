package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"
)

type actorPlan struct {
	Role, ManifestDigest, NetworkID, EpochDigest, NodeID      string
	Listen, Certificate, Key, UpstreamPin                     string
	NextNodeID, Next, NextPin, ServiceCertificate, ServiceKey string
	Deadline, StateRoot, At, Seed, PublisherPin               string
	Authorities, ExcludedFamilies, ExcludedDomains            []string
	ExcludedIdentities                                        []string
	Threshold                                                 int
}

func (value actorPlan) validateRoleLocal() error {
	clientOnly := value.StateRoot != "" || value.At != "" || value.Seed != "" || value.PublisherPin != "" ||
		len(value.Authorities) != 0 || value.Threshold != 0 || len(value.ExcludedIdentities) != 0 ||
		len(value.ExcludedFamilies) != 0 || len(value.ExcludedDomains) != 0
	nextOnly := value.NextNodeID != "" || value.Next != "" || value.NextPin != ""
	serviceOnly := value.ServiceCertificate != "" || value.ServiceKey != ""
	listenerOnly := value.Listen != "" || value.UpstreamPin != "" || value.NodeID != "" || value.EpochDigest != ""
	switch value.Role {
	case "client":
		if listenerOnly || nextOnly || serviceOnly {
			return errors.New("client plan contains information outside its role-local duty")
		}
	case "publisher":
		if clientOnly || nextOnly {
			return errors.New("publisher plan contains information outside its role-local duty")
		}
	case "initiator", "introduction", "rendezvous", "responder":
		if clientOnly || serviceOnly {
			return errors.New("node plan contains information outside its role-local duty")
		}
	default:
		return errors.New("role plan has an invalid actor role")
	}
	return nil
}

func readActorPlan(path string) (actorPlan, error) {
	file, err := os.Open(path)
	if err != nil {
		return actorPlan{}, err
	}
	defer file.Close()
	limited := io.LimitReader(file, 64<<10+1)
	raw, err := io.ReadAll(limited)
	if err != nil || len(raw) == 0 || len(raw) > 64<<10 {
		return actorPlan{}, errors.New("role plan is empty or exceeds 64 KiB")
	}
	var value actorPlan
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return actorPlan{}, fmt.Errorf("decode role plan: %w", err)
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return actorPlan{}, errors.New("role plan contains multiple JSON values")
	}
	if err := value.validateRoleLocal(); err != nil {
		return actorPlan{}, err
	}
	return value, nil
}

func fixedHex(encoded string, destination []byte) error {
	if len(encoded) != len(destination)*2 {
		return errors.New("hexadecimal field has wrong length")
	}
	_, err := hex.Decode(destination, []byte(encoded))
	return err
}

func duration(value string) (time.Duration, error) {
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("parse deadline: %w", err)
	}
	return parsed, nil
}
