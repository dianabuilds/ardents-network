// Package client constructs a typed Ardents Application SDK client.
package client

import (
	"ardents/sdk/go/content"
	"ardents/sdk/go/internal/adapter"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
)

type CredentialSource interface {
	Credential(context.Context) (string, error)
}

type staticCredential struct{ token string }

func StaticCredential(token string) CredentialSource {
	return staticCredential{token: token}
}

func (c staticCredential) Credential(context.Context) (string, error) {
	return c.token, nil
}

type Config struct {
	Endpoint   string
	Credential CredentialSource
	HTTPClient *http.Client
}

type Client struct {
	Content content.Service
}

func New(config Config) (*Client, error) {
	endpoint := strings.TrimSpace(config.Endpoint)
	if endpoint == "" {
		return nil, fmt.Errorf("application endpoint is required")
	}
	if err := validateEndpoint(endpoint); err != nil {
		return nil, err
	}
	if config.Credential == nil {
		return nil, fmt.Errorf("application credential source is required")
	}
	httpClient := config.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	credential := func(ctx context.Context) (string, error) { return config.Credential.Credential(ctx) }
	return &Client{Content: adapter.NewContent(httpClient, endpoint, credential)}, nil
}

func validateEndpoint(endpoint string) error {
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Host == "" {
		return fmt.Errorf("application endpoint is invalid")
	}
	if parsed.Scheme == "https" {
		return nil
	}
	if parsed.Scheme != "http" {
		return fmt.Errorf("application endpoint scheme is unsupported")
	}
	host := parsed.Hostname()
	if strings.EqualFold(host, "localhost") {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("plaintext application endpoint must be loopback")
	}
	return nil
}
