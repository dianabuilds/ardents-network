package main

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	endpointapi "github.com/dianabuilds/ardents-network/internal/endpoint"
	"github.com/dianabuilds/ardents-network/internal/network/duty"
	"github.com/dianabuilds/ardents-network/internal/network/source"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/node"
	"github.com/dianabuilds/ardents-network/internal/route"
)

func runTransitRole(input config, role string) error {
	config, closeState, ready, err := nativeFixtureNodeConfig(input, role)
	if err != nil {
		return err
	}
	defer closeState()
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

func nativeFixtureNodeConfig(input config, role string) (node.Config, func() error, <-chan struct{}, error) {
	local, certificate, identity, err := nativeFixtureRole(input, role)
	if err != nil {
		return node.Config{}, nil, nil, err
	}
	authority, err := fixed(input.TransitAuthority)
	if err != nil {
		return node.Config{}, nil, nil, err
	}
	network, err := fixed(input.Network)
	if err != nil {
		return node.Config{}, nil, nil, err
	}
	stateRoot := input.TransitStateRoots[role]
	localRoot := filepath.Join(filepath.Dir(stateRoot), filepath.Base(stateRoot)+"-duty")
	stateSourceRoot := filepath.Join(filepath.Dir(stateRoot), filepath.Base(stateRoot)+"-source")
	sources, err := input.stateSourceConfig(role)
	if err != nil {
		return node.Config{}, nil, nil, err
	}
	store, err := state.Open(state.Config{Root: stateRoot, NetworkID: network,
		Authorities: map[[32]byte]ed25519.PublicKey{sha256.Sum256(authority[:]): ed25519.PublicKey(authority[:])}, Threshold: 1,
		Clock: time.Now, ObserveClock: time.Now, AcceptedProfile: route.Profile, Source: sources, LocalRoleStateRoot: stateSourceRoot,
		AutomaticRefreshInterval: 100 * time.Millisecond})
	if err != nil {
		return node.Config{}, nil, nil, err
	}
	ready := make(chan struct{})
	openedRoles, err := duty.Open(duty.Config{Root: localRoot, Clock: time.Now, Create: true})
	if err != nil {
		_ = store.Close()
		return node.Config{}, nil, nil, err
	}
	if err := openedRoles.Close(); err != nil {
		_ = store.Close()
		return node.Config{}, nil, nil, err
	}
	config := node.Config{NetworkID: network, NodeID: local.NodeID, IdentityKey: identity,
		Current: func() (node.DutyView, error) { return store.CurrentNodeDuty() }, PollInterval: 10 * time.Millisecond, Quarantine: time.Millisecond,
		LocalRoleStateRoot: localRoot, CheckPlacement: func() error { return nil },
		Emit: func(_ context.Context, event node.Event) error {
			if event.State == "READY" {
				if err := waitForFixtureStateSources(store, input.Deadline); err != nil {
					return err
				}
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
		config.Rendezvous = node.RendezvousProfile{Certificate: certificate, HandshakeLimit: 2, WaitingLimit: 2, PairLimit: 1, PairByteLimit: input.transitRelayByteLimit(), AdmissionTimeout: 3 * time.Second, DrainTimeout: time.Second}
	case "initiator":
		config.Initiator = node.InitiatorProfile{Certificate: certificate, HandshakeLimit: 2, RelayLimit: 2, RelayByteLimit: input.transitRelayByteLimit(), AdmissionTimeout: 3 * time.Second, DrainTimeout: time.Second}
	case "introduction":
		config.Introduction = node.IntroductionProfile{Certificate: certificate, HandshakeLimit: 3, SlotLimit: 1, DeliveryLimit: 1, AdmissionTimeout: 3 * time.Second, DrainTimeout: time.Second}
	case "responder":
		config.Responder = node.ResponderProfile{Certificate: certificate, HandshakeLimit: 2, RelayLimit: 1, RelayByteLimit: input.transitRelayByteLimit(), AdmissionTimeout: 3 * time.Second, DrainTimeout: time.Second}
	default:
		_ = store.Close()
		return node.Config{}, nil, nil, errors.New("C2 fixture transit role is unsupported")
	}
	return config, store.Close, ready, nil
}

func (input config) transitRelayByteLimit() uint64 {
	if !input.DynamicWorkload.configured() {
		return 256 << 10
	}
	return max(uint64(16<<20), uint64(input.DynamicWorkload.BytesEachDirection)*2,
		uint64(input.DynamicWorkload.Cycles)*(128<<10))
}

func (input config) stateSourceConfig(role string) (source.Config, error) {
	certificate, err := tls.X509KeyPair([]byte(input.TransitStateClient.Certificate), []byte(input.TransitStateClient.PrivateKey))
	if err != nil {
		return source.Config{}, errors.New("C2 fixture transit State client credential is invalid")
	}
	digest, err := fixed(input.Digest)
	if err != nil {
		return source.Config{}, err
	}
	var configured source.Config
	configured.ClientCertificate, configured.MaterialIndex, configured.OrderSeed = certificate, input.TransitStateMaterials[role], digest
	for index, declared := range input.TransitStateSources {
		key, decodeErr := fixed(declared.LeafKeyDigest)
		if decodeErr != nil {
			return source.Config{}, decodeErr
		}
		configured.Sources[index] = source.Source{Address: declared.Address, ServerName: declared.ServerName, RootPEM: []byte(declared.Root),
			LeafKeyDigest: key, Identity: sha256.Sum256([]byte("reference-c2-state-source-identity-" + declared.Address)),
			Family: "reference-c2-state-source-" + string(rune('a'+index)), EndpointHandle: "reference-c2-state-source-" + string(rune('a'+index))}
	}
	return configured, nil
}

type fixtureStateReader interface {
	Current() (state.Snapshot, error)
}

func waitForFixtureStateSources(store fixtureStateReader, encodedDeadline string) error {
	deadline, err := time.Parse(time.RFC3339, encodedDeadline)
	if err != nil {
		return err
	}
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		snapshot, currentErr := store.Current()
		if currentErr != nil {
			return currentErr
		}
		if snapshot.SourceAttempts == 2 && snapshot.SourceOutcomes[0] == "valid" && snapshot.SourceOutcomes[1] == "valid" {
			return nil
		}
		if !time.Now().UTC().Before(deadline) {
			return errors.New("C2 fixture State sources did not produce one valid wave")
		}
		<-ticker.C
	}
}

func nativeFixtureRole(input config, role string) (endpointapi.TransitPeer, tls.Certificate, ed25519.PrivateKey, error) {
	introduction, err := input.Introduction.decode()
	if err != nil {
		return endpointapi.TransitPeer{}, tls.Certificate{}, nil, err
	}
	rendezvous, err := input.Rendezvous.decode()
	if err != nil {
		return endpointapi.TransitPeer{}, tls.Certificate{}, nil, err
	}
	responder, err := input.Responder.decode()
	if err != nil {
		return endpointapi.TransitPeer{}, tls.Certificate{}, nil, err
	}
	initiator, err := input.Initiator.decode()
	if err != nil {
		return endpointapi.TransitPeer{}, tls.Certificate{}, nil, err
	}
	var local endpointapi.TransitPeer
	var selected peer
	switch role {
	case "rendezvous":
		local, selected = rendezvous, input.Rendezvous
	case "initiator":
		local, selected = initiator, input.Initiator
	case "introduction":
		local, selected = introduction, input.Introduction
	case "responder":
		local, selected = responder, input.Responder
	default:
		return endpointapi.TransitPeer{}, tls.Certificate{}, nil, errors.New("C2 fixture transit role is unsupported")
	}
	certificate, err := selected.tlsCertificate()
	if err != nil {
		return endpointapi.TransitPeer{}, tls.Certificate{}, nil, err
	}
	identity, ok := certificate.PrivateKey.(ed25519.PrivateKey)
	if !ok {
		return endpointapi.TransitPeer{}, tls.Certificate{}, nil, errors.New("C2 fixture native Node identity is invalid")
	}
	return local, certificate, identity, nil
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
