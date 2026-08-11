package nodelifecycle

import (
	"bytes"
	"crypto/ed25519"
	"errors"
	"net"
	"strconv"
	"time"

	"github.com/dianabuilds/ardents-network/internal/networkstate"
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
	if input.Current == nil || input.Emit == nil || input.ListenAddress == "" {
		return runtimeConfig{}, errors.New("node lifecycle callbacks and listener are required")
	}
	host, port, err := net.SplitHostPort(input.ListenAddress)
	portNumber, portErr := strconv.Atoi(port)
	if err != nil || net.ParseIP(host) == nil || portErr != nil || portNumber < 1 || portNumber > 65535 {
		return runtimeConfig{}, errors.New("node role-probe endpoint must be a literal IP and port")
	}
	if len(input.IdentityKey) != ed25519.PrivateKeySize || len(input.Certificate.Certificate) == 0 ||
		len(input.ClientRootPEM) == 0 || len(input.ClientRootPEM) > 64<<10 || len(input.ClientKeyPins) == 0 || len(input.ClientKeyPins) > 16 {
		return runtimeConfig{}, errors.New("node lifecycle identity and TLS trust are invalid")
	}
	seenPins := make(map[[32]byte]bool, len(input.ClientKeyPins))
	for _, pin := range input.ClientKeyPins {
		if pin == [32]byte{} || seenPins[pin] {
			return runtimeConfig{}, errors.New("node lifecycle client key pins are invalid")
		}
		seenPins[pin] = true
	}
	if input.PollInterval <= 0 || input.PollInterval > time.Second || input.MaximumDuty <= 0 ||
		input.MaximumDuty > 15*time.Second || input.DrainTimeout <= 0 || input.DrainTimeout > 15*time.Second ||
		input.Quarantine < 0 || input.Quarantine > 15*time.Second {
		return runtimeConfig{}, errors.New("node lifecycle time bounds are invalid")
	}
	now := input.Now
	if now == nil {
		now = time.Now
	}
	if err := cloneTLSMaterial(&input, now().UTC()); err != nil {
		return runtimeConfig{}, err
	}
	if input.CheckPlacement == nil {
		input.CheckPlacement = checkProcessPlacement
	}
	input.IdentityKey = append(ed25519.PrivateKey(nil), input.IdentityKey...)
	return runtimeConfig{Config: input, now: func() time.Time { return now().UTC() }}, nil
}

func assessAdmission(config runtimeConfig, snapshot networkstate.Snapshot) admission {
	if !snapshot.RecordPresent || snapshot.NodeID != config.NodeID {
		return admission{kind: admissionAbsent, reason: "local Node has no accepted materialized record"}
	}
	public := config.IdentityKey.Public().(ed25519.PublicKey)
	if !bytes.Equal(public, snapshot.NodePublicKey[:]) || snapshot.NetworkID != config.NetworkID {
		return admission{kind: admissionFailed, reason: "local Node identity or key does not match verified state"}
	}
	if snapshot.Profile != "h3-role-probe-v1" || snapshot.Assignment == "" || snapshot.ProbeCapacity == 0 {
		return admission{kind: admissionPrepared, reason: "profile or deterministic assignment is inactive"}
	}
	if snapshot.ProbeEndpoint != config.ListenAddress {
		return admission{kind: admissionFailed, reason: "role-probe listener does not match the accepted Node Record"}
	}
	now := config.now()
	terminal := now.Add(config.MaximumDuty)
	if snapshot.Conflicting || snapshot.Freshness != "fresh" || now.Before(snapshot.EpochValidFrom) ||
		now.Before(snapshot.RecordValidFrom) || !terminal.Before(snapshot.ValidUntil) ||
		!terminal.Before(snapshot.RecordValidUntil) {
		return admission{kind: admissionPrepared, reason: "freshness, validity, or terminal duty bound is not satisfied"}
	}
	if err := config.CheckPlacement(); err != nil {
		detail := err.Error()
		if len(detail) > 160 {
			detail = detail[:160]
		}
		return admission{kind: admissionPrepared, reason: "resource placement is not ready: " + detail}
	}
	return admission{kind: admissionReady}
}
