package client

import (
	"net/http"
	"strings"
	"time"

	"ardents/proto/ardents/v1/ardentsv1connect"

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
	service ardentsv1connect.ArdentsServiceClient
	token   string
}

func New(cfg Config) *Client {
	httpClient := &http.Client{
		Timeout: cfg.Timeout,
		Transport: contextTransport{
			base: http.DefaultTransport, expectedNode: cfg.ExpectedNode,
			expectedPrincipal: cfg.ExpectedPrincipal, scopes: append([]string(nil), cfg.Scopes...),
		},
	}
	service := ardentsv1connect.NewArdentsServiceClient(httpClient, cfg.BaseURL)
	return &Client{service: service, token: cfg.Token}
}

type contextTransport struct {
	base              http.RoundTripper
	expectedNode      string
	expectedPrincipal string
	scopes            []string
}

func (t contextTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	copy := request.Clone(request.Context())
	copy.Header = request.Header.Clone()
	copy.Header.Set("Ardents-Expected-Node", strings.TrimSpace(t.expectedNode))
	copy.Header.Set("Ardents-Expected-Principal", strings.TrimSpace(t.expectedPrincipal))
	copy.Header.Set("Ardents-Scopes", strings.Join(t.scopes, ","))
	return t.base.RoundTrip(copy)
}

func (c *Client) Service() ardentsv1connect.ArdentsServiceClient {
	return c.service
}

func Request[T any](token string, msg *T) *connect.Request[T] {
	req := connect.NewRequest(msg)
	req.Header().Set("Authorization", "Bearer "+token)
	return req
}
