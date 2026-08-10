package siteexperiment

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"
)

func runResolutionRoleServer(ctx context.Context, handler http.Handler, completed <-chan struct{}) error {
	if ctx == nil || handler == nil || completed == nil {
		return errors.New("resolution role server contract is incomplete")
	}
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", ":8080")
	if err != nil {
		return err
	}
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second}
	served := make(chan error, 1)
	go func() { served <- server.Serve(listener) }()
	select {
	case <-ctx.Done():
		closeErr := server.Close()
		return errors.Join(ctx.Err(), closeErr, normalizeRoleServeError(<-served))
	case serveErr := <-served:
		return errors.Join(errors.New("resolution role server exited before completion"), normalizeRoleServeError(serveErr))
	case <-completed:
	}
	shutdownContext, cancel := context.WithTimeout(context.Background(), time.Second)
	shutdownErr := server.Shutdown(shutdownContext)
	cancel()
	var closeErr error
	if shutdownErr != nil {
		closeErr = server.Close()
	}
	select {
	case serveErr := <-served:
		return errors.Join(shutdownErr, closeErr, normalizeRoleServeError(serveErr))
	case <-time.After(time.Second):
		return errors.Join(shutdownErr, closeErr, errors.New("resolution role server did not stop"))
	}
}

func normalizeRoleServeError(err error) error {
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
