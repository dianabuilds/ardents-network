package main

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	endpointapi "github.com/dianabuilds/ardents-network/internal/endpoint"
	"github.com/dianabuilds/ardents-network/internal/network/duty"
	"github.com/dianabuilds/ardents-network/internal/node"
)

func runTransitRole(input config, role string) error {
	config, ready, err := nativeFixtureNodeConfig(input, role)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	completed := make(chan nativeFixtureResult, 1)
	go func() { result, runErr := node.Run(ctx, config); completed <- nativeFixtureResult{result, runErr} }()
	deadline, _ := input.deadline()
	timer := time.NewTimer(time.Until(deadline))
	defer timer.Stop()
	select {
	case result := <-completed:
		return errors.Join(result.err, errors.New("C2 fixture native Node stopped before readiness"))
	case <-timer.C:
		return errors.New("C2 fixture native Node did not become ready")
	case <-ready:
	}
	work, workCancel := context.WithDeadline(context.Background(), deadline)
	defer workCancel()
	workDone := make(chan error, 1)
	go func() { workDone <- waitForTransitCompletion(work, input.CompletePath) }()
	select {
	case err := <-workDone:
		if err == nil {
			break
		}
		cancel()
		<-completed
		return err
	case result := <-completed:
		return errors.Join(result.err, errors.New("C2 fixture native Node stopped before transit completion"))
	}
	cancel()
	result := <-completed
	if result.err != nil || result.result.State != "WITHDRAWN" {
		return errors.Join(result.err, errors.New("C2 fixture native Node did not drain through its lifecycle"))
	}
	return jsonResult(role, "drained")
}

type nativeFixtureResult struct {
	result node.Result
	err    error
}

func nativeFixtureNodeConfig(input config, role string) (node.Config, <-chan struct{}, error) {
	local, certificate, identity, view, err := nativeFixtureRole(input, role)
	if err != nil {
		return node.Config{}, nil, err
	}
	ready := make(chan struct{})
	localRoot := filepath.Join(input.ReadyRoot, "native-role-"+role)
	openedRoles, err := duty.Open(duty.Config{Root: localRoot, Clock: time.Now, Create: true})
	if err != nil {
		return node.Config{}, nil, err
	}
	if err := openedRoles.Close(); err != nil {
		return node.Config{}, nil, err
	}
	config := node.Config{NetworkID: view.DutyNetworkID(), NodeID: local.NodeID, IdentityKey: identity,
		Current: func() (node.DutyView, error) { return view, nil }, PollInterval: 10 * time.Millisecond, Quarantine: time.Millisecond,
		LocalRoleStateRoot: localRoot, CheckPlacement: func() error { return nil },
		Emit: func(_ context.Context, event node.Event) error {
			if event.State == "READY" {
				select {
				case <-ready:
				default:
					close(ready)
					if err := writeTransitReady(input.ReadyRoot, role); err != nil {
						return err
					}
				}
			}
			return nil
		}}
	switch role {
	case "rendezvous":
		config.Rendezvous = node.RendezvousProfile{Certificate: certificate, HandshakeLimit: 2, WaitingLimit: 2, PairLimit: 1, PairByteLimit: 256 << 10, DrainTimeout: time.Second}
	case "initiator":
		config.Initiator = node.InitiatorProfile{Certificate: certificate, HandshakeLimit: 2, RelayLimit: 2, RelayByteLimit: 256 << 10, DrainTimeout: time.Second}
	case "introduction":
		config.Introduction = node.IntroductionProfile{Certificate: certificate, HandshakeLimit: 3, SlotLimit: 1, DeliveryLimit: 1, DrainTimeout: time.Second}
	case "responder":
		config.Responder = node.ResponderProfile{Certificate: certificate, HandshakeLimit: 2, RelayLimit: 1, RelayByteLimit: 256 << 10, DrainTimeout: time.Second}
	default:
		return node.Config{}, nil, errors.New("C2 fixture transit role is unsupported")
	}
	return config, ready, nil
}

func nativeFixtureRole(input config, role string) (endpointapi.TransitPeer, tls.Certificate, ed25519.PrivateKey, node.DutyView, error) {
	introduction, err := input.Introduction.decode()
	if err != nil {
		return endpointapi.TransitPeer{}, tls.Certificate{}, nil, nil, err
	}
	rendezvous, err := input.Rendezvous.decode()
	if err != nil {
		return endpointapi.TransitPeer{}, tls.Certificate{}, nil, nil, err
	}
	responder, err := input.Responder.decode()
	if err != nil {
		return endpointapi.TransitPeer{}, tls.Certificate{}, nil, nil, err
	}
	initiator, err := input.Initiator.decode()
	if err != nil {
		return endpointapi.TransitPeer{}, tls.Certificate{}, nil, nil, err
	}
	gateway, err := input.Gateway.decode()
	if err != nil {
		return endpointapi.TransitPeer{}, tls.Certificate{}, nil, nil, err
	}
	var local endpointapi.TransitPeer
	var candidates []fixtureDutyCandidate
	var selected peer
	switch role {
	case "rendezvous":
		local, selected, candidates = rendezvous, input.Rendezvous, []fixtureDutyCandidate{fixtureCandidate(initiator, "initiator"), fixtureCandidate(responder, "responder")}
	case "initiator":
		local, selected, candidates = initiator, input.Initiator, []fixtureDutyCandidate{fixtureCandidate(initiator, "initiator"), fixtureCandidate(rendezvous, "rendezvous"), fixtureCandidate(gateway, "destination-resolution")}
	case "introduction":
		local, selected = introduction, input.Introduction
	case "responder":
		local, selected, candidates = responder, input.Responder, []fixtureDutyCandidate{fixtureCandidate(rendezvous, "rendezvous")}
	default:
		return endpointapi.TransitPeer{}, tls.Certificate{}, nil, nil, errors.New("C2 fixture transit role is unsupported")
	}
	certificate, err := selected.tlsCertificate()
	if err != nil {
		return endpointapi.TransitPeer{}, tls.Certificate{}, nil, nil, err
	}
	identity, ok := certificate.PrivateKey.(ed25519.PrivateKey)
	if !ok {
		return endpointapi.TransitPeer{}, tls.Certificate{}, nil, nil, errors.New("C2 fixture native Node identity is invalid")
	}
	view, err := newFixtureDutyView(input, local, role, candidates)
	return local, certificate, identity, view, err
}

func (value peer) tlsCertificate() (tls.Certificate, error) {
	if value.Certificate == "" || value.PrivateKey == "" {
		return tls.Certificate{}, errors.New("C2 fixture transit certificate is unavailable")
	}
	return tls.X509KeyPair([]byte(value.Certificate), []byte(value.PrivateKey))
}

func writeTransitReady(root, role string) error {
	if root == "" || filepath.Base(root) == "." || filepath.Base(root) == string(filepath.Separator) {
		return errors.New("C2 fixture ready root is invalid")
	}
	file, err := os.OpenFile(filepath.Join(root, role), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.WriteString("ready\n"); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func waitForTransitCompletion(ctx context.Context, path string) error {
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() && info.Size() > 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("c2 fixture transit completion is unavailable: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

func jsonResult(role, class string) error {
	return json.NewEncoder(os.Stdout).Encode(result{Schema: "ardents-e2e-reference-c2-result-v1", Role: role, Class: class, Passed: true})
}
