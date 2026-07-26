// Package discovery adapts maintained Discovery truth, current trust, and
// route policy to the bounded Application locator contract. It does not own
// those inputs or perform observation, refresh, probing, fetching, or dialing.
package discovery

import (
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"sort"
	"strconv"
	"strings"

	identitycontract "ardents/api/ardents/identity/v1"
	discoverytruth "ardents/internal/discovery"
	"ardents/internal/network"
)

const (
	ActionResolve = "application.discovery.resolve"

	maxProjectionRecords   = identitycontract.MaxApplicationDiscoveryTargets * 8
	maxProjectionEndpoints = identitycontract.MaxApplicationDiscoveryTargets * 32
)

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

type targetKey struct {
	serviceID string
	endpoint  string
}

type Truth interface {
	FindService(string) ([]discoverytruth.Entry, error)
	Evaluate(discoverytruth.Record) (discoverytruth.TrustResult, error)
	AllowRouteUse(network.Candidate) error
}

type RoutePolicy interface {
	AllowRouteUse(network.Candidate) error
}

type Locator struct {
	truth Truth
}

type MaintainedTruth struct {
	store  *discoverytruth.Service
	trust  *discoverytruth.TrustEvaluator
	policy RoutePolicy
}

func NewMaintainedTruth(
	store *discoverytruth.Service,
	trust *discoverytruth.TrustEvaluator,
	policy RoutePolicy,
) (*MaintainedTruth, error) {
	if store == nil || trust == nil || policy == nil {
		return nil, fmt.Errorf("maintained discovery truth, trust, and route policy are required")
	}
	return &MaintainedTruth{store: store, trust: trust, policy: policy}, nil
}

func (t *MaintainedTruth) FindService(serviceType string) ([]discoverytruth.Entry, error) {
	if t == nil || t.store == nil || t.trust == nil || t.store.State() == "new" {
		return nil, ErrUnavailable
	}
	entries, overflow := t.store.FindServiceBounded(
		serviceType, maxProjectionRecords, maxProjectionEndpoints,
	)
	if overflow {
		return nil, ErrInternal
	}
	return entries, nil
}

func (t *MaintainedTruth) Evaluate(record discoverytruth.Record) (discoverytruth.TrustResult, error) {
	if t == nil || t.trust == nil || t.policy == nil {
		return discoverytruth.TrustResult{}, ErrUnavailable
	}
	return t.trust.Evaluate(record), nil
}

func (t *MaintainedTruth) AllowRouteUse(candidate network.Candidate) error {
	if t == nil || t.store == nil || t.trust == nil || t.policy == nil {
		return ErrUnavailable
	}
	return t.policy.AllowRouteUse(candidate)
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
	if projectionWorkExceedsBudget(entries) {
		return nil, ErrInternal
	}
	schemeOrder := make(map[string]int, len(query.AcceptedSchemes))
	for index, scheme := range query.AcceptedSchemes {
		schemeOrder[scheme] = index
	}
	targets := make([]Target, 0, identitycontract.MaxApplicationDiscoveryTargets)
	for _, entry := range entries {
		record := entry.Record
		trust, trustErr := l.truth.Evaluate(record)
		if trustErr != nil {
			return nil, classifyTruthError(trustErr)
		}
		if record.ServiceMode() != "NetworkPublished" ||
			!trust.Valid || !trust.Trusted || !trust.Usable {
			continue
		}
		serviceID := record.Subject()
		if !validCanonicalID(serviceID) {
			return nil, ErrInternal
		}
		for _, endpoint := range record.EndpointList() {
			endpointScheme, ok := eligibleDirectEndpoint(endpoint)
			if !ok {
				continue
			}
			if _, accepted := schemeOrder[endpointScheme]; !accepted {
				continue
			}
			if policyErr := l.truth.AllowRouteUse(network.Candidate{
				Subject: serviceID, Service: record.ServiceType(), Endpoint: endpoint,
				Scheme: endpointScheme, Mode: record.ServiceMode(), Trusted: trust.Trusted, Usable: true,
			}); policyErr != nil {
				if errors.Is(policyErr, ErrUnavailable) {
					return nil, ErrUnavailable
				}
				continue
			}
			targets = insertBoundedTarget(targets, Target{
				ServiceID: serviceID, Endpoint: endpoint, Scheme: endpointScheme,
			}, schemeOrder)
		}
	}
	if len(targets) == 0 {
		return nil, ErrNotFound
	}
	return targets, nil
}

func projectionWorkExceedsBudget(entries []discoverytruth.Entry) bool {
	if len(entries) > maxProjectionRecords {
		return true
	}
	endpointCount := 0
	for _, entry := range entries {
		endpoints := entry.Record.EndpointList()
		if len(endpoints) > maxProjectionEndpoints-endpointCount {
			return true
		}
		endpointCount += len(endpoints)
	}
	return false
}

func insertBoundedTarget(targets []Target, candidate Target, schemeOrder map[string]int) []Target {
	for _, target := range targets {
		if target.ServiceID == candidate.ServiceID && target.Endpoint == candidate.Endpoint {
			return targets
		}
	}
	index := sort.Search(len(targets), func(index int) bool {
		return targetLess(candidate, targets[index], schemeOrder)
	})
	if len(targets) == identitycontract.MaxApplicationDiscoveryTargets &&
		index == len(targets) {
		return targets
	}
	if len(targets) < identitycontract.MaxApplicationDiscoveryTargets {
		targets = append(targets, Target{})
	}
	copy(targets[index+1:], targets[index:len(targets)-1])
	targets[index] = candidate
	return targets
}

func targetLess(left, right Target, schemeOrder map[string]int) bool {
	if schemeOrder[left.Scheme] != schemeOrder[right.Scheme] {
		return schemeOrder[left.Scheme] < schemeOrder[right.Scheme]
	}
	if left.ServiceID != right.ServiceID {
		return left.ServiceID < right.ServiceID
	}
	return left.Endpoint < right.Endpoint
}

func eligibleDirectEndpoint(endpoint string) (string, bool) {
	if !validCanonicalID(endpoint) || strings.Contains(endpoint, "#") {
		return "", false
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Opaque != "" || parsed.User != nil || parsed.Host == "" {
		return "", false
	}
	if !identitycontract.IsApplicationDiscoveryScheme(parsed.Scheme) {
		return "", false
	}
	portText := parsed.Port()
	if portText == "" {
		return "", false
	}
	for _, current := range portText {
		if current < '0' || current > '9' {
			return "", false
		}
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return "", false
	}
	host, err := netip.ParseAddr(parsed.Hostname())
	if err != nil {
		return "", false
	}
	host = host.Unmap()
	if host.IsUnspecified() || host.IsLoopback() {
		return "", false
	}
	return parsed.Scheme, true
}

func validTargetSet(query Query, targets []Target) bool {
	if !validQuery(query) || !identitycontract.ValidApplicationDiscoveryTargetCount(len(targets)) {
		return false
	}
	schemeOrder := make(map[string]int, len(query.AcceptedSchemes))
	for index, scheme := range query.AcceptedSchemes {
		schemeOrder[scheme] = index
	}
	seen := make(map[targetKey]struct{}, len(targets))
	for index, target := range targets {
		if !validCanonicalID(target.ServiceID) {
			return false
		}
		endpointScheme, endpointOK := eligibleDirectEndpoint(target.Endpoint)
		rank, accepted := schemeOrder[target.Scheme]
		if !endpointOK || !accepted || endpointScheme != target.Scheme {
			return false
		}
		key := targetKey{serviceID: target.ServiceID, endpoint: target.Endpoint}
		if _, duplicate := seen[key]; duplicate {
			return false
		}
		if index > 0 && !targetFollows(targets[index-1], target, schemeOrder, rank) {
			return false
		}
		seen[key] = struct{}{}
	}
	return true
}

func targetFollows(previous, current Target, schemeOrder map[string]int, currentRank int) bool {
	previousRank, ok := schemeOrder[previous.Scheme]
	if !ok || previousRank > currentRank {
		return false
	}
	return targetLess(previous, current, schemeOrder)
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
