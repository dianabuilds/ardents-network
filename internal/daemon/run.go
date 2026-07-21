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

type OperatorSurface interface {
	Handler() http.Handler
	Middleware(http.Handler) http.Handler
}

type OperatorSurfaceFactory func(Owners, string) (OperatorSurface, error)

func Run(localAPI LocalAPIHandlerFactory, operatorSurface OperatorSurfaceFactory) {
	if len(os.Args) == 2 && (os.Args[1] == "version" || os.Args[1] == "--version") {
		encoded, err := json.Marshal(buildinfo.Current())
		if err != nil {
			_, _ = fmt.Fprintln(os.Stderr, "encode build info")
			return
		}
		_, _ = fmt.Println(string(encoded))
		return
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := loadRuntimeConfig()
	if err != nil {
		log.Fatalf("load runtime config: %v", err)
	}

	process := NewOwners(cfg.Node)
	n := process.Node

	if err := n.Start(ctx); err != nil {
		log.Fatalf("start node: %v", err)
	}
	defer stopNode(n)

	if operatorSurface == nil {
		log.Printf("configure observability: operator surface factory is required")
		return
	}
	surface, err := operatorSurface(process, cfg.ObservabilityToken)
	if err != nil {
		log.Printf("configure observability: %v", err)
		return
	}
	server, path, err := newServer(process, cfg, surface, localAPI)
	if err != nil {
		log.Printf("configure server: %v", err)
		return
	}
	observabilityServer := newHTTPServer(cfg.ObservabilityAddr, surface.Handler())
	go shutdownServerOnContext(ctx, server)
	go shutdownServerOnContext(ctx, observabilityServer)

	slog.Info("ardd listening", "addr", server.Addr, "path", path)
	slog.Info("ardd observability listening", "addr", observabilityServer.Addr)
	if err := serveUntilFailure(server, observabilityServer); err != nil {
		log.Printf("serve daemon: %v", err)
		stop()
	}
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

func serveUntilFailure(servers ...*http.Server) error {
	errorsChannel := make(chan error, len(servers))
	for _, server := range servers {
		go func(current *http.Server) {
			err := current.ListenAndServe()
			if errors.Is(err, http.ErrServerClosed) {
				err = nil
			}
			errorsChannel <- err
		}(server)
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

func shutdownServerOnContext(ctx context.Context, server *http.Server) {
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown server: %v", err)
	}
}
