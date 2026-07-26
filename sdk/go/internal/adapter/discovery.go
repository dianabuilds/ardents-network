package adapter

import (
	"context"
	"fmt"
	"net/netip"
	"net/url"
	"strconv"
	"strings"

	identitycontract "ardents/api/ardents/identity/v1"
	"ardents/sdk/go/discovery"
	sdkerrors "ardents/sdk/go/errors"
	applicationv1 "ardents/sdk/go/protocol/applicationv1"
	applicationv1connect "ardents/sdk/go/protocol/applicationv1/applicationv1connect"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"
)

type Discovery struct {
	client applicationv1connect.DiscoveryServiceClient
}

func NewDiscovery(httpClient connect.HTTPClient, endpoint string, options ...connect.ClientOption) *Discovery {
	clientOptions := []connect.ClientOption{
		connect.WithReadMaxBytes(applicationv1.MaxUnaryMessageBytes),
		connect.WithSendMaxBytes(applicationv1.MaxUnaryMessageBytes),
	}
	clientOptions = append(clientOptions, options...)
	// Keep unknown wire fields observable to the strict response validator even
	// if an internal caller supplies a lossy JSON codec option.
	clientOptions = append(clientOptions, connect.WithCodec(strictDiscoveryProtoCodec{}))
	return &Discovery{
		client: applicationv1connect.NewDiscoveryServiceClient(
			httpClient, strings.TrimRight(endpoint, "/"), clientOptions...,
		),
	}
}

type strictDiscoveryProtoCodec struct{}

func (strictDiscoveryProtoCodec) Name() string {
	return "proto"
}

func (strictDiscoveryProtoCodec) Marshal(message any) ([]byte, error) {
	protoMessage, ok := message.(proto.Message)
	if !ok {
		return nil, fmt.Errorf("discovery protobuf codec requires a protobuf message, got %T", message)
	}
	return proto.Marshal(protoMessage)
}

func (strictDiscoveryProtoCodec) Unmarshal(data []byte, message any) error {
	protoMessage, ok := message.(proto.Message)
	if !ok {
		return fmt.Errorf("discovery protobuf codec requires a protobuf message, got %T", message)
	}
	return proto.UnmarshalOptions{DiscardUnknown: false}.Unmarshal(data, protoMessage)
}

func (d *Discovery) Resolve(ctx context.Context, query discovery.Query) ([]discovery.Target, error) {
	schemes := make([]string, 0, len(query.AcceptedSchemes))
	for _, scheme := range query.AcceptedSchemes {
		schemes = append(schemes, string(scheme))
	}
	response, err := d.client.Resolve(ctx, connect.NewRequest(&applicationv1.ResolveServiceRequest{
		ServiceType: query.ServiceType, AcceptedSchemes: schemes,
	}))
	if err != nil {
		return nil, mapError(err)
	}
	if response == nil || response.Msg == nil {
		return nil, invalidDiscoveryResponse()
	}
	wireTargets := response.Msg.GetTargets()
	schemeOrder, ok := acceptedDiscoverySchemeOrder(schemes)
	if len(response.Msg.ProtoReflect().GetUnknown()) != 0 ||
		!validDiscoveryResourceID(query.ServiceType) || !ok ||
		!identitycontract.ValidApplicationDiscoveryTargetCount(len(wireTargets)) {
		return nil, invalidDiscoveryResponse()
	}
	targets := make([]discovery.Target, 0, len(wireTargets))
	seen := make(map[discoveryTargetKey]struct{}, len(wireTargets))
	var previous *applicationv1.ResolvedServiceTarget
	for _, target := range wireTargets {
		if target == nil || len(target.ProtoReflect().GetUnknown()) != 0 ||
			!validDiscoveryResourceID(target.GetServiceId()) {
			return nil, invalidDiscoveryResponse()
		}
		endpointScheme, endpointOK := validDiscoveryDirectEndpoint(target.GetEndpoint())
		rank, accepted := schemeOrder[target.GetScheme()]
		if !endpointOK || !accepted || endpointScheme != target.GetScheme() {
			return nil, invalidDiscoveryResponse()
		}
		key := discoveryTargetKey{serviceID: target.GetServiceId(), endpoint: target.GetEndpoint()}
		if _, duplicate := seen[key]; duplicate {
			return nil, invalidDiscoveryResponse()
		}
		if previous != nil && !discoveryTargetFollows(previous, target, schemeOrder, rank) {
			return nil, invalidDiscoveryResponse()
		}
		seen[key] = struct{}{}
		targets = append(targets, discovery.Target{
			ServiceID: target.GetServiceId(), Endpoint: target.GetEndpoint(), Scheme: discovery.Scheme(target.GetScheme()),
		})
		previous = target
	}
	return targets, nil
}

type discoveryTargetKey struct {
	serviceID string
	endpoint  string
}

func acceptedDiscoverySchemeOrder(schemes []string) (map[string]int, bool) {
	if !identitycontract.ValidApplicationDiscoverySchemeCount(len(schemes)) {
		return nil, false
	}
	order := make(map[string]int, len(schemes))
	for index, scheme := range schemes {
		if !identitycontract.IsApplicationDiscoveryScheme(scheme) {
			return nil, false
		}
		if _, duplicate := order[scheme]; duplicate {
			return nil, false
		}
		order[scheme] = index
	}
	return order, true
}

func discoveryTargetFollows(
	previous *applicationv1.ResolvedServiceTarget,
	current *applicationv1.ResolvedServiceTarget,
	schemeOrder map[string]int,
	currentRank int,
) bool {
	previousRank, ok := schemeOrder[previous.GetScheme()]
	if !ok || previousRank > currentRank {
		return false
	}
	if previousRank < currentRank {
		return true
	}
	if previous.GetServiceId() > current.GetServiceId() {
		return false
	}
	if previous.GetServiceId() < current.GetServiceId() {
		return true
	}
	return previous.GetEndpoint() < current.GetEndpoint()
}

func validDiscoveryResourceID(value string) bool {
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

func validDiscoveryDirectEndpoint(endpoint string) (string, bool) {
	if !validDiscoveryResourceID(endpoint) || strings.Contains(endpoint, "#") {
		return "", false
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Opaque != "" || parsed.User != nil || parsed.Host == "" ||
		!identitycontract.IsApplicationDiscoveryScheme(parsed.Scheme) {
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

func invalidDiscoveryResponse() error {
	return &sdkerrors.Error{
		Code: sdkerrors.Internal, Operation: "application.discovery.resolve",
		Message: "application discovery response is invalid",
	}
}
