package node

import (
	"bytes"
	"crypto/ed25519"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/resource"
	"github.com/dianabuilds/ardents-network/internal/route"
)

type admissionKind byte

const (
	admissionAbsent admissionKind = iota
	admissionPrepared
	admissionReady
	admissionFailed
)

type admission struct {
	kind   admissionKind
	reason string
}

func resolveConfig(input Config) (runtimeConfig, error) {
	if input.Current == nil || input.Emit == nil || input.LocalRoleStateRoot == "" {
		return runtimeConfig{}, errors.New("node lifecycle callbacks are required")
	}
	if len(input.IdentityKey) != ed25519.PrivateKeySize {
		return runtimeConfig{}, errors.New("node lifecycle identity is invalid")
	}
	if input.PollInterval <= 0 || input.PollInterval > time.Second || input.Quarantine < 0 || input.Quarantine > 15*time.Second {
		return runtimeConfig{}, errors.New("node lifecycle time bounds are invalid")
	}
	now := input.Now
	if now == nil {
		now = time.Now
	}
	var (
		probePlan *probePlan
		err       error
	)
	if input.Probe.ListenAddress != "" {
		probePlan, err = newProbePlan(input.Probe, input.IdentityKey.Public().(ed25519.PublicKey), now)
		if err != nil {
			return runtimeConfig{}, err
		}
	}
	if probePlan == nil && input.Rendezvous.Certificate.PrivateKey == nil {
		return runtimeConfig{}, errors.New("node needs one local listener profile")
	}
	enforcePressure := input.ResourceProfile != ""
	if enforcePressure && input.ResourceProfile != "h3-np1-v1" && input.ResourceProfile != "h3-s-v1" &&
		input.ResourceProfile != "h3-s-v1-strong" {
		return runtimeConfig{}, errors.New("node resource profile is not supported")
	}
	var guard *resource.Guard
	if enforcePressure {
		guard, err = resource.New(resource.Config{Profile: input.ResourceProfile, Interval: input.PollInterval,
			Measure: input.ResourceMeasure})
		if err != nil {
			return runtimeConfig{}, err
		}
		if input.CheckPlacement == nil {
			input.CheckPlacement = guard.Check
		}
	} else if input.CheckPlacement == nil {
		input.CheckPlacement = func() error { return nil }
	}
	input.IdentityKey = append(ed25519.PrivateKey(nil), input.IdentityKey...)
	config := runtimeConfig{Config: input, now: func() time.Time { return now().UTC() }, probe: probePlan}
	if enforcePressure {
		config.pressure = guard
	}
	return config, nil
}

func assessAdmission(config runtimeConfig, snapshot dutyFacts) admission {
	if !snapshot.RecordPresent || snapshot.NodeID != config.NodeID {
		return admission{kind: admissionAbsent, reason: "local Node has no accepted materialized record"}
	}
	public := config.IdentityKey.Public().(ed25519.PublicKey)
	if !bytes.Equal(public, snapshot.NodePublicKey[:]) || snapshot.NetworkID != config.NetworkID {
		return admission{kind: admissionFailed, reason: "local Node identity or key does not match verified state"}
	}
	now := config.now()
	if snapshot.Profile == route.Profile {
		if _, err := rendezvousDuty(config.Rendezvous, snapshot); err != nil {
			return admission{kind: admissionPrepared, reason: err.Error()}
		}
		if snapshot.Conflicting || !snapshot.Fresh || now.Before(snapshot.EpochValidFrom) ||
			now.Before(snapshot.RecordValidFrom) || !now.Before(snapshot.ValidUntil) || !now.Before(snapshot.RecordValidUntil) {
			return admission{kind: admissionPrepared, reason: "freshness or validity is not satisfied"}
		}
		if err := config.CheckPlacement(); err != nil {
			return admission{kind: admissionPrepared, reason: "resource placement is not ready: " + boundedReason(err)}
		}
		return admission{kind: admissionReady}
	}
	if snapshot.Profile != "h3-role-probe-v1" || snapshot.Assignment == "" || snapshot.ProbeCapacity == 0 {
		return admission{kind: admissionPrepared, reason: "profile or deterministic assignment is inactive"}
	}
	if snapshot.ProbeEndpoint != config.Probe.ListenAddress {
		return admission{kind: admissionFailed, reason: "role-probe listener does not match the accepted Node Record"}
	}
	terminal := now.Add(config.Probe.MaximumDuty)
	if snapshot.Conflicting || !snapshot.Fresh || now.Before(snapshot.EpochValidFrom) ||
		now.Before(snapshot.RecordValidFrom) || !terminal.Before(snapshot.ValidUntil) ||
		!terminal.Before(snapshot.RecordValidUntil) {
		return admission{kind: admissionPrepared, reason: "freshness, validity, or terminal duty bound is not satisfied"}
	}
	if err := config.CheckPlacement(); err != nil {
		return admission{kind: admissionPrepared, reason: "resource placement is not ready: " + boundedReason(err)}
	}
	return admission{kind: admissionReady}
}

func boundedReason(err error) string {
	detail := err.Error()
	if len(detail) > 160 {
		detail = detail[:160]
	}
	return detail
}
