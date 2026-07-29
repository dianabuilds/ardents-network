package configuration

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	runtimeconfig "ardents/internal/config"
	identityprincipal "ardents/internal/identity/principal"
)

const (
	defaultAddr      = "unix:///run/ardents/control.sock"
	defaultOutput    = "human"
	defaultTimeout   = 10 * time.Second
	contextsFileName = "contexts.json"
	maxContextsBytes = 1 << 20
)

type Config struct {
	Addr                    string
	SSH                     string
	SSHPort                 int
	SSHIdentity             string
	SSHKnownHosts           string
	SSHOperatorSocket       string
	SignerFile              string
	SignerAlias             string
	HostKeyPinRef           string
	ContextName             string
	ContextFile             string
	ExpectedNode            string
	ExpectedPrincipal       string
	ExpectedPublicKey       string
	ExpectedRealm           string
	AuthorityStateRef       string
	AuthorityBackupRef      string
	CheckpointRepositoryRef string
	ScopeHints              []string
	Output                  string
	Watch                   bool
	Interval                time.Duration
	Timeout                 time.Duration
}

type ContextFile struct {
	Default  string                   `json:"default,omitempty"`
	Contexts map[string]StoredContext `json:"contexts,omitempty"`
}

type StoredContext struct {
	Addr                    string   `json:"addr,omitempty"`
	SSH                     string   `json:"ssh,omitempty"`
	SSHPort                 int      `json:"ssh_port,omitempty"`
	SSHIdentity             string   `json:"ssh_identity,omitempty"`
	SSHKnownHosts           string   `json:"ssh_known_hosts,omitempty"`
	SSHOperatorSocket       string   `json:"ssh_operator_socket,omitempty"`
	SignerFile              string   `json:"signer_file,omitempty"`
	SignerAlias             string   `json:"signer_alias,omitempty"`
	HostKeyPinRef           string   `json:"host_key_pin_ref,omitempty"`
	ExpectedNode            string   `json:"expected_node,omitempty"`
	ExpectedPrincipal       string   `json:"expected_principal,omitempty"`
	ExpectedPublicKey       string   `json:"expected_public_key,omitempty"`
	ExpectedRealm           string   `json:"expected_realm,omitempty"`
	AuthorityStateRef       string   `json:"authority_state_ref,omitempty"`
	AuthorityBackupRef      string   `json:"authority_backup_ref,omitempty"`
	CheckpointRepositoryRef string   `json:"checkpoint_repository_ref,omitempty"`
	ScopeHints              []string `json:"scope_hints,omitempty"`
	Timeout                 string   `json:"timeout,omitempty"`
}

func DefaultConfig() Config {
	return Config{
		Addr:     defaultAddr,
		Output:   defaultOutput,
		Interval: time.Second,
		Timeout:  defaultTimeout,
	}
}

func (c *Config) Resolve() error {
	if err := runtimeconfig.RejectObsoleteCredentialEnvironment(); err != nil {
		return err
	}
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

// ResolveTopologyContext resolves one manifest-selected context without
// process environment transport overrides. This keeps the manifest aliases
// bound to the reviewed SSH pin and signer records.
func (c Config) ResolveTopologyContext(name string) (Config, error) {
	if err := runtimeconfig.RejectObsoleteCredentialEnvironment(); err != nil {
		return Config{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return Config{}, errors.New("topology context name is required")
	}
	stored, err := c.loadContext(name)
	if err != nil {
		return Config{}, err
	}
	contextPath, err := c.contextFilePath()
	if err != nil {
		return Config{}, err
	}
	resolved := DefaultConfig()
	resolved.ContextFile = c.ContextFile
	resolved.ContextName = name
	if c.Output != "" {
		resolved.Output = c.Output
	}
	if c.Timeout > 0 {
		resolved.Timeout = c.Timeout
	}
	resolved.applyStored(stored)
	resolved.SSHIdentity = resolveContextLocalPath(contextPath, resolved.SSHIdentity)
	resolved.SSHKnownHosts = resolveContextLocalPath(contextPath, resolved.SSHKnownHosts)
	resolved.SignerFile = resolveContextLocalPath(contextPath, resolved.SignerFile)
	if strings.TrimSpace(resolved.SSH) == "" ||
		strings.TrimSpace(resolved.SSHKnownHosts) == "" ||
		strings.TrimSpace(resolved.SSHOperatorSocket) == "" {
		return Config{}, errors.New("topology context requires SSH target, known_hosts, and Operator socket")
	}
	if strings.TrimSpace(resolved.SignerFile) == "" ||
		strings.TrimSpace(resolved.SignerAlias) == "" ||
		strings.TrimSpace(resolved.HostKeyPinRef) == "" {
		return Config{}, errors.New("topology context requires signer alias and host-key pin reference")
	}
	if strings.TrimSpace(resolved.ExpectedNode) == "" ||
		strings.TrimSpace(resolved.ExpectedPrincipal) == "" {
		return Config{}, errors.New("topology context requires expected Node identity")
	}
	if err := resolved.validateAndNormalize(); err != nil {
		return Config{}, err
	}
	return resolved, nil
}

func resolveContextLocalPath(contextPath, value string) string {
	value = strings.TrimSpace(value)
	if value == "" || filepath.IsAbs(value) {
		return value
	}
	absoluteContextPath, err := filepath.Abs(contextPath)
	if err != nil {
		return value
	}
	return filepath.Clean(filepath.Join(filepath.Dir(absoluteContextPath), value))
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
	data, err := readContextFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read contexts file: %w", err)
	}
	store, err := decodeContextFile(data)
	if err != nil {
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
	data, err := readContextFile(path)
	if err != nil {
		return StoredContext{}, fmt.Errorf("read contexts file: %w", err)
	}
	store, err := decodeContextFile(data)
	if err != nil {
		return StoredContext{}, fmt.Errorf("parse contexts file: %w", err)
	}
	ctx, ok := store.Contexts[name]
	if !ok {
		return StoredContext{}, fmt.Errorf("context %q not found", name)
	}
	return ctx, nil
}

func decodeContextFile(data []byte) (ContextFile, error) {
	var store ContextFile
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&store); err != nil {
		return ContextFile{}, err
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		if err == nil {
			return ContextFile{}, errors.New("multiple JSON values")
		}
		return ContextFile{}, err
	}
	return store, nil
}

func readContextFile(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("contexts file must be a regular file")
	}
	data, err := io.ReadAll(io.LimitReader(file, maxContextsBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxContextsBytes {
		return nil, errors.New("contexts file exceeds size limit")
	}
	return data, nil
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
	if c.Addr == "" || c.Addr == defaultAddr {
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
	if c.SignerAlias == "" {
		c.SignerAlias = stored.SignerAlias
	}
	if c.HostKeyPinRef == "" {
		c.HostKeyPinRef = stored.HostKeyPinRef
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
	if c.ExpectedRealm == "" {
		c.ExpectedRealm = stored.ExpectedRealm
	}
	if c.AuthorityStateRef == "" {
		c.AuthorityStateRef = stored.AuthorityStateRef
	}
	if c.AuthorityBackupRef == "" {
		c.AuthorityBackupRef = stored.AuthorityBackupRef
	}
	if c.CheckpointRepositoryRef == "" {
		c.CheckpointRepositoryRef = stored.CheckpointRepositoryRef
	}
	if len(c.ScopeHints) == 0 {
		c.ScopeHints = append([]string(nil), stored.ScopeHints...)
	}
	if c.Timeout == defaultTimeout && stored.Timeout != "" {
		if d, err := time.ParseDuration(stored.Timeout); err == nil {
			c.Timeout = d
		}
	}
}

func (c *Config) applyEnv() {
	if c.Addr == "" || c.Addr == defaultAddr {
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
	parsed, err := url.Parse(c.Addr)
	validUnixSocket := err == nil && parsed.Scheme == "unix" && parsed.Path != ""
	if validUnixSocket && parsed.Host != "" {
		validUnixSocket = filepath.IsAbs(parsed.Host + parsed.Path)
	}
	if !validUnixSocket {
		return errors.New("operator address must use a protected Unix socket")
	}
	if c.SSH != "" {
		if strings.HasPrefix(c.SSH, "-") || strings.ContainsAny(c.SSH, " \t\r\n") {
			return fmt.Errorf("invalid ssh target %q", c.SSH)
		}
		if c.SSHOperatorSocket == "" {
			return errors.New("SSH transport requires an absolute remote Operator Unix socket")
		}
		if !path.IsAbs(c.SSHOperatorSocket) || strings.ContainsAny(c.SSHOperatorSocket, "\x00:\r\n") {
			return errors.New("SSH transport requires an absolute remote Operator Unix socket")
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
	if c.SignerFile == "" {
		dir, err := os.UserConfigDir()
		if err != nil {
			return fmt.Errorf("resolve signer location: %w", err)
		}
		c.SignerFile = filepath.Join(dir, "ardents", "identity", "device-v1.json")
	}
	if c.SignerFile != "" {
		if strings.TrimSpace(c.ExpectedPrincipal) == "" {
			return fmt.Errorf("Principal sessions require the target Node Principal via --principal or context")
		}
		if _, err := identityprincipal.Parse(c.ExpectedPrincipal); err != nil {
			return fmt.Errorf("target Node Principal is invalid")
		}
	} else {
		return fmt.Errorf("authentication requires a Principal signer on a protected Unix socket or SSH stream-local forwarding")
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

func (c *Config) HasIdentityBinding() bool {
	return strings.TrimSpace(c.ExpectedNode) != "" || strings.TrimSpace(c.ExpectedPrincipal) != "" ||
		strings.TrimSpace(c.ExpectedPublicKey) != ""
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
