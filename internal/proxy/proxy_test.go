package proxy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"wslproxy/internal/config"
)

func testConfig(t *testing.T, target string) config.Config {
	t.Helper()
	_, port, err := net.SplitHostPort(target)
	if err != nil {
		t.Fatal(err)
	}
	targetPort, err := strconv.Atoi(port)
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	listenPort := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	c := config.Default()
	c.Distribution = "Ubuntu"
	c.ListenAddress = "127.0.0.1"
	c.ListenPort = listenPort
	c.TargetPort = targetPort
	return c
}

func startTestProxy(t *testing.T, c config.Config) *Server {
	t.Helper()
	s, err := Start(c, netip.MustParseAddr("127.0.0.1"), nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		s.Stop(ctx)
	})
	return s
}

func TestKeepAliveAndForwarding(t *testing.T) {
	var connections atomic.Int32
	upstream := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RequestURI() != "/path?q=one%20two" {
			t.Errorf("path changed: %s", r.URL.RequestURI())
		}
		if r.Host != "example.test" {
			t.Errorf("Host changed: %s", r.Host)
		}
		if r.Header.Get("X-Forwarded-For") != "127.0.0.1" {
			t.Errorf("untrusted forwarding header: %s", r.Header.Get("X-Forwarded-For"))
		}
		w.Write([]byte("hello"))
	}))
	upstream.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			connections.Add(1)
		}
	}
	upstream.Start()
	defer upstream.Close()
	s := startTestProxy(t, testConfig(t, upstream.Listener.Addr().String()))
	client := &http.Client{Timeout: 3 * time.Second}
	for i := 0; i < 3; i++ {
		req, _ := http.NewRequest("GET", "http://"+s.Address()+"/path?q=one%20two", nil)
		req.Host = "example.test"
		req.Header.Set("X-Forwarded-For", "untrusted")
		response, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(response.Body)
		response.Body.Close()
		if err != nil || string(body) != "hello" {
			t.Fatalf("response %q %v", body, err)
		}
	}
	if connections.Load() != 1 {
		t.Fatalf("keep-alive not used: %d connections", connections.Load())
	}
	stats := s.Snapshot()
	if stats.Requests != 3 || stats.Retries != 0 || stats.Failures != 0 {
		t.Fatalf("stats: %+v", stats)
	}
}

func TestNeverReplayRequestAfterUpstreamDisconnect(t *testing.T) {
	for _, tc := range []struct{ method, body, key string }{{"POST", "payment=123", ""}, {"GET", "", ""}, {"POST", "", "operation-123"}} {
		t.Run(tc.method+tc.key, func(t *testing.T) {
			var received atomic.Int32
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/warm" {
					w.Write([]byte("warm"))
					return
				}
				io.Copy(io.Discard, r.Body)
				received.Add(1)
				conn, _, err := w.(http.Hijacker).Hijack()
				if err != nil {
					t.Error(err)
					return
				}
				conn.Close()
			}))
			defer upstream.Close()
			s := startTestProxy(t, testConfig(t, upstream.Listener.Addr().String()))
			client := &http.Client{Timeout: 3 * time.Second}
			response, err := client.Get("http://" + s.Address() + "/warm")
			if err != nil {
				t.Fatal(err)
			}
			io.Copy(io.Discard, response.Body)
			response.Body.Close()
			req, _ := http.NewRequest(tc.method, "http://"+s.Address()+"/drop", strings.NewReader(tc.body))
			if tc.key != "" {
				req.Header.Set("Idempotency-Key", tc.key)
			}
			response, err = client.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			io.Copy(io.Discard, response.Body)
			response.Body.Close()
			if response.StatusCode != http.StatusBadGateway {
				t.Fatalf("status: %d", response.StatusCode)
			}
			if received.Load() != 1 {
				t.Fatalf("request replayed %d times", received.Load())
			}
			stats := s.Snapshot()
			if stats.Retries != 0 || stats.Failures != 1 {
				t.Fatalf("stats: %+v", stats)
			}
		})
	}
}

func TestTCPRetryBudgetIncludesRefresh(t *testing.T) {
	c := config.Default()
	c.Distribution = "Ubuntu"
	c.ConnectRetries = 2
	c.RetryDelayMS = 0
	var calls, refreshes int
	stats := &Stats{}
	d := &connectDialer{config: c, stats: stats, ip: netip.MustParseAddr("172.28.1.2"),
		resolve: func(context.Context) (netip.Addr, error) { refreshes++; return netip.MustParseAddr("172.28.1.3"), nil },
		dial: func(_ context.Context, _, address string) (net.Conn, error) {
			calls++
			if calls > 1 && address != "172.28.1.3:8001" {
				t.Fatalf("stale IP: %s", address)
			}
			return nil, errors.New("connect refused")
		},
	}
	if _, err := d.DialContext(context.Background(), "tcp4", ""); err == nil {
		t.Fatal("expected connect error")
	}
	if calls != 3 || refreshes != 2 || stats.Snapshot().Retries != 2 {
		t.Fatalf("budget exceeded: calls=%d refresh=%d stats=%+v", calls, refreshes, stats.Snapshot())
	}
}

func TestCancelStopsRetryDelay(t *testing.T) {
	c := config.Default()
	c.Distribution = "Ubuntu"
	c.RetryDelayMS = 10000
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	d := &connectDialer{config: c, stats: &Stats{}, ip: netip.MustParseAddr("172.28.1.2"), dial: func(context.Context, string, string) (net.Conn, error) {
		calls++
		cancel()
		return nil, errors.New("refused")
	}}
	start := time.Now()
	_, err := d.DialContext(ctx, "tcp4", "")
	if !errors.Is(err, context.Canceled) || calls != 1 || time.Since(start) > time.Second {
		t.Fatalf("cancellation ignored: %d %v", calls, err)
	}
}

func TestSuccessfulConnectDoesNotRetry(t *testing.T) {
	c := config.Default()
	c.Distribution = "Ubuntu"
	left, right := net.Pipe()
	defer right.Close()
	calls := 0
	stats := &Stats{}
	d := &connectDialer{config: c, stats: stats, ip: netip.MustParseAddr("172.28.1.2"), dial: func(context.Context, string, string) (net.Conn, error) { calls++; return left, nil }}
	conn, err := d.DialContext(context.Background(), "tcp4", "")
	if err != nil {
		t.Fatal(err)
	}
	conn.Close()
	if calls != 1 || stats.Snapshot().Retries != 0 {
		t.Fatal("successful connect retried")
	}
}

func TestConcurrentRequestsAndStopReleasesPort(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "ok") }))
	defer upstream.Close()
	c := testConfig(t, upstream.Listener.Addr().String())
	s := startTestProxy(t, c)
	client := &http.Client{Timeout: 5 * time.Second}
	var wg sync.WaitGroup
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			response, err := client.Get("http://" + s.Address())
			if err != nil {
				t.Error(err)
				return
			}
			io.Copy(io.Discard, response.Body)
			response.Body.Close()
			if response.StatusCode != 200 {
				t.Errorf("status %d", response.StatusCode)
			}
		}()
	}
	wg.Wait()
	if s.Snapshot().Requests != 64 {
		t.Fatalf("stats: %+v", s.Snapshot())
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := s.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp4", c.ListenEndpoint())
	if err != nil {
		t.Fatalf("port not released: %v", err)
	}
	listener.Close()
}

func TestClientCancellationReachesUpstream(t *testing.T) {
	entered := make(chan struct{})
	canceled := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(entered); <-r.Context().Done(); close(canceled) }))
	defer upstream.Close()
	s := startTestProxy(t, testConfig(t, upstream.Listener.Addr().String()))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", "http://"+s.Address(), nil)
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		response, _ := http.DefaultClient.Do(req)
		if response != nil {
			response.Body.Close()
		}
	}()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("upstream did not receive request")
	}
	cancel()
	select {
	case <-canceled:
	case <-time.After(3 * time.Second):
		t.Fatal("upstream request was not canceled")
	}
	select {
	case <-finished:
	case <-time.After(3 * time.Second):
		t.Fatal("client did not finish")
	}
}
