package configuration

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	identityprincipal "ardents/internal/identity/principal"
)

const (
	defaultAddr      = "127.0.0.1:8080"
	defaultOutput    = "human"
	defaultTimeout   = 10 * time.Second
	defaultTokenEnv  = "ARDENTS_LEGACY_API_TOKEN"
	contextsFileName = "contexts.json"
)

type Config struct {
	Addr              string
	SSH               string
	SSHPort           int
	SSHIdentity       string
	SSHKnownHosts     string
	SSHOperatorSocket string
	SignerFile        string
	Token             string
	TokenFile         string
	ContextName       string
	ContextFile       string
	ExpectedNode      string
	ExpectedPrincipal string
	ExpectedPublicKey string
	ScopeHints        []string
	Output            string
	Watch             bool
	Interval          time.Duration
	Timeout           time.Duration
	LegacyWarning     bool
}

type ContextFile struct {
	Default  string                   `json:"default,omitempty"`
	Contexts map[string]StoredContext `json:"contexts,omitempty"`
}

type StoredContext struct {
	Addr              string   `json:"addr,omitempty"`
	SSH               string   `json:"ssh,omitempty"`
	SSHPort           int      `json:"ssh_port,omitempty"`
	SSHIdentity       string   `json:"ssh_identity,omitempty"`
	SSHKnownHosts     string   `json:"ssh_known_hosts,omitempty"`
	SSHOperatorSocket string   `json:"ssh_operator_socket,omitempty"`
	SignerFile        string   `json:"signer_file,omitempty"`
	TokenEnv          string   `json:"legacy_token_env,omitempty"`
	TokenFile         string   `json:"legacy_token_file,omitempty"`
	ExpectedNode      string   `json:"expected_node,omitempty"`
	ExpectedPrincipal string   `json:"expected_principal,omitempty"`
	ExpectedPublicKey string   `json:"expected_public_key,omitempty"`
	ScopeHints        []string `json:"scope_hints,omitempty"`
	Timeout           string   `json:"timeout,omitempty"`
}

func DefaultConfig() Config {
	return Config{
		Output:   defaultOutput,
		Interval: time.Second,
		Timeout:  defaultTimeout,
	}
}

func (c *Config) Resolve() error {
	ctxName, err := c.resolveContextName()
	if err != nil {
		return err
	}
	stored, err := c.loadContext(ctxName)
	if err != nil {
		return err
	}
	c.applyStored(stored)
	c.applyEnv()
	if err := c.validateAndNormalize(); err != nil {
		return err
	}
	return nil
}

func (c *Config) resolveContextName() (string, error) {
	if c.ContextName != "" {
		return c.ContextName, nil
	}
	if env := strings.TrimSpace(os.Getenv("ARDENTS_CONTEXT")); env != "" {
		return env, nil
	}
	path, err := c.contextFilePath()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read contexts file: %w", err)
	}
	var store ContextFile
	if err := json.Unmarshal(data, &store); err != nil {
		return "", fmt.Errorf("parse contexts file: %w", err)
	}
	return strings.TrimSpace(store.Default), nil
}

func (c *Config) loadContext(name string) (StoredContext, error) {
	if name == "" {
		return StoredContext{}, nil
	}
	path, err := c.contextFilePath()
	if err != nil {
		return StoredContext{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return StoredContext{}, fmt.Errorf("read contexts file: %w", err)
	}
	var store ContextFile
	if err := json.Unmarshal(data, &store); err != nil {
		return StoredContext{}, fmt.Errorf("parse contexts file: %w", err)
	}
	ctx, ok := store.Contexts[name]
	if !ok {
		return StoredContext{}, fmt.Errorf("context %q not found", name)
	}
	return ctx, nil
}

func (c *Config) contextFilePath() (string, error) {
	if c.ContextFile != "" {
		return c.ContextFile, nil
	}
	if env := strings.TrimSpace(os.Getenv("ARDENTS_CONTEXT_FILE")); env != "" {
		return env, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}
	return filepath.Join(dir, "ardents", contextsFileName), nil
}

func (c *Config) applyStored(stored StoredContext) {
	if c.Addr == "" {
		c.Addr = stored.Addr
	}
	if c.SSH == "" {
		c.SSH = stored.SSH
	}
	if c.SSHPort == 0 {
		c.SSHPort = stored.SSHPort
	}
	if c.SSHIdentity == "" {
		c.SSHIdentity = stored.SSHIdentity
	}
	if c.SSHKnownHosts == "" {
		c.SSHKnownHosts = stored.SSHKnownHosts
	}
	if c.SSHOperatorSocket == "" {
		c.SSHOperatorSocket = stored.SSHOperatorSocket
	}
	if c.SignerFile == "" {
		c.SignerFile = stored.SignerFile
	}
	if c.TokenFile == "" {
		c.TokenFile = stored.TokenFile
	}
	if c.ExpectedPrincipal == "" {
		c.ExpectedPrincipal = stored.ExpectedPrincipal
	}
	if c.ExpectedNode == "" {
		c.ExpectedNode = stored.ExpectedNode
	}
	if c.ExpectedPublicKey == "" {
		c.ExpectedPublicKey = stored.ExpectedPublicKey
	}
	if len(c.ScopeHints) == 0 {
		c.ScopeHints = append([]string(nil), stored.ScopeHints...)
	}
	if c.Timeout == defaultTimeout && stored.Timeout != "" {
		if d, err := time.ParseDuration(stored.Timeout); err == nil {
			c.Timeout = d
		}
	}
	if c.Token == "" && c.TokenFile == "" {
		if envName := strings.TrimSpace(stored.TokenEnv); envName != "" {
			c.Token = strings.TrimSpace(os.Getenv(envName))
		}
	}
}

func (c *Config) applyEnv() {
	if c.Addr == "" {
		c.Addr = strings.TrimSpace(os.Getenv("ARDENTS_ADDR"))
	}
	if c.SSH == "" {
		c.SSH = strings.TrimSpace(os.Getenv("ARDENTS_SSH"))
	}
	if c.SSHPort == 0 {
		if raw := strings.TrimSpace(os.Getenv("ARDENTS_SSH_PORT")); raw != "" {
			port, err := strconv.Atoi(raw)
			if err != nil {
				c.SSHPort = -1
			} else {
				c.SSHPort = port
			}
		}
	}
	if c.SSHIdentity == "" {
		c.SSHIdentity = strings.TrimSpace(os.Getenv("ARDENTS_SSH_IDENTITY"))
	}
	if c.SSHKnownHosts == "" {
		c.SSHKnownHosts = strings.TrimSpace(os.Getenv("ARDENTS_SSH_KNOWN_HOSTS"))
	}
	if c.SSHOperatorSocket == "" {
		c.SSHOperatorSocket = strings.TrimSpace(os.Getenv("ARDENTS_SSH_OPERATOR_SOCKET"))
	}
	if c.SignerFile == "" {
		c.SignerFile = strings.TrimSpace(os.Getenv("ARDENTS_SIGNER_FILE"))
	}
	if c.Token == "" {
		c.Token = strings.TrimSpace(os.Getenv(defaultTokenEnv))
	}
	if c.TokenFile == "" {
		c.TokenFile = strings.TrimSpace(os.Getenv("ARDENTS_LEGACY_TOKEN_FILE"))
	}
	if c.ExpectedPrincipal == "" {
		c.ExpectedPrincipal = strings.TrimSpace(os.Getenv("ARDENTS_EXPECTED_PRINCIPAL"))
	}
	if c.ExpectedNode == "" {
		c.ExpectedNode = strings.TrimSpace(os.Getenv("ARDENTS_EXPECTED_NODE"))
	}
	if c.ExpectedPublicKey == "" {
		c.ExpectedPublicKey = strings.TrimSpace(os.Getenv("ARDENTS_EXPECTED_PUBLIC_KEY"))
	}
	if len(c.ScopeHints) == 0 {
		if raw := strings.TrimSpace(os.Getenv("ARDENTS_SCOPE_HINTS")); raw != "" {
			c.ScopeHints = splitCSV(raw)
		}
	}
	if c.Output == defaultOutput {
		if raw := strings.TrimSpace(os.Getenv("ARDENTS_OUTPUT")); raw != "" {
			c.Output = raw
		}
	}
	if c.Timeout == defaultTimeout {
		if raw := strings.TrimSpace(os.Getenv("ARDENTS_TIMEOUT")); raw != "" {
			if d, err := time.ParseDuration(raw); err == nil {
				c.Timeout = d
			}
		}
	}
}

func (c *Config) validateAndNormalize() error {
	if c.Addr == "" {
		c.Addr = defaultAddr
	}
	if !strings.Contains(c.Addr, "://") {
		scheme := "https"
		if isLoopbackAddress(c.Addr) {
			scheme = "http"
		}
		c.Addr = scheme + "://" + c.Addr
	}
	parsed, err := url.Parse(c.Addr)
	if err != nil || parsed.Scheme == "" {
		return fmt.Errorf("invalid addr %q", c.Addr)
	}
	if parsed.Scheme == "unix" {
		if parsed.Path == "" || parsed.Host != "" {
			return fmt.Errorf("invalid unix socket address %q", c.Addr)
		}
		if c.SSH != "" {
			return fmt.Errorf("--ssh cannot be combined with a unix socket address")
		}
	} else if parsed.Host == "" {
		return fmt.Errorf("invalid addr %q", c.Addr)
	}
	if c.SSH != "" {
		if strings.HasPrefix(c.SSH, "-") || strings.ContainsAny(c.SSH, " \t\r\n") {
			return fmt.Errorf("invalid ssh target %q", c.SSH)
		}
		if c.SSHOperatorSocket != "" {
			if !path.IsAbs(c.SSHOperatorSocket) || strings.ContainsAny(c.SSHOperatorSocket, "\x00:\r\n") {
				return fmt.Errorf("ssh operator socket must be an absolute remote path")
			}
		} else if parsed.Scheme != "http" || !isLoopbackAddress(parsed.Host) {
			return fmt.Errorf("ssh transport requires a loopback HTTP API address")
		}
		if c.SSHPort == 0 {
			c.SSHPort = 22
		}
		if c.SSHPort < 1 || c.SSHPort > 65535 {
			return fmt.Errorf("ssh port must be between 1 and 65535")
		}
	} else if c.SSHPort != 0 || c.SSHIdentity != "" || c.SSHKnownHosts != "" || c.SSHOperatorSocket != "" {
		return fmt.Errorf("ssh transport options require --ssh")
	}
	if c.Token == "" && c.TokenFile != "" {
		token, err := readTokenFile(c.TokenFile)
		if err != nil {
			return err
		}
		c.Token = token
	}
	principalTransport := parsed.Scheme == "unix" || c.SSHOperatorSocket != ""
	if c.Token == "" && c.SignerFile == "" && principalTransport {
		dir, err := os.UserConfigDir()
		if err != nil {
			return fmt.Errorf("resolve signer location: %w", err)
		}
		c.SignerFile = filepath.Join(dir, "ardents", "identity", "device-v1.json")
	}
	if c.Token != "" && c.SignerFile != "" {
		return fmt.Errorf("Principal signer and legacy bearer cannot be configured together")
	}
	if c.SignerFile != "" {
		if !principalTransport {
			return fmt.Errorf("Principal sessions require a protected Unix socket or SSH stream-local forwarding")
		}
		if strings.TrimSpace(c.ExpectedPrincipal) == "" {
			return fmt.Errorf("Principal sessions require the target Node Principal via --principal or context")
		}
		if _, err := identityprincipal.Parse(c.ExpectedPrincipal); err != nil {
			return fmt.Errorf("target Node Principal is invalid")
		}
	} else if c.Token == "" {
		return fmt.Errorf("authentication requires a Principal signer on a protected transport or an explicit legacy bearer")
	}
	if c.AuthMode() == AuthModeLegacy {
		c.LegacyWarning = true
	}
	switch c.Output {
	case "human", "json":
	default:
		return fmt.Errorf("unsupported output %q", c.Output)
	}
	if c.Timeout <= 0 {
		return fmt.Errorf("timeout must be greater than zero")
	}
	if c.Interval <= 0 {
		c.Interval = time.Second
	}
	c.ScopeHints = compactStrings(c.ScopeHints)
	return nil
}

type AuthMode string

const (
	AuthModeNone      AuthMode = ""
	AuthModePrincipal AuthMode = "principal"
	AuthModeLegacy    AuthMode = "legacy"
)

func (c Config) AuthMode() AuthMode {
	if strings.TrimSpace(c.SignerFile) != "" && strings.TrimSpace(c.Token) == "" {
		return AuthModePrincipal
	}
	if strings.TrimSpace(c.Token) != "" && strings.TrimSpace(c.SignerFile) == "" {
		return AuthModeLegacy
	}
	return AuthModeNone
}

func (c *Config) HasIdentityBinding() bool {
	return strings.TrimSpace(c.ExpectedNode) != "" || strings.TrimSpace(c.ExpectedPrincipal) != "" ||
		strings.TrimSpace(c.ExpectedPublicKey) != ""
}

func isLoopbackAddress(addr string) bool {
	parsed, err := url.Parse("//" + addr)
	if err != nil {
		return false
	}
	host := parsed.Hostname()
	return strings.EqualFold(host, "localhost") || net.ParseIP(host).IsLoopback()
}

func readTokenFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read token file: %w", err)
	}
	token := strings.TrimSpace(string(data))
	if token == "" {
		return "", fmt.Errorf("token file %q is empty", path)
	}
	return token, nil
}

func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	return compactStrings(parts)
}

func compactStrings(in []string) []string {
	out := make([]string, 0, len(in))
	for _, item := range in {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}
