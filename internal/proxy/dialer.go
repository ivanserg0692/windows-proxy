package proxy

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"sync"
	"time"

	"wslproxy/internal/config"
)

type ResolveFunc func(context.Context) (netip.Addr, error)
type dialFunc func(context.Context, string, string) (net.Conn, error)

type connectDialer struct {
	config  config.Config
	stats   *Stats
	resolve ResolveFunc
	dial    dialFunc
	mu      sync.Mutex
	ip      netip.Addr
}

func (d *connectDialer) DialContext(ctx context.Context, network, _ string) (net.Conn, error) {
	d.mu.Lock()
	ip := d.ip
	d.mu.Unlock()
	var lastConnectErr error
	for attempt := 0; attempt <= d.config.ConnectRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if attempt > 0 {
			timer := time.NewTimer(d.config.RetryDelay())
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			case <-timer.C:
			}
			if d.resolve != nil {
				// Refresh only between failed TCP connects. Discovery never starts a new retry budget.
				refreshedIP, err := d.resolve(ctx)
				if err != nil {
					return nil, fmt.Errorf("refresh WSL IP after TCP failure: %w (previous connect: %v)", err, lastConnectErr)
				}
				if err := d.config.CheckTarget(refreshedIP); err != nil {
					return nil, err
				}
				ip = refreshedIP
			}
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			d.stats.retries.Add(1)
		}
		attemptCtx, cancel := context.WithTimeout(ctx, d.config.ConnectTimeout())
		conn, err := d.dial(attemptCtx, network, d.config.TargetEndpoint(ip))
		cancel()
		if err == nil {
			d.mu.Lock()
			d.ip = ip
			d.mu.Unlock()
			d.stats.setUpstream(ip)
			return conn, nil
		}
		lastConnectErr = err
	}
	return nil, fmt.Errorf("TCP connect failed after %d attempt(s): %w", d.config.ConnectRetries+1, lastConnectErr)
}
