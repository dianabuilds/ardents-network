package adapter

import (
	"context"
	"strings"

	identitycontract "ardents/api/ardents/identity/v1"
	"ardents/sdk/go/discovery"
	sdkerrors "ardents/sdk/go/errors"
	applicationv1 "ardents/sdk/go/protocol/applicationv1"
	applicationv1connect "ardents/sdk/go/protocol/applicationv1/applicationv1connect"

	"connectrpc.com/connect"
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
	return &Discovery{
		client: applicationv1connect.NewDiscoveryServiceClient(
			httpClient, strings.TrimRight(endpoint, "/"), clientOptions...,
		),
	}
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
	wireTargets := response.Msg.GetTargets()
	if !identitycontract.ValidApplicationDiscoveryTargetCount(len(wireTargets)) {
		return nil, invalidDiscoveryResponse()
	}
	targets := make([]discovery.Target, 0, len(wireTargets))
	for _, target := range wireTargets {
		if target == nil || target.GetServiceId() == "" || target.GetEndpoint() == "" ||
			!identitycontract.IsApplicationDiscoveryScheme(target.GetScheme()) {
			return nil, invalidDiscoveryResponse()
		}
		targets = append(targets, discovery.Target{
			ServiceID: target.GetServiceId(), Endpoint: target.GetEndpoint(), Scheme: discovery.Scheme(target.GetScheme()),
		})
	}
	return targets, nil
}

func invalidDiscoveryResponse() error {
	return &sdkerrors.Error{
		Code: sdkerrors.Internal, Operation: "application.discovery.resolve",
		Message: "application discovery response is invalid",
	}
}
