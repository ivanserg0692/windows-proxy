package proxy

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/netip"
	"net/url"
	"sync"
	"time"

	"wslproxy/internal/config"
)

type Server struct {
	http        *http.Server
	transport   *transport
	listener    net.Listener
	stats       *Stats
	done        chan struct{}
	mu          sync.Mutex
	err         error
	cancel      context.CancelFunc
	connections sync.WaitGroup
}

func Start(cfg config.Config, ip netip.Addr, resolve ResolveFunc, logger *slog.Logger) (*Server, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if err := cfg.CheckTarget(ip); err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp4", cfg.ListenEndpoint())
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w", cfg.ListenEndpoint(), err)
	}
	stats := &Stats{}
	stats.setUpstream(ip)
	dialer := &connectDialer{config: cfg, ip: ip, stats: stats, resolve: resolve, dial: (&net.Dialer{KeepAlive: 30 * time.Second}).DialContext}
	proxyTransport := newTransport(dialer, cfg.TargetEndpoint(ip))
	upstream := &url.URL{Scheme: "http", Host: cfg.TargetEndpoint(ip)}
	proxy := &httputil.ReverseProxy{
		Transport: proxyTransport,
		Rewrite: func(proxyRequest *httputil.ProxyRequest) {
			proxyRequest.SetURL(upstream)
			proxyRequest.Out.Host = proxyRequest.In.Host
			proxyRequest.SetXForwarded()
		},
		ErrorLog: log.New(&logWriter{logger: logger}, "", 0),
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			stats.failures.Add(1)
			logger.Error("proxy_request_failed", "method", r.Method, "error", err.Error())
			http.Error(w, "WSL upstream unavailable", http.StatusBadGateway)
		},
	}
	baseCtx, cancel := context.WithCancel(context.Background())
	server := &Server{transport: proxyTransport, listener: listener, stats: stats, done: make(chan struct{}), cancel: cancel}
	server.http = &http.Server{
		ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1024 * 1024,
		BaseContext: func(net.Listener) context.Context { return baseCtx },
		ConnState:   server.connectionState,
		ErrorLog:    log.New(&logWriter{logger: logger}, "", 0),
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			stats.requests.Add(1)
			defer func() {
				if recovered := recover(); recovered != nil {
					if recovered == http.ErrAbortHandler {
						stats.failures.Add(1)
						logger.Error("proxy_stream_failed")
					}
					panic(recovered)
				}
			}()
			if r.Method == http.MethodConnect {
				http.Error(w, "CONNECT is not supported", http.StatusMethodNotAllowed)
				return
			}
			if r.Header.Get("Upgrade") != "" {
				http.Error(w, "protocol upgrades are not supported", http.StatusNotImplemented)
				return
			}
			proxy.ServeHTTP(w, r)
		}),
	}
	go func() {
		err := server.http.Serve(listener)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		server.mu.Lock()
		server.err = err
		server.mu.Unlock()
		close(server.done)
	}()
	return server, nil
}

func (s *Server) Snapshot() Snapshot    { return s.stats.Snapshot() }
func (s *Server) Done() <-chan struct{} { return s.done }
func (s *Server) Err() error            { s.mu.Lock(); defer s.mu.Unlock(); return s.err }
func (s *Server) Address() string       { return s.listener.Addr().String() }

func (s *Server) connectionState(_ net.Conn, state http.ConnState) {
	switch state {
	case http.StateNew:
		s.connections.Add(1)
		s.stats.activeConnections.Add(1)
	case http.StateClosed:
		s.stats.activeConnections.Add(-1)
		s.connections.Done()
	}
}

func (s *Server) Stop(ctx context.Context) error {
	err := s.http.Shutdown(ctx)
	if err != nil {
		s.cancel()
		s.http.Close()
	}
	s.cancel()
	s.transport.close()
	<-s.done
	// Serve has finished accepting connections. Wait for all closed-state hooks,
	// which can still be running after Shutdown or Close returns.
	s.connections.Wait()
	return err
}

type logWriter struct{ logger *slog.Logger }

func (w *logWriter) Write(p []byte) (int, error) {
	w.logger.Error("http_server", "message", string(p))
	return len(p), nil
}
