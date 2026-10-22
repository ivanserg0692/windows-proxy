package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/netip"
	"sync"
	"time"

	"wslproxy/internal/config"
	"wslproxy/internal/proxy"
	"wslproxy/internal/wsl"
)

type Running struct {
	Config config.Config
	Server *proxy.Server
}

type Service struct {
	Store     config.Store
	WSL       wsl.Client
	Logger    *slog.Logger
	operation sync.Mutex
	mu        sync.Mutex
	running   *Running
}

func (s *Service) Start(ctx context.Context) (*Running, Report, error) {
	s.operation.Lock()
	defer s.operation.Unlock()
	s.mu.Lock()
	current := s.running
	s.mu.Unlock()
	if current != nil {
		return current, Report{}, nil
	}
	cfg, err := s.Store.Load()
	if err != nil {
		return nil, Report{}, fmt.Errorf("load saved configuration: %w", err)
	}
	report, err := s.Diagnose(ctx, cfg)
	if err != nil {
		return nil, report, err
	}
	if err := ctx.Err(); err != nil {
		return nil, report, err
	}
	var refresh proxy.ResolveFunc
	if cfg.ManualTargetIP == "" {
		refresh = func(ctx context.Context) (netip.Addr, error) { return s.refresh(ctx, cfg) }
	}
	server, err := proxy.Start(cfg, report.Selected, refresh, s.Logger)
	if err != nil {
		return nil, report, err
	}
	if err := ctx.Err(); err != nil {
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Stop(stopCtx)
		return nil, report, err
	}
	running := &Running{Config: cfg, Server: server}
	s.mu.Lock()
	s.running = running
	s.mu.Unlock()
	s.Logger.Info("proxy_started", "listen", server.Address(), "distribution", cfg.Distribution, "upstream", cfg.TargetEndpoint(report.Selected))
	return running, report, nil
}

func (s *Service) Stop(ctx context.Context) error {
	s.operation.Lock()
	defer s.operation.Unlock()
	s.mu.Lock()
	running := s.running
	s.mu.Unlock()
	if running == nil {
		return nil
	}
	err := running.Server.Stop(ctx)
	s.mu.Lock()
	s.running = nil
	s.mu.Unlock()
	s.Logger.Info("proxy_stopped", "stats", running.Server.Snapshot(), "error", err)
	return err
}
