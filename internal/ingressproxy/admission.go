package ingressproxy

import "sync"

type admissionLimiter struct {
	mu           sync.Mutex
	maxTotal     int
	maxPerPort   int
	maxPerSource int
	total        int
	byPort       map[uint16]int
	bySource     map[string]int
}

func newAdmissionLimiter(config Config) *admissionLimiter {
	return &admissionLimiter{
		maxTotal: config.MaxConnections, maxPerPort: config.MaxConnectionsPerPort,
		maxPerSource: config.MaxConnectionsPerSource,
		byPort:       make(map[uint16]int),
		bySource:     make(map[string]int),
	}
}

func (a *admissionLimiter) acquire(port uint16, source string) (func(), EventReason) {
	a.mu.Lock()
	defer a.mu.Unlock()
	switch {
	case a.total >= a.maxTotal:
		return nil, ReasonGlobalLimit
	case a.byPort[port] >= a.maxPerPort:
		return nil, ReasonPortLimit
	case a.bySource[source] >= a.maxPerSource:
		return nil, ReasonSourceLimit
	}
	a.total++
	a.byPort[port]++
	a.bySource[source]++
	return func() {
		a.mu.Lock()
		defer a.mu.Unlock()
		a.total--
		a.byPort[port]--
		a.bySource[source]--
	}, EventReason("")
}
