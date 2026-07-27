// Package client owns authenticated local-control client calls.
// It does not own command parsing or presentation.
package client

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"ardents/internal/localapi/protocol/ardentsv1connect"

	"connectrpc.com/connect"
)

type Config struct {
	BaseURL           string
	SSH               string
	SSHPort           int
	SSHIdentity       string
	SSHKnownHosts     string
	SSHOperatorSocket string
	Timeout           time.Duration
	ExpectedNode      string
	ExpectedPrincipal string
	Scopes            []string
	Signer            SessionSigner
}

type Client struct {
	service           Service
	sessions          *SessionManager
	close             func() error
	identityPublic    ardentsv1connect.IdentityServiceClient
	identityProtected ardentsv1connect.IdentityServiceClient
	targetNode        string
}

type Service interface {
	ardentsv1connect.NodeServiceClient
	ardentsv1connect.AuthorityServiceClient
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
	ardentsv1connect.AuthorityServiceClient
	ardentsv1connect.ConfigurationServiceClient
	ardentsv1connect.NetworkServiceClient
	ardentsv1connect.WorkloadServiceClient
	ardentsv1connect.ContentServiceClient
	ardentsv1connect.TransferServiceClient
	ardentsv1connect.RetentionServiceClient
	ardentsv1connect.DiagnosticsServiceClient
}

func New(cfg Config) (*Client, error) {
	baseURL, transport, _, closeTransport, err := controlTransport(cfg)
	if err != nil {
		return nil, err
	}
	if cfg.Signer == nil {
		_ = closeTransport()
		return nil, errors.New("Principal signer is required")
	}
	httpClient := &http.Client{
		Timeout: cfg.Timeout,
		Transport: contextTransport{
			base: transport, expectedNode: cfg.ExpectedNode,
			expectedPrincipal: cfg.ExpectedPrincipal, scopes: append([]string(nil), cfg.Scopes...),
		},
	}
	rawIdentity := ardentsv1connect.NewIdentityServiceClient(httpClient, baseURL)
	sessions := NewSessionManager(rawIdentity, cfg.Signer, cfg.ExpectedPrincipal, time.Now)
	interceptor := newSessionInterceptor(sessions)
	service := NewService(httpClient, baseURL, connect.WithInterceptors(interceptor))
	protectedIdentity := ardentsv1connect.NewIdentityServiceClient(httpClient, baseURL, connect.WithInterceptors(interceptor))
	return &Client{service: service, sessions: sessions, close: closeTransport, identityPublic: rawIdentity, identityProtected: protectedIdentity, targetNode: cfg.ExpectedPrincipal}, nil
}

func (c *Client) PublicIdentityService() (ardentsv1connect.IdentityServiceClient, error) {
	if c == nil || c.identityPublic == nil {
		return nil, errors.New("Principal identity service is not configured")
	}
	return c.identityPublic, nil
}

func (c *Client) ProtectedIdentityService() (ardentsv1connect.IdentityServiceClient, error) {
	if c == nil || c.identityProtected == nil {
		return nil, errors.New("Principal identity service is not configured")
	}
	return c.identityProtected, nil
}

func (c *Client) TargetNodePrincipal() string {
	if c == nil {
		return ""
	}
	return c.targetNode
}

func NewService(httpClient connect.HTTPClient, baseURL string, opts ...connect.ClientOption) Service {
	return services{
		NodeServiceClient:          ardentsv1connect.NewNodeServiceClient(httpClient, baseURL, opts...),
		AuthorityServiceClient:     ardentsv1connect.NewAuthorityServiceClient(httpClient, baseURL, opts...),
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

func Request[T any](msg *T) *connect.Request[T] {
	return connect.NewRequest(msg)
}

func (c *Client) Login(ctx context.Context) (SessionKey, error) {
	if c == nil || c.sessions == nil {
		return SessionKey{}, errors.New("Principal session mode is not configured")
	}
	if _, _, err := c.sessions.authorization(ctx); err != nil {
		return SessionKey{}, err
	}
	return c.sessions.Status(), nil
}

func (c *Client) SessionStatus() SessionKey {
	if c == nil || c.sessions == nil {
		return SessionKey{}
	}
	return c.sessions.Status()
}

func (c *Client) Logout() error {
	if c != nil && c.sessions != nil {
		return c.sessions.Logout()
	}
	return nil
}

func (c *Client) Close() error {
	if c == nil {
		return nil
	}
	logoutErr := c.Logout()
	if c.close != nil {
		return errors.Join(logoutErr, c.close())
	}
	return logoutErr
}
