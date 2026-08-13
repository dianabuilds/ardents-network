package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route"
)

func (raw actorPlan) client() (route.Actor, func(), error) {
	at, err := time.Parse(time.RFC3339, raw.At)
	if err != nil {
		return route.Actor{}, nil, err
	}
	var network, seed, publisher [32]byte
	if err := fixedHex(raw.NetworkID, network[:]); err != nil {
		return route.Actor{}, nil, err
	}
	if err := fixedHex(raw.Seed, seed[:]); err != nil {
		return route.Actor{}, nil, err
	}
	if err := fixedHex(raw.PublisherPin, publisher[:]); err != nil {
		return route.Actor{}, nil, err
	}
	authorities := make(map[[32]byte]ed25519.PublicKey, len(raw.Authorities))
	for _, encoded := range raw.Authorities {
		public := make([]byte, ed25519.PublicKeySize)
		if err := fixedHex(encoded, public); err != nil {
			return route.Actor{}, nil, err
		}
		authorities[sha256.Sum256(public)] = ed25519.PublicKey(public)
	}
	opened, err := state.Open(state.Config{Root: raw.StateRoot, NetworkID: network,
		Authorities: authorities, Threshold: raw.Threshold, AcceptedProfile: "h3-route-tracer-v1", Now: at})
	if err != nil {
		return route.Actor{}, nil, err
	}
	view, err := opened.Current()
	if err != nil {
		opened.Close()
		return route.Actor{}, nil, err
	}
	excluded, err := identities(raw.ExcludedIdentities)
	if err != nil {
		opened.Close()
		return route.Actor{}, nil, err
	}
	plan, err := route.Select(view, route.Selection{Seed: seed, At: at, ExcludedIdentities: excluded,
		ExcludedFamilies: raw.ExcludedFamilies, ExcludedDomains: raw.ExcludedDomains})
	if err != nil {
		opened.Close()
		return route.Actor{}, nil, err
	}
	certificate, err := tls.LoadX509KeyPair(raw.Certificate, raw.Key)
	if err != nil {
		opened.Close()
		return route.Actor{}, nil, err
	}
	deadline, err := duration(raw.Deadline)
	if err != nil {
		opened.Close()
		return route.Actor{}, nil, err
	}
	canary, err := hex.DecodeString(raw.Canary)
	if err != nil || raw.Canary != "" && len(canary) != 32 {
		opened.Close()
		return route.Actor{}, nil, errors.New("canary must be empty or exactly 32 bytes")
	}
	return route.Actor{Role: "client", Plan: plan, ClientCertificate: certificate,
		PublisherPin: publisher, Canary: canary, Deadline: deadline}, func() { _ = opened.Close() }, nil
}

func identities(values []string) ([][32]byte, error) {
	result := make([][32]byte, len(values))
	for index := range values {
		if err := fixedHex(values[index], result[index][:]); err != nil {
			return nil, err
		}
	}
	return result, nil
}
