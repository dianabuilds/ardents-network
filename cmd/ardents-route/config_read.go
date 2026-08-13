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
	Role, NetworkID, EpochDigest, NodeID                      string
	Listen, Certificate, Key, UpstreamPin                     string
	NextNodeID, Next, NextPin, ServiceCertificate, ServiceKey string
	Deadline, StateRoot, At, Seed, PublisherPin, Canary       string
	Authorities, ExcludedFamilies, ExcludedDomains            []string
	ExcludedIdentities                                        []string
	Threshold                                                 int
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
