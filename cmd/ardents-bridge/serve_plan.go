package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/dianabuilds/ardents-network/internal/camouflage"
	"github.com/dianabuilds/ardents-network/internal/node"
	"github.com/dianabuilds/ardents-network/internal/node/probe"
	"github.com/dianabuilds/ardents-network/internal/planfile"
	"github.com/dianabuilds/ardents-network/internal/routeplan"
)

type servePlan struct {
	Schema                   string   `json:"schema"`
	ImportPlan               string   `json:"import_plan"`
	Binary                   string   `json:"binary"`
	CandidateStateRoot       string   `json:"candidate_state_root"`
	Certificate              string   `json:"certificate"`
	Key                      string   `json:"key"`
	NextInitiatorPlan        string   `json:"next_initiator_plan"`
	RouteManifestDigest      string   `json:"route_manifest_digest"`
	Deadline                 string   `json:"deadline"`
	IdentityKey              string   `json:"identity_key"`
	ProbeListen              string   `json:"probe_listen"`
	ProbeCertificate         string   `json:"probe_certificate"`
	ProbeKey                 string   `json:"probe_key"`
	ProbeClientRoot          string   `json:"probe_client_root"`
	ProbeClientKeyDigests    []string `json:"probe_client_key_digests"`
	MaximumDutyMilliseconds  uint32   `json:"maximum_duty_ms"`
	DrainTimeoutMilliseconds uint32   `json:"drain_timeout_ms"`
	ResourceProfile          string   `json:"resource_profile,omitempty"`
}

type serveRuntime struct {
	bridge importRuntime
	server camouflage.Server
	node   node.Config
}

func loadServePlan(path string) (serveRuntime, error) {
	var raw servePlan
	if err := planfile.Decode(path, 32<<10, &raw); err != nil {
		return serveRuntime{}, err
	}
	if raw.Schema != "ardents-h3-bridge-serve-plan-v1" || raw.ImportPlan == "" || raw.IdentityKey == "" ||
		raw.MaximumDutyMilliseconds == 0 || raw.MaximumDutyMilliseconds > 15000 ||
		raw.DrainTimeoutMilliseconds == 0 || raw.DrainTimeoutMilliseconds > 15000 || (raw.ResourceProfile != "h3-s-v1" && raw.ResourceProfile != "h3-s-v1-strong") {
		return serveRuntime{}, errors.New("serve plan is not canonical or complete")
	}
	bridgeRuntime, err := loadImportPlan(raw.ImportPlan, time.Now)
	if err != nil {
		return serveRuntime{}, err
	}
	fail := func(cause error) (serveRuntime, error) {
		return serveRuntime{}, errors.Join(cause, bridgeRuntime.close())
	}
	deadline, err := time.Parse(time.RFC3339, raw.Deadline)
	if err != nil {
		return fail(fmt.Errorf("parse serve deadline: %w", err))
	}
	var manifest [32]byte
	if err := planfile.FixedHex(raw.RouteManifestDigest, manifest[:]); err != nil {
		return fail(err)
	}
	nextLeg, err := routeplan.BridgeNext(raw.NextInitiatorPlan, manifest)
	if err != nil {
		return fail(err)
	}
	identity, err := node.IdentityKey(raw.IdentityKey)
	if err != nil {
		return fail(err)
	}
	certificate, err := planfile.KeyPair(raw.ProbeCertificate, raw.ProbeKey)
	if err != nil {
		return fail(err)
	}
	clientRoot, err := planfile.Read(raw.ProbeClientRoot, 64<<10)
	if err != nil {
		return fail(err)
	}
	pins, err := planfile.Digests(raw.ProbeClientKeyDigests, 16)
	if err != nil || len(pins) == 0 {
		return fail(errors.New("probe client pins are invalid"))
	}
	runtime := serveRuntime{bridge: bridgeRuntime}
	runtime.server = camouflage.Server{Binary: raw.Binary, StateRoot: raw.CandidateStateRoot,
		Certificate: raw.Certificate, Key: raw.Key, NextLeg: nextLeg, Deadline: deadline, ResourceProfile: raw.ResourceProfile}
	runtime.node = node.Config{IdentityKey: identity, LocalRoleStateRoot: bridgeRuntime.localRoleRoot,
		Probe: probe.Config{ListenAddress: raw.ProbeListen, Certificate: certificate, ClientRootPEM: clientRoot,
			ClientKeyPins: pins, MaximumDuty: time.Duration(raw.MaximumDutyMilliseconds) * time.Millisecond,
			DrainTimeout: time.Duration(raw.DrainTimeoutMilliseconds) * time.Millisecond},
		PollInterval: 100 * time.Millisecond, Quarantine: time.Second, ResourceProfile: raw.ResourceProfile, Now: time.Now}
	return runtime, nil
}
