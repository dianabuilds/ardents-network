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
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type CredentialSource interface {
	Credential(context.Context) (string, error)
}

type staticCredential struct{ token string }

type fileCredential struct{ path string }

func StaticCredential(token string) CredentialSource {
	return staticCredential{token: token}
}

func FileCredential(path string) (CredentialSource, error) {
	path = strings.TrimSpace(path)
	if strings.ContainsRune(path, '\x00') || !filepath.IsAbs(path) {
		return nil, fmt.Errorf("application credential path must be absolute")
	}
	return fileCredential{path: path}, nil
}

func (c staticCredential) Credential(context.Context) (string, error) {
	return c.token, nil
}

func (c fileCredential) Credential(context.Context) (string, error) {
	info, err := os.Lstat(c.path)
	if err != nil || !info.Mode().IsRegular() ||
		(runtime.GOOS != "windows" && info.Mode().Perm()&0o027 != 0) {
		return "", fmt.Errorf("application credential is unavailable")
	}
	raw, err := os.ReadFile(c.path)
	if err != nil {
		return "", fmt.Errorf("application credential is unavailable")
	}
	token := strings.TrimSpace(string(raw))
	if token == "" {
		return "", fmt.Errorf("application credential is unavailable")
	}
	return token, nil
}

type Config struct {
	Endpoint   string
	SocketPath string
	Credential CredentialSource
	HTTPClient *http.Client
}

type Client struct {
	Content content.Service
}

func New(config Config) (*Client, error) {
	endpoint := strings.TrimSpace(config.Endpoint)
	socketPath := strings.TrimSpace(config.SocketPath)
	if endpoint != "" && socketPath != "" {
		return nil, fmt.Errorf("configure either application endpoint or socket path")
	}
	if endpoint == "" && socketPath == "" {
		return nil, fmt.Errorf("application endpoint is required")
	}
	if config.Credential == nil {
		return nil, fmt.Errorf("application credential source is required")
	}
	httpClient := config.HTTPClient
	if socketPath != "" {
		var err error
		httpClient, err = unixHTTPClient(socketPath, httpClient)
		if err != nil {
			return nil, err
		}
		endpoint = "http://localhost"
	} else {
		if err := validateEndpoint(endpoint); err != nil {
			return nil, err
		}
		if httpClient == nil {
			httpClient = http.DefaultClient
		}
	}
	credential := func(ctx context.Context) (string, error) { return config.Credential.Credential(ctx) }
	return &Client{Content: adapter.NewContent(httpClient, endpoint, credential)}, nil
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
