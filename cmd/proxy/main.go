package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	tea "charm.land/bubbletea/v2"
	"wslproxy/internal/app"
	"wslproxy/internal/config"
	"wslproxy/internal/tui"
	"wslproxy/internal/wsl"
)

func main() { os.Exit(run()) }

func run() int {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	args := os.Args[1:]
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Println("WSL Proxy\n  proxy.exe        Interactive configuration and proxy\n  proxy.exe serve  Use saved configuration; JSON logs\nConfig: %APPDATA%\\WSLProxy\\config.json")
		return 0
	}
	if len(args) > 1 || (len(args) == 1 && args[0] != "serve") {
		logger.Error("invalid_arguments", "usage", "proxy.exe [serve]")
		return 2
	}
	if runtime.GOOS != "windows" {
		logger.Error("unsupported_platform", "required", "windows/amd64")
		return 1
	}
	store, err := config.UserStore()
	if err != nil {
		logger.Error("config_path_failed", "error", err.Error())
		return 1
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	service := &app.Service{Store: store, WSL: wsl.Client{Runner: wsl.OSRunner{}}, Logger: logger}
	defer func() {
		cancel()
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer stopCancel()
		service.Stop(stopCtx)
	}()
	if len(args) == 1 {
		return serve(ctx, service, logger)
	}
	logs := &tui.Logs{}
	service.Logger = logs.Logger()
	_, err = tea.NewProgram(tui.New(ctx, service, logs), tea.WithContext(ctx)).Run()
	if err != nil && !errors.Is(err, tea.ErrInterrupted) && ctx.Err() == nil {
		logger.Error("tui_failed", "error", err.Error())
		return 1
	}
	return 0
}

func serve(ctx context.Context, service *app.Service, logger *slog.Logger) int {
	running, report, err := service.Start(ctx)
	for _, probe := range report.Probes {
		logger.Info("target_probe", "ip", probe.Candidate.IP.String(), "interface", probe.Candidate.Interface, "duration_ms", probe.Duration.Milliseconds(), "error", probe.Error)
	}
	if err != nil {
		logger.Error("startup_failed", "error", err.Error())
		return 1
	}
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return 0
		case <-running.Server.Done():
			logger.Error("server_stopped", "error", running.Server.Err())
			return 1
		case <-ticker.C:
			logger.Info("proxy_stats", "stats", running.Server.Snapshot())
		}
	}
}
