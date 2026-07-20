package participation

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	db "ardents/internal/persistence"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/waku-org/go-waku/waku/persistence"
	"github.com/waku-org/go-waku/waku/persistence/sqlite"
	wakuNode "github.com/waku-org/go-waku/waku/v2/node"
	"github.com/waku-org/go-waku/waku/v2/protocol"
	"github.com/waku-org/go-waku/waku/v2/protocol/relay"
	"github.com/waku-org/go-waku/waku/v2/utils"
)

func NewMessageProvider(path string) (*persistence.DBStore, error) {
	dsn, err := MessageProviderDSN(path)
	if err != nil {
		return nil, err
	}
	db, err := sqlite.NewDB(dsn, utils.Logger())
	if err != nil {
		return nil, err
	}
	return persistence.NewDBStore(
		prometheus.DefaultRegisterer,
		utils.Logger(),
		persistence.WithDB(db),
		persistence.WithMigrations(sqlite.Migrations),
	)
}

func MessageProviderExists(path string) (bool, error) {
	if strings.TrimSpace(path) == "" {
		return false, nil
	}
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !info.Mode().IsRegular() {
		return false, fmt.Errorf("Waku Store must be a regular file")
	}
	return true, nil
}

func MessageProviderDSN(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return ":memory:", nil
	}
	if err := db.EnsurePrivateDir(filepath.Dir(path)); err != nil {
		return "", err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return "", err
	}
	if err := db.ProtectPrivateFile(path); err != nil {
		return "", err
	}
	return path, nil
}

func BindAddress(explicit string, envVar string) string {
	if addr := strings.TrimSpace(explicit); addr != "" {
		return addr
	}
	if addr := strings.TrimSpace(os.Getenv(envVar)); addr != "" {
		return addr
	}
	return "0.0.0.0"
}

func ListenPort(port int) int {
	if port < 0 {
		return 0
	}
	return port
}

func SecureWebsocketAddress(hostAddr *net.TCPAddr, fallback string) string {
	addr := hostAddr.IP.String()
	if strings.TrimSpace(addr) == "" || addr == "<nil>" {
		return fallback
	}
	return addr
}

func StartWakuNode(ctx context.Context, node *wakuNode.WakuNode, defaultPubsubTopic string, relayEnabled bool) error {
	if err := node.Start(ctx); err != nil {
		return err
	}
	if !relayEnabled {
		return nil
	}
	if _, err := node.Relay().Subscribe(ctx, protocol.NewContentFilter(defaultPubsubTopic), relay.WithoutConsumer()); err != nil {
		node.Stop()
		return fmt.Errorf("subscribe relay: %w", err)
	}
	return nil
}
