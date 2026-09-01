package endpoint

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/interfacev1/administration"
	applicationconnection "github.com/dianabuilds/ardents-network/internal/application/interfacev1/connection"
	"github.com/dianabuilds/ardents-network/internal/entry"
	"github.com/dianabuilds/ardents-network/internal/naming/alpha"
	"github.com/dianabuilds/ardents-network/internal/network/duty"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/service/instance"
)

// ParticipantRuntimeConfig contains the already decoded owner roots, trust
// anchors, and local Application addresses for one Endpoint process. It has no
// Browser, Target, Route plan, Grant, private Service Authority, or peer input.
type ParticipantRuntimeConfig struct {
	Network                 state.Config
	RefreshNetwork          bool
	EntryRoot               string
	TransitAcquisitionRoot  string
	AlphaCorpusRoot         string
	AlphaCorpusAuthority    ed25519.PublicKey
	AlphaCohort             string
	LocalRoleRoot           string
	ApplicationAddress      string
	AdministrationAddress   string
	PublicationRoot         string
	ServiceInstanceRoot     string
	BrokerID                [32]byte
	ConnectionPrincipal     [32]byte
	AdministrationPrincipal [32]byte
	BytesEachDirection      uint32
	Clock                   func() time.Time
	TimeConfident           func() bool
	Observe                 func(ParticipantRuntimeEvent) error
}

// ParticipantRuntimeEvent is the typed process lifecycle observed by the thin
// command Adapter. It contains no authority or network-route material.
type ParticipantRuntimeEvent struct {
	Kind                  string
	NetworkID             [32]byte
	ApplicationAddress    string
	AdministrationAddress string
}

// RunParticipant owns the live State, Entry, alpha floor, Endpoint, and both
// local Application transports until cancellation.
func RunParticipant(ctx context.Context, config ParticipantRuntimeConfig) (runErr error) {
	if ctx == nil || config.Network.Root == "" || config.EntryRoot == "" || config.TransitAcquisitionRoot == "" ||
		config.AlphaCorpusRoot == "" || len(config.AlphaCorpusAuthority) != ed25519.PublicKeySize || config.AlphaCohort == "" ||
		config.LocalRoleRoot == "" || config.ApplicationAddress == "" || config.AdministrationAddress == "" ||
		config.ApplicationAddress == config.AdministrationAddress || config.BrokerID == [32]byte{} ||
		config.ConnectionPrincipal == [32]byte{} || config.AdministrationPrincipal == [32]byte{} ||
		config.BytesEachDirection == 0 || config.TimeConfident == nil || config.Observe == nil {
		return errors.New("participant runtime configuration is incomplete")
	}
	clock := config.Clock
	if clock == nil {
		clock = time.Now
	}
	config.Network.Clock = clock
	network, err := state.Open(config.Network)
	if err != nil {
		return fmt.Errorf("open authenticated Network State: %w", err)
	}
	defer func() { runErr = errors.Join(runErr, network.Close()) }()
	if config.RefreshNetwork {
		if _, err := network.Refresh(ctx); err != nil {
			return fmt.Errorf("refresh authenticated Network State: %w", err)
		}
	}
	now := clock().UTC()
	view, err := network.CurrentResolution()
	if err != nil {
		return fmt.Errorf("read current Network State resolution: %w", err)
	}
	if _, available := view.Epoch(now, now.Add(time.Second)); !available {
		return errors.New("current Network State resolution is unavailable")
	}
	if !config.TimeConfident() {
		return errors.New("participant time confidence is unavailable")
	}
	entryOwner, err := entry.Open(entry.Config{Root: config.EntryRoot, Current: func() (entry.View, error) {
		current, currentErr := network.Current()
		if currentErr != nil {
			return entry.View{}, currentErr
		}
		return participantEntryView(current), nil
	}, Conflict: func(identity, family [32]byte) (bool, error) {
		return duty.ReadConflict(config.LocalRoleRoot, clock, identity, family)
	}, Clock: clock, TimeConfident: config.TimeConfident})
	if err != nil {
		return fmt.Errorf("open participant Entry state: %w", err)
	}
	defer func() { runErr = errors.Join(runErr, entryOwner.Close()) }()
	if _, err := entryOwner.Contact(); err != nil {
		return fmt.Errorf("read current participant Entry contact: %w", err)
	}
	floor, err := alpha.OpenPersistentFloor(alpha.PersistentFloorConfig{Root: config.AlphaCorpusRoot,
		Authority: config.AlphaCorpusAuthority, Cohort: config.AlphaCohort, Network: config.Network.NetworkID})
	if err != nil {
		return fmt.Errorf("open alpha corpus floor: %w", err)
	}
	defer func() { runErr = errors.Join(runErr, floor.Close()) }()
	corpus, err := floor.Current()
	if err != nil || corpus.ValidAt(now) != nil {
		return errors.New("accepted alpha corpus is unavailable")
	}
	setup := setup{NetworkID: config.Network.NetworkID, BrokerID: config.BrokerID,
		ConnectionPrincipal: config.ConnectionPrincipal, AdministrationPrincipal: config.AdministrationPrincipal,
		PublicationRoot: config.PublicationRoot, TransitAcquisitionRoot: config.TransitAcquisitionRoot,
		CreateTransitAcquisitionRoot: true, Clock: clock}
	var instanceRoot *instance.Root
	if config.ServiceInstanceRoot != "" {
		instanceRoot, err = instance.Open(config.ServiceInstanceRoot)
		if err != nil {
			return fmt.Errorf("open Service Instance root: %w", err)
		}
		defer func() { runErr = errors.Join(runErr, instanceRoot.Close()) }()
		credential, credentialErr := instanceRoot.Credential()
		if credentialErr != nil || credential.NetworkID != config.Network.NetworkID {
			return errors.Join(errors.New("accepted Service Instance Credential is unavailable"), credentialErr)
		}
		setup.AuthorityPublic = ed25519.PublicKey(append([]byte(nil), credential.AuthorityPublic[:]...))
		setup.IntroductionPublic = ed25519.PublicKey(append([]byte(nil), credential.IntroductionHPKEPublic[:]...))
	}
	owner, err := newEndpoint(setup)
	if err != nil {
		return fmt.Errorf("open participant Endpoint: %w", err)
	}
	defer func() { runErr = errors.Join(runErr, owner.Close()) }()
	if instanceRoot != nil {
		floor, floorErr := owner.publications.Floor()
		if floorErr != nil {
			return fmt.Errorf("read Service publication floor: %w", floorErr)
		}
		binding, bindingErr := instanceRoot.OpenBinding(floor)
		if bindingErr != nil {
			return fmt.Errorf("open current Service Instance binding: %w", bindingErr)
		}
		if err := owner.configurePublisher(func() (publisherAttachmentStateView, error) { return network.CurrentResolution() }, entryOwner, binding); err != nil {
			return errors.Join(errors.New("configure State-projected Publisher attachments"), err, binding.Withdraw())
		}
	}
	connectionOwner, err := owner.openConnectionInterface(connectionInterfaceConfig{Floor: floor,
		Current: func() (applicationStateView, error) { return network.CurrentResolution() }, Entry: entryOwner,
		Principal: config.ConnectionPrincipal, BytesEachDirection: config.BytesEachDirection, Clock: clock})
	if err != nil {
		return fmt.Errorf("open Connection Interface owner: %w", err)
	}
	connectionServer, err := applicationconnection.Listen(config.ApplicationAddress, connectionOwner)
	if err != nil {
		return fmt.Errorf("open local Connection Interface: %w", err)
	}
	defer func() { runErr = errors.Join(runErr, connectionServer.Close()) }()
	administrationOwner, err := owner.OpenServiceAdministration(serviceAdministrationConfig{
		Principal: config.AdministrationPrincipal, Clock: clock})
	if err != nil {
		return fmt.Errorf("open Service Administration owner: %w", err)
	}
	administrationServer, err := administration.Listen(config.AdministrationAddress, administrationOwner)
	if err != nil {
		return fmt.Errorf("open local Service Administration: %w", err)
	}
	defer func() { runErr = errors.Join(runErr, administrationServer.Close()) }()
	if err := config.Observe(ParticipantRuntimeEvent{Kind: "ready", NetworkID: config.Network.NetworkID,
		ApplicationAddress: config.ApplicationAddress, AdministrationAddress: config.AdministrationAddress}); err != nil {
		return err
	}
	<-ctx.Done()
	return config.Observe(ParticipantRuntimeEvent{Kind: "stopped", NetworkID: config.Network.NetworkID})
}

func participantEntryView(current state.Snapshot) entry.View {
	view := entry.View{NetworkID: current.NetworkID, Epoch: current.Epoch, Digest: current.Digest,
		Profile: current.Profile, Fresh: current.Freshness == "fresh"}
	for _, candidate := range current.Candidates[:current.CandidateCount] {
		view.Candidates = append(view.Candidates, entry.Candidate{NodeID: candidate.NodeID, PublicKey: candidate.PublicKey,
			KeyID: candidate.KeyID, FamilyID: candidate.FamilyID, RecordDigest: candidate.RecordDigest,
			DomainProofDigest: candidate.DomainProofDigest, Endpoint: candidate.Endpoint, Capacity: candidate.Capacity,
			Domain: candidate.Domain, ValidFrom: candidate.ValidFrom, ValidUntil: candidate.ValidUntil,
			AssignmentNotAfter: candidate.AssignmentNotAfter})
	}
	return view
}
