// Package discovery adapts maintained Discovery truth to the bounded
// Application locator contract. It does not own Discovery records, trust
// state, observation, refresh, probing, fetching, or dialing.
package discovery

import (
	"errors"
	"fmt"
	"net/url"

	identitycontract "ardents/api/ardents/identity/v1"
	discoverytruth "ardents/internal/discovery"
)

const ActionResolve = "application.discovery.resolve"

var (
	ErrInvalidArgument = errors.New("application discovery query is invalid")
	ErrNotFound        = errors.New("application discovery target was not found")
	ErrUnavailable     = errors.New("application discovery truth is unavailable")
	ErrInternal        = errors.New("application discovery invariant failed")
)

type Query struct {
	ServiceType     string
	AcceptedSchemes []string
}

type Target struct {
	ServiceID string
	Endpoint  string
	Scheme    string
}

type Truth interface {
	FindService(string) ([]discoverytruth.Entry, error)
	Evaluate(discoverytruth.Record) (discoverytruth.TrustResult, error)
}

type Locator struct {
	truth Truth
}

type MaintainedTruth struct {
	store *discoverytruth.Service
	trust *discoverytruth.TrustEvaluator
}

func NewMaintainedTruth(store *discoverytruth.Service, trust *discoverytruth.TrustEvaluator) (*MaintainedTruth, error) {
	if store == nil || trust == nil {
		return nil, fmt.Errorf("maintained discovery truth is required")
	}
	return &MaintainedTruth{store: store, trust: trust}, nil
}

func (t *MaintainedTruth) FindService(serviceType string) ([]discoverytruth.Entry, error) {
	if t == nil || t.store == nil || t.trust == nil || t.store.State() == "new" {
		return nil, ErrUnavailable
	}
	return t.store.FindService(serviceType), nil
}

func (t *MaintainedTruth) Evaluate(record discoverytruth.Record) (discoverytruth.TrustResult, error) {
	if t == nil || t.trust == nil {
		return discoverytruth.TrustResult{}, ErrUnavailable
	}
	return t.trust.Evaluate(record), nil
}

func NewLocator(truth Truth) (*Locator, error) {
	if truth == nil {
		return nil, fmt.Errorf("application discovery truth is required")
	}
	return &Locator{truth: truth}, nil
}

func (l *Locator) Resolve(query Query) ([]Target, error) {
	if l == nil || l.truth == nil {
		return nil, ErrUnavailable
	}
	if !validQuery(query) {
		return nil, ErrInvalidArgument
	}
	entries, err := l.truth.FindService(query.ServiceType)
	if err != nil {
		return nil, classifyTruthError(err)
	}
	for _, scheme := range query.AcceptedSchemes {
		for _, entry := range entries {
			record := entry.Record
			trust, trustErr := l.truth.Evaluate(record)
			if trustErr != nil {
				return nil, classifyTruthError(trustErr)
			}
			if record.ServiceMode() != "NetworkPublished" || !trust.Usable {
				continue
			}
			for _, endpoint := range record.EndpointList() {
				parsed, err := url.Parse(endpoint)
				if err != nil || parsed.Scheme != scheme {
					continue
				}
				serviceID := record.Subject()
				if serviceID == "" || endpoint == "" {
					return nil, ErrInternal
				}
				return []Target{{ServiceID: serviceID, Endpoint: endpoint, Scheme: scheme}}, nil
			}
		}
	}
	return nil, ErrNotFound
}

func validQuery(query Query) bool {
	if !validCanonicalID(query.ServiceType) ||
		!identitycontract.ValidApplicationDiscoverySchemeCount(len(query.AcceptedSchemes)) {
		return false
	}
	seen := make(map[string]struct{}, len(query.AcceptedSchemes))
	for _, scheme := range query.AcceptedSchemes {
		if !identitycontract.IsApplicationDiscoveryScheme(scheme) {
			return false
		}
		if _, duplicate := seen[scheme]; duplicate {
			return false
		}
		seen[scheme] = struct{}{}
	}
	return true
}

func classifyTruthError(err error) error {
	if errors.Is(err, ErrUnavailable) {
		return ErrUnavailable
	}
	return ErrInternal
}

func validCanonicalID(value string) bool {
	if !identitycontract.ValidCanonicalResourceIDSize(len(value)) {
		return false
	}
	for _, current := range value {
		if current < 0x21 || current > 0x7e {
			return false
		}
	}
	return true
}
