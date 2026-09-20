package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sync"
)

type relayTraffic struct {
	mu                         sync.Mutex
	upstreamBytes, clientBytes uint64
	overflow                   bool
}

func (traffic *relayTraffic) add(upstream bool, count int64) {
	if count <= 0 {
		return
	}
	traffic.mu.Lock()
	defer traffic.mu.Unlock()
	target := &traffic.clientBytes
	if upstream {
		target = &traffic.upstreamBytes
	}
	if uint64(count) > math.MaxUint64-*target {
		traffic.overflow = true
		return
	}
	*target += uint64(count)
}

func (traffic *relayTraffic) emit() error {
	traffic.mu.Lock()
	defer traffic.mu.Unlock()
	if traffic.overflow {
		return fmt.Errorf("relay traffic counter overflow")
	}
	return json.NewEncoder(os.Stdout).Encode(struct {
		Kind          string `json:"kind"`
		UpstreamBytes uint64 `json:"upstream_bytes"`
		ClientBytes   uint64 `json:"client_bytes"`
	}{"netem-relay-result", traffic.upstreamBytes, traffic.clientBytes})
}
