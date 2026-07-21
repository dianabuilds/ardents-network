// Package client owns authenticated local-control client calls.
// It does not own command parsing or presentation.
package client

import (
	"net/http"
	"strings"
	"time"

	"ardents/internal/localapi/protocol/ardentsv1connect"

	"connectrpc.com/connect"
)

type Config struct {
	BaseURL           string
	Token             string
	Timeout           time.Duration
	ExpectedNode      string
	ExpectedPrincipal string
	Scopes            []string
}

type Client struct {
	service Service
	token   string
}

type Service interface {
	ardentsv1connect.NodeServiceClient
	ardentsv1connect.ConfigurationServiceClient
	ardentsv1connect.NetworkServiceClient
	ardentsv1connect.WorkloadServiceClient
	ardentsv1connect.ContentServiceClient
	ardentsv1connect.TransferServiceClient
	ardentsv1connect.RetentionServiceClient
	ardentsv1connect.DiagnosticsServiceClient
}

type services struct {
	ardentsv1connect.NodeServiceClient
	ardentsv1connect.ConfigurationServiceClient
	ardentsv1connect.NetworkServiceClient
	ardentsv1connect.WorkloadServiceClient
	ardentsv1connect.ContentServiceClient
	ardentsv1connect.TransferServiceClient
	ardentsv1connect.RetentionServiceClient
	ardentsv1connect.DiagnosticsServiceClient
}

func New(cfg Config) *Client {
	httpClient := &http.Client{
		Timeout: cfg.Timeout,
		Transport: contextTransport{
			base: http.DefaultTransport, expectedNode: cfg.ExpectedNode,
			expectedPrincipal: cfg.ExpectedPrincipal, scopes: append([]string(nil), cfg.Scopes...),
		},
	}
	service := NewService(httpClient, cfg.BaseURL)
	return &Client{service: service, token: cfg.Token}
}

func NewService(httpClient connect.HTTPClient, baseURL string, opts ...connect.ClientOption) Service {
	return services{
		NodeServiceClient:          ardentsv1connect.NewNodeServiceClient(httpClient, baseURL, opts...),
		ConfigurationServiceClient: ardentsv1connect.NewConfigurationServiceClient(httpClient, baseURL, opts...),
		NetworkServiceClient:       ardentsv1connect.NewNetworkServiceClient(httpClient, baseURL, opts...),
		WorkloadServiceClient:      ardentsv1connect.NewWorkloadServiceClient(httpClient, baseURL, opts...),
		ContentServiceClient:       ardentsv1connect.NewContentServiceClient(httpClient, baseURL, opts...),
		TransferServiceClient:      ardentsv1connect.NewTransferServiceClient(httpClient, baseURL, opts...),
		RetentionServiceClient:     ardentsv1connect.NewRetentionServiceClient(httpClient, baseURL, opts...),
		DiagnosticsServiceClient:   ardentsv1connect.NewDiagnosticsServiceClient(httpClient, baseURL, opts...),
	}
}

type contextTransport struct {
	base              http.RoundTripper
	expectedNode      string
	expectedPrincipal string
	scopes            []string
}

func (t contextTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	clonedRequest := request.Clone(request.Context())
	clonedRequest.Header = request.Header.Clone()
	clonedRequest.Header.Set("Ardents-Expected-Node", strings.TrimSpace(t.expectedNode))
	clonedRequest.Header.Set("Ardents-Expected-Principal", strings.TrimSpace(t.expectedPrincipal))
	clonedRequest.Header.Set("Ardents-Scopes", strings.Join(t.scopes, ","))
	return t.base.RoundTrip(clonedRequest)
}

func (c *Client) Service() Service {
	return c.service
}

func Request[T any](token string, msg *T) *connect.Request[T] {
	req := connect.NewRequest(msg)
	req.Header().Set("Authorization", "Bearer "+token)
	return req
}
