package proxy

import (
	"net/netip"
	"sync"
	"sync/atomic"
)

type Stats struct {
	requests atomic.Uint64
	retries  atomic.Uint64
	failures atomic.Uint64
	mu       sync.RWMutex
	upstream netip.Addr
}

type Snapshot struct {
	Requests uint64 `json:"requests"`
	Retries  uint64 `json:"retries"`
	Failures uint64 `json:"failures"`
	Upstream string `json:"upstream_ip"`
}

func (s *Stats) Snapshot() Snapshot {
	s.mu.RLock()
	ip := s.upstream
	s.mu.RUnlock()
	return Snapshot{Requests: s.requests.Load(), Retries: s.retries.Load(), Failures: s.failures.Load(), Upstream: ip.String()}
}

func (s *Stats) setUpstream(ip netip.Addr) { s.mu.Lock(); s.upstream = ip; s.mu.Unlock() }
