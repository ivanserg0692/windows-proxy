package proxy

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"
)

const maxConnections = 32
const idleTimeout = 60 * time.Second

// transport sends each HTTP request exactly once. Transport.RoundTrip is deliberately
// not used: its automatic retries include failures after requests have been written.
// ClientConn supplies Go's HTTP implementation while this small pool owns reuse.
type transport struct {
	base        *http.Transport
	address     string
	mu          sync.Mutex
	connections map[*http.ClientConn]time.Time
	creating    int
	closed      bool
	changed     chan struct{}
}

func newTransport(d *connectDialer, address string) *transport {
	protocols := new(http.Protocols)
	protocols.SetHTTP1(true)
	return &transport{
		base: &http.Transport{DialContext: d.DialContext, Proxy: nil, Protocols: protocols,
			DisableCompression: true, ResponseHeaderTimeout: 30 * time.Second,
			MaxResponseHeaderBytes: 1024 * 1024},
		address: address, connections: make(map[*http.ClientConn]time.Time), changed: make(chan struct{}, 1),
	}
}

func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	cc, err := t.acquire(req.Context())
	if err != nil {
		if req.Body != nil {
			req.Body.Close()
		}
		return nil, err
	}
	response, err := cc.RoundTrip(req)
	if err != nil {
		t.mu.Lock()
		delete(t.connections, cc)
		t.mu.Unlock()
		cc.Close()
		t.notify()
		return nil, err
	}
	return response, nil
}

func (t *transport) acquire(ctx context.Context) (*http.ClientConn, error) {
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		t.mu.Lock()
		if t.closed {
			t.mu.Unlock()
			return nil, fmt.Errorf("proxy transport is closed")
		}
		for cc, used := range t.connections {
			if cc.Err() != nil || (cc.Available() > 0 && time.Since(used) > idleTimeout) {
				delete(t.connections, cc)
				cc.Close()
				continue
			}
			if cc.Available() > 0 && cc.Reserve() == nil {
				t.connections[cc] = time.Now()
				t.mu.Unlock()
				return cc, nil
			}
		}
		if len(t.connections)+t.creating < maxConnections {
			t.creating++
			t.mu.Unlock()
			cc, err := t.base.NewClientConn(ctx, "http", t.address)
			t.mu.Lock()
			t.creating--
			if err == nil && t.closed {
				cc.Close()
				err = fmt.Errorf("proxy transport is closed")
			}
			if err == nil {
				cc.SetStateHook(func(*http.ClientConn) { t.notify() })
				err = cc.Reserve()
				if err == nil {
					t.connections[cc] = time.Now()
				} else {
					cc.Close()
				}
			}
			t.mu.Unlock()
			t.notify()
			return cc, err
		}
		t.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-t.changed:
		}
	}
}

func (t *transport) notify() {
	select {
	case t.changed <- struct{}{}:
	default:
	}
}

func (t *transport) close() {
	t.mu.Lock()
	t.closed = true
	for cc := range t.connections {
		cc.Close()
		delete(t.connections, cc)
	}
	t.mu.Unlock()
	t.notify()
}
