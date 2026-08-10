package siteexperiment

import (
	"context"
	"encoding/hex"
	"errors"
	"net"
	"os"
	"path/filepath"
	"time"
)

// RoleConfig contains only the local, role-scoped inputs accepted by a Gate C
// container process.
type RoleConfig struct {
	SocketPath        string
	GatewaySocketPath string
	NonceHex          string
	ConfigPath        string
	EvidenceDir       string
	ProbeKind         string
	ObserverName      string
	ObserverAddress   string
}

// RunRole runs one member of the closed Gate C container-role set.
func RunRole(ctx context.Context, role string, config RoleConfig) error {
	if ctx == nil {
		return errors.New("gate C role context is required")
	}
	switch role {
	case "http-application":
		nonce, err := hex.DecodeString(config.NonceHex)
		if err != nil || len(nonce) != 32 || hex.EncodeToString(nonce) != config.NonceHex {
			return errors.New("HTTP Application nonce must be 32 canonical bytes")
		}
		if err := runHTTPApplicationRole(ctx, config.SocketPath, nonce); err != nil {
			return err
		}
		return writeBoundedJSON(filepath.Join(config.EvidenceDir, "http-application.json"), map[string]any{"schema_version": "gatec-http-application-evidence/v1", "status": "completed", "ordinary_listener": false})
	case "http-client":
		return runHTTPClientRole(ctx, config.SocketPath, config.NonceHex, config.EvidenceDir)
	case "client-endpoint":
		return runClientEndpointRole(ctx, config.ConfigPath, config.EvidenceDir)
	case "resolution-relay":
		return runResolutionRelayRole(ctx, config.EvidenceDir)
	case "resolution-gateway":
		return runResolutionGatewayRole(ctx, config.ConfigPath, config.SocketPath, config.EvidenceDir)
	case "authority":
		return runAuthorityRole(ctx, config.ConfigPath, config.SocketPath, config.GatewaySocketPath, config.EvidenceDir)
	case "administration":
		return runAdministrationRole(ctx, config.ConfigPath, config.SocketPath, config.EvidenceDir)
	case "isolation-probe":
		return runIsolationProbe(ctx, config.ProbeKind, config.ObserverName, config.ObserverAddress)
	case "isolation-observer":
		return runIsolationObserver(ctx)
	case "isolation-control":
		return runIsolationControl(ctx, config.ObserverName, config.ObserverAddress)
	default:
		return errors.New("role is not part of the closed Gate C set")
	}
}

func runHTTPApplicationRole(ctx context.Context, socketPath string, nonce []byte) error {
	if socketPath == "" || !filepath.IsAbs(socketPath) || filepath.Clean(socketPath) != socketPath {
		return errors.New("HTTP Application socket path must be absolute and canonical")
	}
	parent := filepath.Dir(socketPath)
	info, err := os.Lstat(parent)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("HTTP Application socket parent is not an owned real directory")
	}
	if _, err := os.Lstat(socketPath); !os.IsNotExist(err) {
		return errors.New("HTTP Application socket path is not clean")
	}
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: socketPath, Net: "unix"})
	if err != nil {
		return err
	}
	defer func() {
		_ = listener.Close()
		removeOwnedSocket(socketPath)
	}()
	if err := os.Chmod(socketPath, 0o666); err != nil {
		return err
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(15 * time.Second)
	}
	if err := listener.SetDeadline(deadline); err != nil {
		return err
	}
	connection, err := listener.AcceptUnix()
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return err
	}
	_ = listener.Close()
	return serveHTTPApplication(connection, nonce)
}

func removeOwnedSocket(path string) {
	info, err := os.Lstat(path)
	if err == nil && info.Mode()&os.ModeSocket != 0 {
		_ = os.Remove(path)
	}
}
