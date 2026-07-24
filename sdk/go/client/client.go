// Package client constructs a typed Ardents Application SDK client.
// It does not own Application admission or server-side product state.
package client

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"runtime"
	"strings"

	"ardents/sdk/go/content"
	sdkidentity "ardents/sdk/go/identity"
	"ardents/sdk/go/internal/adapter"

	"connectrpc.com/connect"
)

type Config struct {
	SocketPath    string
	Signer        SessionSigner
	NodePrincipal string
	Delegation    *sdkidentity.Artifact
	HTTPClient    *http.Client
}

type Client struct {
	Content content.Service
	Session SessionProvider
}

func New(config Config) (*Client, error) {
	socketPath := strings.TrimSpace(config.SocketPath)
	if socketPath == "" {
		return nil, fmt.Errorf("Application Principal sessions require a protected Unix socket")
	}
	if config.Signer == nil {
		return nil, fmt.Errorf("Application Principal session signer is required")
	}
	node := config.NodePrincipal
	if !adapter.ValidPrincipalID(node) {
		return nil, fmt.Errorf("Application Principal sessions require a canonical Node Principal")
	}
	httpClient, err := unixHTTPClient(socketPath, config.HTTPClient)
	if err != nil {
		return nil, err
	}
	const endpoint = "http://localhost"
	manager := adapter.NewSessionManager(httpClient, endpoint, config.Signer, node, nil)
	interceptor := adapter.NewSessionInterceptor(manager)
	if config.Delegation != nil {
		interceptor, err = adapter.NewSessionInterceptorWithDelegation(manager, config.Delegation)
		if err != nil {
			return nil, fmt.Errorf("Application Delegation configuration is invalid")
		}
	}
	return &Client{
		Content: adapter.NewContent(httpClient, endpoint, connect.WithInterceptors(interceptor)),
		Session: &sessionProvider{manager: manager},
	}, nil
}

func unixHTTPClient(path string, configured *http.Client) (*http.Client, error) {
	if runtime.GOOS == "windows" {
		return nil, fmt.Errorf("application Unix sockets are unavailable on Windows")
	}
	if strings.ContainsRune(path, '\x00') || !filepath.IsAbs(path) {
		return nil, fmt.Errorf("application socket path must be absolute")
	}
	client := &http.Client{}
	if configured != nil {
		*client = *configured
	}
	var transport *http.Transport
	switch configuredTransport := client.Transport.(type) {
	case nil:
		transport = http.DefaultTransport.(*http.Transport).Clone()
	case *http.Transport:
		transport = configuredTransport.Clone()
	default:
		return nil, fmt.Errorf("application Unix socket requires an HTTP transport")
	}
	dialer := &net.Dialer{}
	transport.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
		return dialer.DialContext(ctx, "unix", path)
	}
	client.Transport = transport
	return client, nil
}
