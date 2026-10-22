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
	http      *http.Server
	transport *transport
	listener  net.Listener
	stats     *Stats
	done      chan struct{}
	mu        sync.Mutex
	err       error
	cancel    context.CancelFunc
}

func Start(c config.Config, ip netip.Addr, resolve ResolveFunc, logger *slog.Logger) (*Server, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	if err := c.CheckTarget(ip); err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp4", c.ListenEndpoint())
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w", c.ListenEndpoint(), err)
	}
	stats := &Stats{}
	stats.setUpstream(ip)
	dialer := &connectDialer{config: c, ip: ip, stats: stats, resolve: resolve, dial: (&net.Dialer{KeepAlive: 30 * time.Second}).DialContext}
	t := newTransport(dialer, c.TargetEndpoint(ip))
	upstream := &url.URL{Scheme: "http", Host: c.TargetEndpoint(ip)}
	proxy := &httputil.ReverseProxy{
		Transport: t,
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(upstream)
			r.Out.Host = r.In.Host
			r.SetXForwarded()
		},
		ErrorLog: log.New(&logWriter{logger: logger}, "", 0),
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			stats.failures.Add(1)
			logger.Error("proxy_request_failed", "method", r.Method, "error", err.Error())
			http.Error(w, "WSL upstream unavailable", http.StatusBadGateway)
		},
	}
	baseCtx, cancel := context.WithCancel(context.Background())
	s := &Server{transport: t, listener: listener, stats: stats, done: make(chan struct{}), cancel: cancel}
	s.http = &http.Server{
		ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1024 * 1024,
		BaseContext: func(net.Listener) context.Context { return baseCtx },
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
		err := s.http.Serve(listener)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		s.mu.Lock()
		s.err = err
		s.mu.Unlock()
		close(s.done)
	}()
	return s, nil
}

func (s *Server) Snapshot() Snapshot    { return s.stats.Snapshot() }
func (s *Server) Done() <-chan struct{} { return s.done }
func (s *Server) Err() error            { s.mu.Lock(); defer s.mu.Unlock(); return s.err }
func (s *Server) Address() string       { return s.listener.Addr().String() }

func (s *Server) Stop(ctx context.Context) error {
	err := s.http.Shutdown(ctx)
	if err != nil {
		s.cancel()
		s.http.Close()
	}
	s.cancel()
	s.transport.close()
	<-s.done
	return err
}

type logWriter struct{ logger *slog.Logger }

func (w *logWriter) Write(p []byte) (int, error) {
	w.logger.Error("http_server", "message", string(p))
	return len(p), nil
}
