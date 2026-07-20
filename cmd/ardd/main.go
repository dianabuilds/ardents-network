package main

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
	"ardents/internal/observability"
	runtimeprocess "ardents/internal/runtime/process"
	connect "ardents/internal/transport/connectrpc"
)

func main() {
	if len(os.Args) == 2 && (os.Args[1] == "version" || os.Args[1] == "--version") {
		encoded, err := json.Marshal(buildinfo.Current())
		if err != nil {
			fmt.Fprintln(os.Stderr, "encode build info")
			return
		}
		fmt.Println(string(encoded))
		return
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := loadRuntimeConfig()
	if err != nil {
		log.Fatalf("load runtime config: %v", err)
	}

	n := newRuntime(cfg.Node)

	if err := n.Start(ctx); err != nil {
		log.Fatalf("start node: %v", err)
	}
	defer stopNode(n)

	surface, err := observability.NewSurface(n, cfg.ObservabilityToken)
	if err != nil {
		log.Printf("configure observability: %v", err)
		return
	}
	server, path, err := newServer(n, cfg, surface)
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

func stopNode(n runtimeprocess.NodeRuntime) {
	if err := n.Stop(context.Background()); err != nil {
		log.Printf("stop node: %v", err)
	}
}

func newServer(n runtimeprocess.NodeRuntime, cfg runtimeConfig, surface *observability.Surface) (*http.Server, string, error) {
	if err := validateLocalAPIListenAddr(cfg.ListenAddr); err != nil {
		return nil, "", err
	}
	mux := http.NewServeMux()
	path, handler, err := newLocalAPIHandler(n, cfg)
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

func newLocalAPIHandler(runtime runtimeprocess.NodeRuntime, cfg runtimeConfig) (string, http.Handler, error) {
	target := runtime.GetNodeRuntime()
	path, handler, err := connect.NewHandler(connect.Dependencies{
		Node:          runtime,
		Discovery:     runtime,
		Diagnostics:   runtime,
		Workload:      runtime,
		Hosting:       runtime,
		Data:          runtime,
		Configuration: runtime,
		Audit:         runtime,
	}, connect.AuthConfig{
		Token: cfg.APIToken, SubjectID: cfg.APISubject, Capabilities: cfg.APICapabilities,
		ExpiresAt: cfg.APICredentialEnd, TargetNode: target.Node.Name, TargetPrincipal: target.Identity.Principal,
	})
	if err != nil {
		return "", nil, err
	}
	return path, limitLocalAPIHandler(handler, localAPIMaxBodyBytes, localAPIRequestTimeout), nil
}

func listenAddr() string {
	addr := os.Getenv("ARDENTS_ADDR")
	if addr == "" {
		return "127.0.0.1:8080"
	}
	return addr
}

func shutdownServerOnContext(ctx context.Context, server *http.Server) {
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown server: %v", err)
	}
}
