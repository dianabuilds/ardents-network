package daemon

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"ardents/internal/buildinfo"
	runtimeconfig "ardents/internal/config"
	diagapi "ardents/internal/diagnostics"
	identityaccess "ardents/internal/identity/access"
	identityprotocol "ardents/internal/identity/protocol"
	"ardents/internal/storage"
	applicationv1 "ardents/sdk/go/protocol/applicationv1"
)

type LocalAPIConfig struct {
	TargetNode  string
	TargetID    string
	Protected   bool
	PeerBinding [32]byte
	Source      identityaccess.SourceKey
}

type LocalAPIHandlerFactory func(Owners, LocalAPIConfig) (string, http.Handler, error)

type ApplicationAPIConfig struct {
	TargetID    string
	Protected   bool
	PeerBinding [32]byte
	Source      identityaccess.SourceKey
}

type ApplicationAPIHandlerFactory func(Owners, ApplicationAPIConfig) (string, http.Handler, error)

type OperatorSurface interface {
	Handler() http.Handler
	Middleware(http.Handler) http.Handler
}

type OperatorSurfaceFactory func(Owners, string) (OperatorSurface, error)

func Run(localAPI LocalAPIHandlerFactory, applicationAPI ApplicationAPIHandlerFactory, operatorSurface OperatorSurfaceFactory) (returnErr error) {
	if len(os.Args) == 2 && (os.Args[1] == "version" || os.Args[1] == "--version") {
		encoded, err := json.Marshal(buildinfo.Current())
		if err != nil {
			_, _ = fmt.Fprintln(os.Stderr, "encode build info")
			return fmt.Errorf("encode build info: %w", err)
		}
		_, _ = fmt.Println(string(encoded))
		return nil
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	stateDir, err := startupStateDirectory()
	if err != nil {
		return fmt.Errorf("resolve state directory before startup: %w", err)
	}
	stateLock, err := storage.AcquireStateDirLock(stateDir)
	if err != nil {
		return fmt.Errorf("acquire state directory lock: %w", err)
	}
	defer func() {
		if err := stateLock.Close(); err != nil {
			slog.Error("release state directory lock", "error", err)
		}
	}()
	cfg, err := loadRuntimeConfig()
	if err != nil {
		return fmt.Errorf("load runtime config: %w", err)
	}
	if filepath.Clean(cfg.Node.Data.Dir) != filepath.Clean(stateDir) {
		return fmt.Errorf("state directory changed while acquiring startup lock")
	}
	identityAccess, err := storage.OpenIdentityAccess(ctx, stateDir, identityaccess.StorageSchema())
	if err != nil {
		return fmt.Errorf("open identity access database: %w", err)
	}
	defer func() {
		returnErr = errors.Join(returnErr, closeIdentityAccess(identityAccess))
	}()

	process, err := newDaemonOwners(cfg.Node, identityAccess)
	if err != nil {
		return fmt.Errorf("construct identity access service: %w", err)
	}
	n := process.Node

	if err := n.Start(ctx); err != nil {
		return fmt.Errorf("start node: %w", err)
	}
	defer stopNode(n)
	if err := configureApplicationIdentity(&process, identityAccess, applicationIdentityOptions{Enabled: cfg.ApplicationEnabled}); err != nil {
		return fmt.Errorf("configure Application identity: %w", err)
	}
	if err := ensureFirstOperatorBootstrapTicket(ctx, process.PrincipalAccess, n.GetNodeRuntime().Identity.Principal); err != nil {
		return fmt.Errorf("prepare first Operator bootstrap: %w", err)
	}

	if operatorSurface == nil {
		return fmt.Errorf("configure observability: operator surface factory is required")
	}
	surface, err := operatorSurface(process, cfg.ObservabilityToken)
	if err != nil {
		return fmt.Errorf("configure observability: %w", err)
	}
	observabilityServer := newHTTPServer(cfg.ObservabilityAddr, surface.Handler())
	targets := []serveTarget{}
	servers := []*http.Server{observabilityServer}
	protectedHandler, handlerErr := newProtectedOperatorHandler(process, cfg, localAPI)
	if handlerErr != nil {
		return fmt.Errorf("configure protected Operator API: %w", handlerErr)
	}
	socketServer, socketListener, socketErr := newUnixHTTPServer(cfg.SocketPath, protectedHandler)
	if socketErr != nil {
		return fmt.Errorf("configure local API socket: %w", socketErr)
	}
	servers = append(servers, socketServer)
	targets = append(targets, serveTarget{serve: func() error { return socketServer.Serve(socketListener) }})
	slog.Info("ardentsd local control socket ready", "path", cfg.SocketPath)
	if cfg.ApplicationEnabled {
		if cfg.ApplicationSocketPath != "" {
			_, applicationHandler, applicationErr := newProtectedApplicationHandler(process, cfg, applicationAPI)
			if applicationErr != nil {
				return fmt.Errorf("configure protected Application Interface: %w", applicationErr)
			}
			applicationSocketServer, applicationListener, socketErr := newApplicationUnixHTTPServer(cfg.ApplicationSocketPath, applicationHandler)
			if socketErr != nil {
				return fmt.Errorf("configure Application Interface socket: %w", socketErr)
			}
			servers = append(servers, applicationSocketServer)
			targets = append(targets, serveTarget{serve: func() error { return applicationSocketServer.Serve(applicationListener) }})
			slog.Info("ardentsd Application Interface socket ready", "path", cfg.ApplicationSocketPath)
		}
	}
	targets = append(targets, serveTarget{serve: observabilityServer.ListenAndServe})
	slog.Info("ardentsd observability listening", "addr", observabilityServer.Addr)
	if err := serveAndDrain(ctx, stop, servers, targets...); err != nil {
		return fmt.Errorf("serve daemon: %w", err)
	}
	return nil
}

func ensureFirstOperatorBootstrapTicket(ctx context.Context, service *identityaccess.Service, node string) error {
	if service == nil {
		return fmt.Errorf("Principal access service is required")
	}
	configPath := runtimeconfig.OperatorFile()
	if strings.TrimSpace(configPath) == "" {
		return fmt.Errorf("canonical operator configuration is required")
	}
	ticket, err := service.IssueBootstrapTicket(ctx, node)
	if errors.Is(err, identityaccess.ErrConflict) {
		return nil
	}
	if err != nil {
		return err
	}
	path := filepath.Join(filepath.Dir(configPath), "operator-bootstrap-ticket")
	encoded := []byte(base64.RawURLEncoding.EncodeToString(ticket[:]))
	if err := storage.AtomicWritePrivateFile(path, encoded); err != nil {
		return fmt.Errorf("write protected Bootstrap Ticket")
	}
	slog.Info("first Operator bootstrap ticket ready", "path", path)
	return nil
}

func startupStateDirectory() (string, error) {
	path := runtimeconfig.OperatorFile()
	if path == "" {
		return "", fmt.Errorf("ARDENTS_CONFIG_FILE is required")
	}
	doc, err := runtimeconfig.Load(path)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(doc.Node.DataDir) == "" {
		return "", fmt.Errorf("operator configuration has no state directory")
	}
	return doc.Node.DataDir, nil
}

func stopNode(n *Node) {
	if err := n.Stop(context.Background()); err != nil {
		log.Printf("stop node: %v", err)
	}
}

type serveTarget struct {
	serve func() error
}

type applicationIdentityOptions struct {
	Enabled bool
}

func newDaemonOwners(cfg Config, identityAccess *storage.Handle, applicationOptions ...applicationIdentityOptions) (Owners, error) {
	owners := NewOwners(cfg)
	owners.IdentityAccess = identityAccess
	options := applicationIdentityOptions{}
	if len(applicationOptions) > 1 {
		return Owners{}, fmt.Errorf("duplicate Application identity options")
	}
	if len(applicationOptions) == 1 {
		options = applicationOptions[0]
	}
	if err := configureApplicationIdentity(&owners, identityAccess, options); err != nil {
		return Owners{}, err
	}
	return owners, nil
}

// configureApplicationIdentity runs after Node.Start in production so the
// Node key already exists. Tests without an Application surface can still use
// newDaemonOwners before startup because no installation binding is required.
func configureApplicationIdentity(owners *Owners, identityAccess *storage.Handle, options applicationIdentityOptions) error {
	if owners == nil || owners.Node == nil || identityAccess == nil {
		return fmt.Errorf("identity access dependencies are required")
	}
	owners.IdentityAccess = identityAccess
	issuer := nodeAccessGrantIssuer{node: owners.Node}
	service, err := identityaccess.NewService(identityaccess.Config{
		Database: identityAccess, GrantIssuer: issuer, Audit: identityAccessAudit{events: owners.Events},
		EnableBootstrapTickets: true, EnableApplicationEnrollment: options.Enabled,
	})
	if err != nil {
		return err
	}
	owners.PrincipalAccess = service
	return nil
}

type identityAccessAudit struct{ events diagapi.EventWriter }

func (a identityAccessAudit) RecordIdentityAccess(event identityaccess.AuditEvent) {
	if a.events == nil {
		return
	}
	a.events.RecordEventCommand(diagapi.RecordEventCommand{Domain: "identity_access", Type: "principal_access_" + event.Outcome, Resource: event.Audience.Node, Message: "Principal access " + event.Outcome, ReasonCode: event.Reason, Payload: map[string]any{"outcome": event.Outcome, "reason": event.Reason, "principal": event.Principal, "device": event.DeviceID, "node": event.Audience.Node, "interface": event.Audience.Interface.String(), "protocol_major": event.Audience.ProtocolMajor}})
}

type nodeAccessGrantIssuer struct{ node *Node }

func (i nodeAccessGrantIssuer) privateKey() ed25519.PrivateKey {
	i.node.mu.Lock()
	defer i.node.mu.Unlock()
	return append(ed25519.PrivateKey(nil), i.node.private...)
}

func (i nodeAccessGrantIssuer) PublicKey() ed25519.PublicKey {
	key := i.privateKey()
	if len(key) != ed25519.PrivateKeySize {
		return nil
	}
	return append(ed25519.PublicKey(nil), key.Public().(ed25519.PublicKey)...)
}

func (i nodeAccessGrantIssuer) IssueAccessGrant(payload *identityprotocol.AccessGrantPayload) (*identityaccess.Artifact, error) {
	return identityaccess.SignAccessGrant(payload, i.privateKey())
}

func (i nodeAccessGrantIssuer) IssueAccessGrantRevocation(payload *identityprotocol.AccessGrantRevocationPayload, grant *identityaccess.Artifact) (*identityaccess.Artifact, error) {
	return identityaccess.SignAccessGrantRevocation(payload, i.privateKey(), payload.GetRevokedAt().AsTime(), grant)
}

func (i nodeAccessGrantIssuer) IssueDeviceRevocation(payload *identityprotocol.DeviceRevocationPayload) (*identityaccess.Artifact, error) {
	return identityaccess.SignDeviceRevocation(payload, i.privateKey(), payload.GetRevokedAt().AsTime())
}

func newProtectedOperatorHandler(process Owners, cfg runtimeConfig, factory LocalAPIHandlerFactory) (http.Handler, error) {
	if process.PrincipalAccess == nil || factory == nil {
		return nil, fmt.Errorf("Operator access dependencies are required")
	}
	node := process.Node.GetNodeRuntime().Identity.Principal
	peer := sha256.Sum256([]byte("ardents:operator-unix-peer-fallback:v1\x00" + filepath.Clean(cfg.SocketPath)))
	sourceDigest := sha256.Sum256(append([]byte("ardents:operator-unix-source:v1\x00"), peer[:]...))
	var source identityaccess.SourceKey
	copy(source[:], sourceDigest[:])
	_, handler, err := factory(process, LocalAPIConfig{
		TargetNode: process.Node.GetNodeRuntime().Node.Name,
		TargetID:   node, Protected: true, PeerBinding: peer, Source: source,
	})
	if err != nil {
		return nil, err
	}
	return limitLocalAPIHandler(handler, localAPIMaxBodyBytes, localAPIRequestTimeout), nil
}

func closeIdentityAccess(identityAccess *storage.Handle) error {
	if err := identityAccess.Close(context.Background()); err != nil {
		return fmt.Errorf("close identity access database: %w", err)
	}
	return nil
}

func serveAndDrain(ctx context.Context, stop context.CancelFunc, servers []*http.Server, targets ...serveTarget) error {
	errorsChannel := make(chan error, len(targets))
	for _, target := range targets {
		go func(current serveTarget) {
			err := current.serve()
			if errors.Is(err, http.ErrServerClosed) {
				err = nil
			}
			errorsChannel <- err
		}(target)
	}
	completed := 0
	var returnErr error
	select {
	case err := <-errorsChannel:
		completed++
		returnErr = errors.Join(returnErr, err)
	case <-ctx.Done():
	}
	stop()

	shutdownErrors := make(chan error, len(servers))
	for _, server := range servers {
		go func(current *http.Server) {
			shutdownErrors <- current.Shutdown(context.Background())
		}(server)
	}
	for range servers {
		returnErr = errors.Join(returnErr, <-shutdownErrors)
	}
	for completed < len(targets) {
		returnErr = errors.Join(returnErr, <-errorsChannel)
		completed++
	}
	return returnErr
}

func newProtectedApplicationHandler(process Owners, cfg runtimeConfig, factory ApplicationAPIHandlerFactory) (string, http.Handler, error) {
	if process.PrincipalAccess == nil || factory == nil {
		return "", nil, fmt.Errorf("Application Principal access dependencies are required")
	}
	node := process.Node.GetNodeRuntime().Identity.Principal
	peer := sha256.Sum256([]byte("ardents:application-unix-peer-fallback:v1\x00" + filepath.Clean(cfg.ApplicationSocketPath)))
	sourceDigest := sha256.Sum256(append([]byte("ardents:application-unix-source:v1\x00"), peer[:]...))
	var source identityaccess.SourceKey
	copy(source[:], sourceDigest[:])
	path, handler, err := factory(process, ApplicationAPIConfig{
		TargetID: node, Protected: true, PeerBinding: peer, Source: source,
	})
	if err != nil {
		return "", nil, err
	}
	return path, limitLocalAPIHandler(handler, int64(applicationv1.MaxUnaryMessageBytes), localAPIRequestTimeout), nil
}
