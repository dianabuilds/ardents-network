package waku

import (
	"ardents/internal/network"
	networkroute "ardents/internal/network/routing"
	db "ardents/internal/storage"
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	libp2pevent "github.com/libp2p/go-libp2p/core/event"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/waku-org/go-waku/waku/persistence"
	"github.com/waku-org/go-waku/waku/persistence/sqlite"
	wakuNode "github.com/waku-org/go-waku/waku/v2/node"
	"github.com/waku-org/go-waku/waku/v2/protocol"
	"github.com/waku-org/go-waku/waku/v2/protocol/relay"
	"github.com/waku-org/go-waku/waku/v2/utils"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

func (s *Service) BuildCandidates(record network.RouteRecord, trusted bool) []networkroute.Candidate {
	return networkroute.BuildCandidates(record.Subject, record.Service, record.Mode, record.Endpoints, trusted, s.isObservedUsable)
}

func (s *Service) PublishRelayEnvelope(ctx context.Context, envelope network.Envelope) error {
	s.mu.Lock()
	constrained := s.cfg.NodeProfile == network.NodeProfileConstrainedClient
	s.mu.Unlock()
	if constrained {
		return fmt.Errorf("relay publication is unavailable in constrained light-client mode")
	}
	done, err := s.acquireNetworkOperation(len(envelope.Payload), "")
	if err != nil {
		return err
	}
	s.mu.Lock()
	node := s.node
	s.mu.Unlock()
	err = Publish(ctx, node, envelope)
	done(err)
	return err
}

func (s *Service) PublishLightpushEnvelope(ctx context.Context, provider string, envelope network.Envelope) error {
	s.mu.Lock()
	node := s.node
	constrained := s.cfg.NodeProfile == network.NodeProfileConstrainedClient
	s.mu.Unlock()
	if !constrained {
		return fmt.Errorf("lightpush client operation is available only in constrained light-client mode")
	}
	done, err := s.acquireNetworkOperation(len(envelope.Payload), provider)
	if err != nil {
		return err
	}
	err = PublishLightpush(ctx, node, provider, envelope)
	done(err)
	return err
}

func (s *Service) SubscribeFilterEnvelopes(ctx context.Context, providers []string, contentTopic string) (<-chan network.Envelope, error) {
	s.mu.Lock()
	node := s.node
	constrained := s.cfg.NodeProfile == network.NodeProfileConstrainedClient
	s.mu.Unlock()
	if !constrained {
		return nil, fmt.Errorf("filter client operation is available only in constrained light-client mode")
	}
	providerKey := strings.Join(providers, "\n")
	done, err := s.acquireNetworkOperation(0, providerKey)
	if err != nil {
		return nil, err
	}
	items, err := SubscribeFilter(ctx, node, providers, network.DefaultPubsubTopic, contentTopic)
	done(err)
	if err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Service) FetchEnvelopes(ctx context.Context, endpoints []string, contentTopic string) ([]network.Envelope, error) {
	done, err := s.acquireNetworkOperation(0, strings.Join(endpoints, "\n"))
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	node := s.node
	maxResults := s.cfg.Limits.MaxStoreResults
	s.mu.Unlock()
	items, err := FetchEnvelopes(
		ctx, node, endpoints, maxResults, network.DefaultPubsubTopic, contentTopic,
	)
	done(err)
	if err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Service) SubscribeRelayEnvelopes(ctx context.Context, pubsubTopic string, contentTopics ...string) (<-chan network.Envelope, error) {
	s.mu.Lock()
	constrained := s.cfg.NodeProfile == network.NodeProfileConstrainedClient
	s.mu.Unlock()
	if constrained {
		return nil, fmt.Errorf("relay subscription is unavailable in constrained light-client mode")
	}
	done, err := s.acquireNetworkOperation(0, "")
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	node := s.node
	s.mu.Unlock()
	items, err := Subscribe(ctx, node, pubsubTopic, contentTopics...)
	done(err)
	if err != nil {
		return nil, err
	}
	out := make(chan network.Envelope, 16)
	go func() {
		defer close(out)
		for item := range items {
			out <- item
		}
	}()
	return out, nil
}

func (s *Service) isObservedUsable(endpoint string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, ok := s.observed[strings.TrimSpace(endpoint)]
	return ok && state.usable
}

type endpointObservation struct {
	usable bool
	reason string
}

type Service struct {
	mu                   sync.Mutex
	state                string
	reason               string
	id                   string
	activeProfile        network.Profile
	activeMode           network.Mode
	endpoints            []string
	bootstrap            network.BootstrapStatus
	bootstrapNodes       []string
	discoveredNodes      []string
	dnsDiscoveryError    string
	lastDNSRefresh       time.Time
	dnsDiscovery         dnsPeerDiscovery
	reachability         network.ReachabilitySnapshot
	privateLANProbeUntil time.Time
	reachabilityEvents   libp2pevent.Subscription
	reachabilityObs      func()
	bootstrapObs         func(network.BootstrapDialReport)
	observed             map[string]endpointObservation
	controller           *network.ModeController
	switchReason         network.SwitchReason
	switchAuto           bool
	recoveryState        network.RecoveryState
	modeRestartPending   bool
	lastBootstrapAttempt time.Time
	runtimeCancel        context.CancelFunc
	runtimeDone          chan struct{}
	node                 *wakuNode.WakuNode
	messageProvider      *persistence.DBStore
	operationSlots       chan struct{}
	operationRate        *rate.Limiter
	providerPenalties    map[string]providerPenalty
	abuse                network.AbuseSnapshot
	cfg                  network.Config
	limitConfigErr       error
}

func New(cfg ...network.Config) *Service {
	svc := &Service{
		state:     "new",
		bootstrap: network.BootstrapStatus{State: "idle", Reason: "transport not started"},
		observed:  map[string]endpointObservation{},
	}
	if len(cfg) > 0 {
		svc.cfg = cfg[0]
	}
	svc.cfg.DNSDiscoveryURLs = cloneStrings(svc.cfg.DNSDiscoveryURLs)
	svc.cfg.AdvertiseAddresses = cloneStrings(svc.cfg.AdvertiseAddresses)
	svc.cfg.ReachabilityMode = network.NormalizeReachabilityMode(svc.cfg.ReachabilityMode)
	svc.reachability = initialReachability(svc.cfg.ReachabilityMode)
	svc.dnsDiscovery = wakuDNSPeerDiscovery{}
	svc.cfg.Profile = network.NormalizeProfile(svc.cfg.Profile)
	svc.cfg.NodeProfile = network.NormalizeNodeProfile(svc.cfg.NodeProfile)
	svc.activeProfile = svc.cfg.Profile
	svc.activeMode = network.ModeSteady
	svc.controller = network.NewModeController(network.DefaultSelectionPolicy())
	svc.limitConfigErr = validateLimits(svc.cfg.Limits)
	svc.initializeLimits()
	return svc
}

func (s *Service) State() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.serviceStateLocked().State
}

func (s *Service) Reason() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.serviceStateLocked().Reason
}

func (s *Service) Endpoints() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneStrings(s.publishableEndpointsLocked())
}

func (s *Service) PeerCount() int {
	s.mu.Lock()
	node := s.node
	s.mu.Unlock()
	if node == nil {
		return 0
	}
	return node.PeerCount()
}

func (s *Service) RelayPeerCount(pubsubTopic string) int {
	s.mu.Lock()
	node := s.node
	constrained := s.cfg.NodeProfile == network.NodeProfileConstrainedClient
	s.mu.Unlock()
	if node == nil || constrained {
		return 0
	}
	if pubsubTopic == "" {
		pubsubTopic = network.DefaultPubsubTopic
	}
	return len(node.Relay().PubSub().ListPeers(pubsubTopic))
}

func (s *Service) SetBootstrapNodes(nodes []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bootstrapNodes = cloneStrings(nodes)
}

func (s *Service) SetBootstrapObserver(fn func(network.BootstrapDialReport)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bootstrapObs = fn
}

func (s *Service) BootstrapStatus() network.BootstrapStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.currentBootstrapStatusViewLocked()
}

func NewMessageProvider(path string, retention network.StoreRetention) (*persistence.DBStore, error) {
	if retention.MaxMessages < 1 || retention.MaxAge < time.Second || retention.MaxBytes < 4<<20 {
		return nil, fmt.Errorf("Waku Store retention limits must be finite and positive")
	}
	if strings.TrimSpace(path) != "" {
		if _, err := os.Stat(path); err == nil {
			used, sizeErr := storeDiskBytes(path)
			if sizeErr != nil {
				return nil, fmt.Errorf("inspect existing Waku Store size: %w", sizeErr)
			}
			if used > retention.MaxBytes {
				return nil, fmt.Errorf("existing Waku Store exceeds configured byte limit")
			}
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("inspect existing Waku Store: %w", err)
		}
	}
	dsn, err := MessageProviderDSN(path)
	if err != nil {
		return nil, err
	}
	database, err := sqlite.NewDB(dsn, utils.Logger())
	if err != nil {
		return nil, err
	}
	options := []persistence.DBOption{
		persistence.WithDB(database),
		persistence.WithMigrations(func(db *sql.DB, logger *zap.Logger) error {
			if err := sqlite.Migrations(db, logger); err != nil {
				return err
			}
			return configureBoundedStore(db, retention)
		}),
	}
	options = append(options, persistence.WithRetentionPolicy(retention.MaxMessages, retention.MaxAge))
	return persistence.NewDBStore(
		prometheus.DefaultRegisterer,
		utils.Logger(),
		options...,
	)
}

func configureBoundedStore(database *sql.DB, retention network.StoreRetention) error {
	var pageSize int64
	if err := database.QueryRow("PRAGMA page_size").Scan(&pageSize); err != nil {
		return fmt.Errorf("inspect Waku Store page size: %w", err)
	}
	mainBudget := retention.MaxBytes * 3 / 4
	walBudget := retention.MaxBytes / 8
	maxPages := mainBudget / pageSize
	walPages := walBudget / pageSize
	if maxPages < 1 || walPages < 1 {
		return fmt.Errorf("Waku Store byte limit is too small")
	}
	var appliedMaxPages int64
	if err := database.QueryRow(fmt.Sprintf("PRAGMA max_page_count = %d", maxPages)).Scan(&appliedMaxPages); err != nil {
		return fmt.Errorf("configure Waku Store page limit: %w", err)
	}
	if appliedMaxPages > maxPages {
		return fmt.Errorf("existing Waku Store main database exceeds configured byte budget")
	}
	statements := []string{
		fmt.Sprintf("PRAGMA journal_size_limit = %d", walBudget),
		fmt.Sprintf("PRAGMA wal_autocheckpoint = %d", walPages),
		"DROP TRIGGER IF EXISTS ardents_store_retention_after_insert",
		fmt.Sprintf(`CREATE TRIGGER ardents_store_retention_after_insert
AFTER INSERT ON message
BEGIN
  UPDATE message
     SET storedAt = CAST(strftime('%%s','now') AS INTEGER) * 1000000000
   WHERE messageHash = NEW.messageHash;
  DELETE FROM message
   WHERE storedAt < (CAST(strftime('%%s','now') AS INTEGER) - %d) * 1000000000;
  DELETE FROM message
   WHERE messageHash IN (
     SELECT messageHash FROM message
      ORDER BY storedAt DESC, messageHash DESC
      LIMIT 1 OFFSET %d
   );
END`, int64(retention.MaxAge/time.Second), retention.MaxMessages),
		fmt.Sprintf(`UPDATE message
SET storedAt = CAST(strftime('%%s','now') AS INTEGER) * 1000000000
WHERE storedAt > (CAST(strftime('%%s','now') AS INTEGER) + %d) * 1000000000`,
			int64(persistence.MaxTimeVariance/time.Second)),
	}
	for _, statement := range statements {
		if _, err := database.Exec(statement); err != nil {
			return fmt.Errorf("configure bounded Waku Store: %w", err)
		}
	}
	return nil
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
		return false, fmt.Errorf("waku Store must be a regular file")
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
