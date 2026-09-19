//go:build linux

package main

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"io"

	"github.com/dianabuilds/ardents-network/internal/application/streamqualification"
	"github.com/dianabuilds/ardents-network/internal/endpoint"
	"github.com/dianabuilds/ardents-network/internal/network/state"
)

type qualificationPlan struct {
	Mode         string
	Participants []qualificationParticipantPlan
}

type decodedQualificationPlan struct {
	Mode    string
	Configs []endpoint.StreamQualificationConfig
}
type qualificationParticipantPlan struct {
	Participant json.RawMessage
	Role        streamqualification.Role
	Profile     streamqualification.Profile
	Condition   streamqualification.NetworkCondition
	Seed        string
	ReaderIndex int
	Link        string
	HostingRoot string
}

func decodePlan(input io.Reader) (decodedQualificationPlan, error) {
	body, err := io.ReadAll(io.LimitReader(input, (256<<10)+1))
	if err != nil || len(body) > 256<<10 {
		return decodedQualificationPlan{}, errors.New("qualification plan exceeds its bound")
	}
	var plan qualificationPlan
	if err := decodeExact(body, &plan); err != nil {
		return decodedQualificationPlan{}, err
	}
	if plan.Mode == "" {
		plan.Mode = "stream"
	}
	if plan.Mode != "stream" && plan.Mode != "net32-idle" {
		return decodedQualificationPlan{}, errors.New("qualification plan mode is invalid")
	}
	if len(plan.Participants) == 0 || len(plan.Participants) > 5 {
		return decodedQualificationPlan{}, errors.New("qualification participant set invalid")
	}
	var configs []endpoint.StreamQualificationConfig
	readers := map[int]bool{}
	publisher := false
	var ownerRole streamqualification.Role
	for _, item := range plan.Participants {
		if _, err := item.Profile.Definition(item.Role); err != nil {
			return decodedQualificationPlan{}, err
		}
		if _, err := item.Condition.CarrierRatioLimit(); err != nil {
			return decodedQualificationPlan{}, err
		}
		if ownerRole == 0 {
			ownerRole = item.Role
		} else if ownerRole != item.Role {
			return decodedQualificationPlan{}, errors.New("one host run cannot combine User and Publisher owners")
		}
		if item.Role == streamqualification.PublisherRole {
			if publisher || item.ReaderIndex != 0 {
				return decodedQualificationPlan{}, errors.New("duplicate Publisher")
			}
			publisher = true
		} else {
			if item.ReaderIndex < 0 || item.ReaderIndex > 3 || readers[item.ReaderIndex] {
				return decodedQualificationPlan{}, errors.New("duplicate or invalid Reader")
			}
			readers[item.ReaderIndex] = true
		}
		var participant struct {
			endpoint.TextParticipantConfig
			Network json.RawMessage
		}
		if err := decodeExact(item.Participant, &participant); err != nil {
			return decodedQualificationPlan{}, err
		}
		var network struct {
			state.Config
			Authorities map[string]string
		}
		if err := decodeExact(participant.Network, &network); err != nil {
			return decodedQualificationPlan{}, err
		}
		network.Config.Authorities = make(map[[32]byte]ed25519.PublicKey)
		for spelling, key := range network.Authorities {
			id, err := decodeIdentity(spelling)
			if err != nil {
				return decodedQualificationPlan{}, err
			}
			public, err := decodeIdentity(key)
			if err != nil {
				return decodedQualificationPlan{}, err
			}
			network.Config.Authorities[id] = append(ed25519.PublicKey(nil), public[:]...)
		}
		seed, err := decodeIdentity(item.Seed)
		if err != nil {
			return decodedQualificationPlan{}, err
		}
		participant.TextParticipantConfig.Network = network.Config
		if len(configs) > 0 && (configs[0].HostingRoot != item.HostingRoot || configs[0].Profile != item.Profile || configs[0].Condition != item.Condition || configs[0].Seed != seed) {
			return decodedQualificationPlan{}, errors.New("one host run must share its period and workload")
		}
		configs = append(configs, endpoint.StreamQualificationConfig{Participant: participant.TextParticipantConfig, Role: item.Role, Profile: item.Profile, Condition: item.Condition, Seed: seed, ReaderIndex: item.ReaderIndex, Link: item.Link, HostingRoot: item.HostingRoot})
	}
	if plan.Mode == "net32-idle" {
		if len(configs) != 1 || ownerRole != streamqualification.ReaderRole || configs[0].Profile != streamqualification.ClientToPublisher || configs[0].Condition != streamqualification.NormalNetwork || configs[0].ReaderIndex != 0 || configs[0].Link != "" {
			return decodedQualificationPlan{}, errors.New("NET-32 idle plan must contain one normal User without a destination")
		}
		return decodedQualificationPlan{Mode: plan.Mode, Configs: configs}, nil
	}
	if ownerRole == streamqualification.PublisherRole && (len(configs) != 1 || !publisher) {
		return decodedQualificationPlan{}, errors.New("Publisher owner plan must contain its one participant")
	}
	if ownerRole == streamqualification.ReaderRole && (len(configs) != 4 || len(readers) != 4) {
		return decodedQualificationPlan{}, errors.New("User owner plan must contain all four Readers")
	}
	return decodedQualificationPlan{Mode: plan.Mode, Configs: configs}, nil
}

func decodeExact(body []byte, value any) error {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	var trailing any
	if decoder.Decode(&trailing) != io.EOF {
		return errors.New("qualification plan trailing data")
	}
	return nil
}
