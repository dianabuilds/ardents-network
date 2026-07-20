package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"ardents/internal/workload/ingressproxy"
)

func main() {
	target := flag.String("target", "", "fixed internal workload DNS target")
	ports := flag.String("ports", "", "comma-separated admitted TCP ports")
	flag.Parse()
	parsed, err := parsePorts(*ports)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	if err := ingressproxy.Run(ctx, *target, parsed); err != nil && ctx.Err() == nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
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
