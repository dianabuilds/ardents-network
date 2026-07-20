package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	connect "ardents/internal/transport/connectrpc"
)

const (
	localAPIMaxBodyBytes   int64 = 1 << 20
	localAPIRequestTimeout       = 30 * time.Second
	localAPIReadTimeout          = 15 * time.Second
	localAPIWriteTimeout         = 35 * time.Second
	localAPIIdleTimeout          = 60 * time.Second
	localAPIHeaderTimeout        = 5 * time.Second
	localAPIMaxHeaderBytes       = 16 << 10
)

func loadAPIToken() (string, error) {
	token := strings.TrimSpace(os.Getenv(apiTokenEnv))
	path := strings.TrimSpace(os.Getenv(apiTokenFileEnv))
	if token != "" && path != "" {
		return "", fmt.Errorf("configure only one of %s and %s", apiTokenEnv, apiTokenFileEnv)
	}
	if token != "" {
		return token, nil
	}
	if path != "" {
		return readAPITokenFile(path)
	}
	return "", fmt.Errorf("%s or %s is required", apiTokenEnv, apiTokenFileEnv)
}

func readAPITokenFile(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", fmt.Errorf("inspect api token file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("api token file must be a regular file")
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		return "", fmt.Errorf("api token file permissions must not allow group or other access")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read api token file: %w", err)
	}
	token := strings.TrimSpace(string(data))
	if token == "" {
		return "", fmt.Errorf("api token file is empty")
	}
	return token, nil
}

func validateLocalAPIListenAddr(addr string) error {
	host, portText, err := net.SplitHostPort(strings.TrimSpace(addr))
	if err != nil {
		return fmt.Errorf("invalid local api address: %w", err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 0 || port > 65535 {
		return fmt.Errorf("invalid local api port")
	}
	if strings.EqualFold(host, "localhost") {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("plaintext local api must bind to a loopback address; secure remote mode is not configured")
	}
	return nil
}

func localAdminCapabilities() []string {
	return connect.OperatorActions()
}

func newHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: localAPIHeaderTimeout,
		ReadTimeout:       localAPIReadTimeout,
		WriteTimeout:      localAPIWriteTimeout,
		IdleTimeout:       localAPIIdleTimeout,
		MaxHeaderBytes:    localAPIMaxHeaderBytes,
	}
}

func limitLocalAPIHandler(handler http.Handler, maxBodyBytes int64, timeout time.Duration) http.Handler {
	timed := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/StreamNodeEvents") {
			_ = http.NewResponseController(w).SetWriteDeadline(time.Time{})
			handler.ServeHTTP(w, r)
			return
		}
		http.TimeoutHandler(handler, timeout, "request timeout").ServeHTTP(w, r)
	})
	limited := http.MaxBytesHandler(timed, maxBodyBytes)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ContentLength > maxBodyBytes {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		limited.ServeHTTP(w, r)
	})
}
