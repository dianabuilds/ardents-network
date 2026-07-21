package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ardents/internal/buildinfo"
	applicationv1 "ardents/sdk/go/protocol/applicationv1"
)

type LocalAPIConfig struct {
	Token        string
	SubjectID    string
	Capabilities []string
	ExpiresAt    time.Time
	TargetNode   string
	TargetID     string
}

type LocalAPIHandlerFactory func(Owners, LocalAPIConfig) (string, http.Handler, error)

type ApplicationAPIConfig struct {
	Token        string
	Subject      string
	Capabilities []string
	ExpiresAt    time.Time
}

type ApplicationAPIHandlerFactory func(Owners, ApplicationAPIConfig) (string, http.Handler, error)

type OperatorSurface interface {
	Handler() http.Handler
	Middleware(http.Handler) http.Handler
}

type OperatorSurfaceFactory func(Owners, string) (OperatorSurface, error)

func Run(localAPI LocalAPIHandlerFactory, applicationAPI ApplicationAPIHandlerFactory, operatorSurface OperatorSurfaceFactory) error {
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

	cfg, err := loadRuntimeConfig()
	if err != nil {
		return fmt.Errorf("load runtime config: %w", err)
	}

	process := NewOwners(cfg.Node)
	n := process.Node

	if err := n.Start(ctx); err != nil {
		return fmt.Errorf("start node: %w", err)
	}
	defer stopNode(n)

	if operatorSurface == nil {
		return fmt.Errorf("configure observability: operator surface factory is required")
	}
	surface, err := operatorSurface(process, cfg.ObservabilityToken)
	if err != nil {
		return fmt.Errorf("configure observability: %w", err)
	}
	server, path, err := newServer(process, cfg, surface, localAPI)
	if err != nil {
		return fmt.Errorf("configure server: %w", err)
	}
	observabilityServer := newHTTPServer(cfg.ObservabilityAddr, surface.Handler())
	targets := []serveTarget{{serve: server.ListenAndServe}}
	servers := []*http.Server{server, observabilityServer}
	if cfg.SocketPath != "" {
		socketServer, socketListener, socketErr := newUnixHTTPServer(cfg.SocketPath, server.Handler)
		if socketErr != nil {
			return fmt.Errorf("configure local API socket: %w", socketErr)
		}
		servers = append(servers, socketServer)
		targets = append(targets, serveTarget{serve: func() error { return socketServer.Serve(socketListener) }})
		slog.Info("ardentsd local control socket ready", "path", cfg.SocketPath)
	}
	if cfg.ApplicationEnabled {
		applicationPath, applicationHandler, applicationErr := newApplicationAPIHandler(process, cfg, applicationAPI)
		if applicationErr != nil {
			return fmt.Errorf("configure Application Interface: %w", applicationErr)
		}
		if cfg.ApplicationListenAddr != "" {
			applicationServer := newHTTPServer(cfg.ApplicationListenAddr, applicationHandler)
			servers = append(servers, applicationServer)
			targets = append(targets, serveTarget{serve: applicationServer.ListenAndServe})
			slog.Info("ardentsd Application Interface listening", "addr", applicationServer.Addr, "path", applicationPath)
		}
		if cfg.ApplicationSocketPath != "" {
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
	for _, current := range servers {
		go shutdownServerOnContext(ctx, current)
	}

	slog.Info("ardentsd listening", "addr", server.Addr, "path", path)
	slog.Info("ardentsd observability listening", "addr", observabilityServer.Addr)
	if err := serveUntilFailure(targets...); err != nil {
		stop()
		return fmt.Errorf("serve daemon: %w", err)
	}
	return nil
}

func stopNode(n *Node) {
	if err := n.Stop(context.Background()); err != nil {
		log.Printf("stop node: %v", err)
	}
}

func newServer(process Owners, cfg runtimeConfig, surface OperatorSurface, localAPI LocalAPIHandlerFactory) (*http.Server, string, error) {
	if err := validateLocalAPIListenAddr(cfg.ListenAddr); err != nil {
		return nil, "", err
	}
	mux := http.NewServeMux()
	path, handler, err := newLocalAPIHandler(process, cfg, localAPI)
	if err != nil {
		return nil, "", err
	}
	mux.Handle(path, surface.Middleware(handler))
	return newHTTPServer(cfg.ListenAddr, mux), path, nil
}

type serveTarget struct {
	serve func() error
}

func serveUntilFailure(targets ...serveTarget) error {
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
	return <-errorsChannel
}

func newLocalAPIHandler(process Owners, cfg runtimeConfig, factory LocalAPIHandlerFactory) (string, http.Handler, error) {
	if factory == nil {
		return "", nil, fmt.Errorf("local API handler factory is required")
	}
	runtime := process.Node
	target := runtime.GetNodeRuntime()
	path, handler, err := factory(process, LocalAPIConfig{
		Token: cfg.APIToken, SubjectID: cfg.APISubject, Capabilities: cfg.APICapabilities,
		ExpiresAt: cfg.APICredentialEnd, TargetNode: target.Node.Name, TargetID: target.Identity.Principal,
	})
	if err != nil {
		return "", nil, err
	}
	return path, limitLocalAPIHandler(handler, localAPIMaxBodyBytes, localAPIRequestTimeout), nil
}

func newApplicationAPIHandler(process Owners, cfg runtimeConfig, factory ApplicationAPIHandlerFactory) (string, http.Handler, error) {
	if factory == nil {
		return "", nil, fmt.Errorf("Application Interface handler factory is required")
	}
	path, handler, err := factory(process, ApplicationAPIConfig{
		Token: cfg.ApplicationToken, Subject: cfg.ApplicationSubject,
		Capabilities: cfg.ApplicationCapabilities, ExpiresAt: cfg.ApplicationCredentialEnd,
	})
	if err != nil {
		return "", nil, err
	}
	return path, limitLocalAPIHandler(handler, int64(applicationv1.MaxUnaryMessageBytes), localAPIRequestTimeout), nil
}

func shutdownServerOnContext(ctx context.Context, server *http.Server) {
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown server: %v", err)
	}
}
