package waku

import (
	"ardents/internal/network"
	networkpeer "ardents/internal/network/peer"
	db "ardents/internal/storage"
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	gethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/libp2p/go-libp2p"
	libp2ptcp "github.com/libp2p/go-libp2p/p2p/transport/tcp"
	"github.com/waku-org/go-waku/waku/persistence"
	wakuNode "github.com/waku-org/go-waku/waku/v2/node"
	"github.com/waku-org/go-waku/waku/v2/protocol"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var transportSeq int64

func (s *Service) Start(ctx context.Context) error {
	s.mu.Lock()
	if len(s.endpoints) > 0 {
		s.mu.Unlock()
		return nil
	}
	node, err := s.prepareNodeLocked()
	if err != nil {
		return s.failStartLocked(err)
	}
	if err := StartWakuNode(ctx, node, network.DefaultPubsubTopic, s.relayEnabledLocked()); err != nil {
		return s.failStartLocked(err)
	}
	s.markStartedLocked(node)
	if err := s.startReachabilityObservationLocked(node); err != nil {
		s.node = nil
		s.endpoints = nil
		startErr := s.failStartLocked(err)
		node.Stop()
		return startErr
	}
	s.markBoundReachabilityLocked()
	s.mu.Unlock()

	s.refreshDNSPeersObserved(ctx)
	status := s.dialBootstrapPeers(ctx)

	s.mu.Lock()
	s.bootstrap = status
	s.reconcileRuntimeLocked(timeNowUTC())
	s.startRuntimeLoopLocked()
	s.mu.Unlock()
	return nil
}

func (s *Service) prepareNodeLocked() (*wakuNode.WakuNode, error) {
	if err := s.validateStartupConfigLocked(); err != nil {
		return nil, err
	}
	hostAddr, err := net.ResolveTCPAddr("tcp", net.JoinHostPort(bindAddress(s.cfg), strconv.Itoa(listenPort(s.cfg))))
	if err != nil {
		return nil, err
	}
	provider, storeExisted, err := s.prepareMessageProviderLocked()
	if err != nil {
		return nil, err
	}
	nodeKey, err := newTransportKeyStore(s.cfg.PrivateKeyPath).Ensure(storeExisted)
	if err != nil {
		return nil, err
	}
	return s.buildNode(hostAddr, provider, nodeKey)
}

func (s *Service) validateStartupConfigLocked() error {
	if s.limitConfigErr != nil {
		return s.limitConfigErr
	}
	if _, err := network.ResolveProfile(s.activeProfile); err != nil {
		return err
	}
	return ValidateConfig(s.cfg, timeNowUTC())
}

func (s *Service) Stop(_ context.Context) error {
	s.mu.Lock()
	cancel := s.runtimeCancel
	done := s.runtimeDone
	node := s.node
	reachabilityEvents := s.reachabilityEvents
	s.runtimeCancel = nil
	s.runtimeDone = nil
	s.node = nil
	s.messageProvider = nil
	s.reachabilityEvents = nil
	s.endpoints = nil
	s.state = "stopped"
	s.reason = ""
	s.bootstrap = network.BootstrapStatus{State: "idle", Reason: "transport stopped"}
	s.observed = map[string]endpointObservation{}
	s.discoveredNodes = nil
	s.dnsDiscoveryError = ""
	s.lastDNSRefresh = time.Time{}
	s.reachability = initialReachability(s.cfg.ReachabilityMode)
	s.activeMode = network.ModeSteady
	s.switchReason = network.SwitchReasonStopped
	s.switchAuto = false
	s.recoveryState = ""
	s.modeRestartPending = false
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
	if reachabilityEvents != nil {
		if err := reachabilityEvents.Close(); err != nil {
			return fmt.Errorf("close reachability event subscription: %w", err)
		}
	}
	if node != nil {
		node.Stop()
	}
	return nil
}

func (s *Service) failStartLocked(err error) error {
	s.state = "failed"
	s.reason = err.Error()
	s.bootstrap = network.BootstrapStatus{State: "degraded", Reason: err.Error()}
	s.activeMode = network.ModeSteady
	s.switchReason = network.SwitchReasonStartupFailed
	s.switchAuto = false
	s.recoveryState = network.RecoveryStateBlocked
	s.mu.Unlock()
	return err
}

func (s *Service) markStartedLocked(node *wakuNode.WakuNode) {
	id := atomic.AddInt64(&transportSeq, 1)
	s.id = fmt.Sprintf("peer-%d", id)
	s.node = node
	s.endpoints = publishedEndpoints(node, s.cfg)
	s.observed = newEndpointObservations(s.endpoints, true)
	s.state = "ready"
	s.reason = ""
	s.activeMode = network.ModeSteady
	s.modeRestartPending = false
}

func (s *Service) buildNode(hostAddr *net.TCPAddr, provider *persistence.DBStore, key *ecdsa.PrivateKey) (*wakuNode.WakuNode, error) {
	profile := network.NormalizeProfile(s.activeProfile)
	definition, err := network.ResolveProfile(profile)
	if err != nil {
		return nil, err
	}
	libp2pOptions, err := libP2POptionsForDefinition(definition)
	if err != nil {
		return nil, err
	}
	reachabilityOptions, err := reachabilityLibP2POptions(s.cfg)
	if err != nil {
		return nil, err
	}
	libp2pOptions = append(libp2pOptions, reachabilityOptions...)
	profileOptions, err := s.profileNodeOptions(definition, hostAddr)
	if err != nil {
		return nil, err
	}
	options := append(baseWakuNodeOptions(hostAddr, libp2pOptions, key), s.messagingNodeOptions(provider)...)
	options = append(options, profileOptions...)
	return wakuNode.New(options...)
}

func baseWakuNodeOptions(hostAddr *net.TCPAddr, libp2pOptions []libp2p.Option, key *ecdsa.PrivateKey) []wakuNode.WakuNodeOption {
	return []wakuNode.WakuNodeOption{
		wakuNode.WithLogger(silentWakuLogger()),
		wakuNode.WithHostAddress(hostAddr),
		wakuNode.WithLibP2POptions(libp2pOptions...),
		wakuNode.WithPrivateKey(key),
		wakuNode.WithClusterID(protocol.ClusterIndex),
		wakuNode.WithShards([]uint16{0}),
	}
}

func libP2POptionsForDefinition(definition network.Definition) ([]libp2p.Option, error) {
	switch definition.StartupVariant {
	case network.StartupVariantTCPOnly, network.StartupVariantTCPWSS:
		return []libp2p.Option{
			libp2p.NoTransports,
			libp2p.Transport(libp2ptcp.NewTCPTransport),
		}, nil
	default:
		return nil, fmt.Errorf("transport profile %q is not implemented", definition.Profile)
	}
}

func (s *Service) profileNodeOptions(definition network.Definition, hostAddr *net.TCPAddr) ([]wakuNode.WakuNodeOption, error) {
	switch definition.StartupVariant {
	case network.StartupVariantTCPOnly:
		return nil, nil
	case network.StartupVariantTCPWSS:
		certPath := strings.TrimSpace(s.cfg.WSSCertPath)
		keyPath := strings.TrimSpace(s.cfg.WSSKeyPath)
		if certPath == "" || keyPath == "" {
			return nil, fmt.Errorf("transport profile %q requires secure websocket certificate and key paths", definition.Profile)
		}
		return []wakuNode.WakuNodeOption{
			wakuNode.WithSecureWebsockets(secureWebsocketAddress(hostAddr, s.cfg), s.cfg.WSSPort, certPath, keyPath),
		}, nil
	default:
		return nil, fmt.Errorf("transport profile %q is not implemented", definition.Profile)
	}
}

func secureWebsocketAddress(hostAddr *net.TCPAddr, cfg network.Config) string {
	addr := hostAddr.IP.String()
	if strings.TrimSpace(addr) == "" || addr == "<nil>" {
		addr = bindAddress(cfg)
	}
	return addr
}

func advertisedEndpoints(endpoints []string, cfg network.Config) []string {
	if network.NormalizeProfile(cfg.Profile) != network.ProfileTCPWSS {
		return endpoints
	}
	hostPrefix := multiaddrHostPrefix(strings.TrimSpace(cfg.WSSAdvertiseAddress))
	portMarker := "/tcp/" + strconv.Itoa(cfg.WSSPort)
	for index, endpoint := range endpoints {
		if !isSecureWebsocketEndpoint(endpoint) {
			continue
		}
		if marker := strings.Index(endpoint, portMarker); marker >= 0 {
			endpoints[index] = hostPrefix + endpoint[marker:]
		}
	}
	return endpoints
}

func multiaddrHostPrefix(host string) string {
	if ip := net.ParseIP(host); ip != nil {
		if ip.To4() != nil {
			return "/ip4/" + host
		}
		return "/ip6/" + host
	}
	return "/dns4/" + host
}

func isSecureWebsocketEndpoint(endpoint string) bool {
	return strings.Contains(endpoint, "/wss") || strings.Contains(endpoint, "/tls/ws")
}

func bindAddress(cfg network.Config) string {
	return BindAddress(cfg.BindAddress, network.BindAddressEnv)
}

func listenPort(cfg network.Config) int {
	return ListenPort(cfg.ListenPort)
}

func (s *Service) dialBootstrapPeers(parent context.Context) network.BootstrapStatus {
	s.mu.Lock()
	node := s.node
	peers := s.effectiveBootstrapNodesLocked()
	observer := s.bootstrapObs
	s.lastBootstrapAttempt = timeNowUTC()
	s.mu.Unlock()
	if node == nil {
		return network.BootstrapStatus{State: "degraded", Reason: "transport node is not started"}
	}
	attempts, failures, lastErr := 0, 0, ""
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	for _, addr := range peers {
		addr, ok := networkpeer.Normalize(addr)
		if !ok {
			continue
		}
		attempts++
		err := node.DialPeer(ctx, addr)
		s.observeEndpoint(addr, err == nil, err)
		lastErr = network.UpdateBootstrapFailures(err, failures, &failures, lastErr)
		s.reportBootstrapDial(observer, addr, err)
	}
	if s.cfg.NodeProfile == network.NodeProfileConstrainedClient {
		providers := InspectLightProviders(node)
		return network.ClassifyLightBootstrapStatus(
			providers.FilterPeers, providers.LightpushPeers, providers.StorePeers, attempts, failures,
		)
	}
	relayPeers := AwaitRelayPeerCount(ctx, node, network.DefaultPubsubTopic)
	return network.ClassifyBootstrapStatus(relayPeers, attempts, failures, lastErr)
}

func (s *Service) reportBootstrapDial(observer func(network.BootstrapDialReport), addr string, err error) {
	report := network.BootstrapDialReport{Peer: addr, Success: err == nil}
	if err != nil {
		report.Detail = err.Error()
	}
	if observer != nil {
		go observer(report)
	}
}

func (s *Service) observeEndpoint(addr string, usable bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state := endpointObservation{usable: usable}
	if err != nil {
		state.reason = err.Error()
	}
	s.observed[addr] = state
}

const bootstrapRetryInterval = 2 * time.Second
const desiredRelayPeers = 3

func (s *Service) shouldRetryBootstrapLocked(now time.Time) bool {
	if s.node == nil {
		return false
	}
	signals := HealthSnapshot(s.readinessStateLocked(s.currentBootstrapStatusViewLocked()))
	if signals.BootstrapSourceCount == 0 {
		return false
	}
	if s.cfg.NodeProfile == network.NodeProfileConstrainedClient {
		return signals.BootstrapStatus.State != "ready" &&
			(s.lastBootstrapAttempt.IsZero() || now.Sub(s.lastBootstrapAttempt) >= bootstrapRetryInterval)
	}
	target := min(desiredRelayPeers, signals.BootstrapSourceCount)
	if signals.RelayPeerCount >= target {
		return false
	}
	return s.lastBootstrapAttempt.IsZero() || now.Sub(s.lastBootstrapAttempt) >= bootstrapRetryInterval
}

type transportKeyStore struct {
	path string
}

type transportKeyLedger struct {
	PrivateKey string `json:"private_key"`
}

func newTransportKeyStore(path string) transportKeyStore {
	return transportKeyStore{path: strings.TrimSpace(path)}
}

func (s transportKeyStore) Ensure(requireExisting bool) (*ecdsa.PrivateKey, error) {
	if key, err := s.load(); err != nil || key != nil {
		return key, err
	}
	if requireExisting {
		return nil, fmt.Errorf("transport key is missing while persistent Waku Store exists; restore matching backup")
	}
	key, err := gethcrypto.GenerateKey()
	if err != nil {
		return nil, err
	}
	if err := s.save(key); err != nil {
		return nil, err
	}
	return key, nil
}

func (s transportKeyStore) load() (*ecdsa.PrivateKey, error) {
	if s.path == "" {
		return nil, nil
	}
	raw, found, err := db.ReadPrivateFile(s.path)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("transport key file is empty")
	}
	var stored transportKeyLedger
	if err := json.Unmarshal(raw, &stored); err != nil {
		return nil, err
	}
	if strings.TrimSpace(stored.PrivateKey) == "" {
		return nil, fmt.Errorf("transport key file does not contain a private key")
	}
	decoded, err := hex.DecodeString(stored.PrivateKey)
	if err != nil {
		return nil, err
	}
	return gethcrypto.ToECDSA(decoded)
}

func (s transportKeyStore) save(key *ecdsa.PrivateKey) error {
	if s.path == "" || key == nil {
		return nil
	}
	raw, err := json.MarshalIndent(transportKeyLedger{
		PrivateKey: hex.EncodeToString(gethcrypto.FromECDSA(key)),
	}, "", "  ")
	if err != nil {
		return err
	}
	return db.AtomicWritePrivateFile(s.path, raw)
}

func silentWakuLogger() *zap.Logger {
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
		zapcore.AddSync(io.Discard),
		zapcore.DebugLevel,
	)
	return zap.New(core)
}

func cloneStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	return append([]string(nil), in...)
}

func mergeUniqueStrings(groups ...[]string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0)
	for _, group := range groups {
		for _, item := range group {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			if _, exists := seen[item]; exists {
				continue
			}
			seen[item] = struct{}{}
			out = append(out, item)
		}
	}
	return out
}

func newEndpointObservations(endpoints []string, usable bool) map[string]endpointObservation {
	observed := make(map[string]endpointObservation, len(endpoints))
	for _, endpoint := range endpoints {
		observed[endpoint] = endpointObservation{usable: usable}
	}
	return observed
}

type stringAddress interface {
	String() string
}

func stringifyListenAddresses[T stringAddress](addrs []T) []string {
	out := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		out = append(out, addr.String())
	}
	return out
}
