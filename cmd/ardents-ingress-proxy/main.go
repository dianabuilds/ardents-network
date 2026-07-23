package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"ardents/internal/ingressproxy"
)

func main() {
	target := flag.String("target", "", "fixed internal workload DNS target")
	ports := flag.String("ports", "", "comma-separated admitted TCP ports")
	dialTimeout := flag.Duration("dial-timeout", ingressproxy.DefaultDialTimeout, "backend connection timeout")
	idleTimeout := flag.Duration("idle-timeout", ingressproxy.DefaultIdleTimeout, "connection inactivity timeout")
	writeTimeout := flag.Duration("write-timeout", ingressproxy.DefaultWriteTimeout, "per-write timeout")
	maxConnections := flag.Int("max-connections", ingressproxy.DefaultMaxConnections, "global active connection limit")
	maxPerPort := flag.Int("max-connections-per-port", ingressproxy.DefaultMaxConnectionsPerPort, "per-port active connection limit")
	maxPerSource := flag.Int("max-connections-per-source", ingressproxy.DefaultMaxConnectionsPerSource, "per-source active connection limit")
	flag.Parse()
	parsed, err := parsePorts(*ports)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	logger := &eventWriter{}
	config := ingressproxy.DefaultConfig(*target, parsed)
	config.DialTimeout = *dialTimeout
	config.IdleTimeout = *idleTimeout
	config.WriteTimeout = *writeTimeout
	config.MaxConnections = *maxConnections
	config.MaxConnectionsPerPort = *maxPerPort
	config.MaxConnectionsPerSource = *maxPerSource
	config.Observe = logger.write
	if err := ingressproxy.RunConfig(ctx, config); err != nil && ctx.Err() == nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

type eventWriter struct {
	mu sync.Mutex
}

func (w *eventWriter) write(event ingressproxy.Event) {
	w.mu.Lock()
	defer w.mu.Unlock()
	_ = json.NewEncoder(os.Stderr).Encode(event)
}

func parsePorts(raw string) ([]uint16, error) {
	parts := strings.Split(raw, ",")
	ports := make([]uint16, 0, len(parts))
	for _, part := range parts {
		value, err := strconv.ParseUint(strings.TrimSpace(part), 10, 16)
		if err != nil {
			return nil, fmt.Errorf("invalid ingress proxy port set")
		}
		ports = append(ports, uint16(value))
	}
	return ports, nil
}
