//go:build linux

package endpoint

import (
	"crypto/ed25519"
	"errors"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/service/instance"
	"github.com/dianabuilds/ardents-network/internal/service/publication"
)

const (
	publishCapability  = uint32(1)
	connectCapability  = uint32(2)
	maximumStreamBytes = uint32(768 << 20)
)

// Setup fixes one Endpoint broker generation and its two local principals.
type setup struct {
	NetworkID               [32]byte
	BrokerID                [32]byte
	AuthorityPublic         ed25519.PublicKey
	IntroductionPublic      ed25519.PublicKey
	ConnectionPrincipal     [32]byte
	AdministrationPrincipal [32]byte
	PublicationRoot         string
	Clock                   func() time.Time
	Resources               func(string, int) uint32
	Admission               *broker.Broker
}

// endpoint owns one broker generation's sessions and current publication.
type endpoint struct {
	endpointTextState
	clock            func() time.Time
	network          [32]byte
	authority        [32]byte
	admission        *broker.Broker
	publications     *publication.Publication
	resources        func(string, int) uint32
	publisherMu      sync.Mutex
	publisherBinding *instance.Binding
}

// New creates one finite Endpoint-local admission and publication boundary.
func newEndpoint(input setup) (*endpoint, error) {
	if input.NetworkID == [32]byte{} || input.BrokerID == [32]byte{} || input.ConnectionPrincipal == [32]byte{} ||
		len(input.AuthorityPublic) != 0 && len(input.AuthorityPublic) != ed25519.PublicKeySize ||
		len(input.IntroductionPublic) != 0 && len(input.IntroductionPublic) != ed25519.PublicKeySize {
		return nil, errors.New("endpoint setup is incomplete")
	}
	var authority [32]byte
	copy(authority[:], input.AuthorityPublic)
	clock := input.Clock
	if clock == nil {
		clock = time.Now
	}
	resources := input.Resources
	if resources == nil {
		resources = newResourceObserver()
	}
	admission := input.Admission
	if admission == nil {
		grants := []broker.Grant{{Principal: input.ConnectionPrincipal, Surface: broker.Connection}}
		if input.AdministrationPrincipal != [32]byte{} {
			grants = append(grants, broker.Grant{Principal: input.AdministrationPrincipal, Surface: broker.Administration})
		}
		openedAdmission, err := broker.New(broker.Config{ID: input.BrokerID, Grants: grants, Clock: clock})
		if err != nil {
			return nil, err
		}
		admission = openedAdmission
	}
	endpoint := &endpoint{clock: clock, network: input.NetworkID, authority: authority,
		admission: admission, resources: resources}
	if input.AdministrationPrincipal != [32]byte{} && authority != [32]byte{} {
		if input.PublicationRoot == "" {
			return nil, errors.New("publisher setup lacks a publication root")
		}
		opened, err := publication.Open(publication.Config{Root: input.PublicationRoot,
			NetworkID: input.NetworkID,
			Authority: ed25519.PublicKey(authority[:]), Clock: clock})
		if err != nil {
			return nil, err
		}
		endpoint.publications = opened
	}
	return endpoint, nil
}

// Close releases the local admission boundary, the retained text state, and
// the publication root. Client-only endpoints have no publication owner to
// close.
func (endpoint *endpoint) Close() error {
	if endpoint == nil {
		return nil
	}
	endpoint.admission.Close()
	textErr := errors.Join(endpoint.closeTextContexts(), endpoint.closeTextSourceRoots())
	if endpoint.publications == nil {
		return textErr
	}
	return errors.Join(textErr, endpoint.publications.Close())
}
