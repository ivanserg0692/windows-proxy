package app

import (
	"context"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"testing"

	"wslproxy/internal/config"
	"wslproxy/internal/wsl"
)

type runner struct{ calls [][]string }

func (r *runner) Run(_ context.Context, args ...string) ([]byte, error) {
	r.calls = append(r.calls, args)
	if strings.Join(args, " ") == "--list --quiet" {
		return []byte("Ubuntu"), nil
	}
	return nil, nil
}

func TestManualIPDiagnosticUsesWindowsTCP(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			conn.Close()
		}
	}()
	_, port, _ := net.SplitHostPort(listener.Addr().String())
	cfg := config.Default()
	cfg.Distribution = "Ubuntu"
	cfg.ManualTargetIP = "127.0.0.1"
	cfg.TargetPort, _ = strconv.Atoi(port)
	commandRunner := &runner{}
	service := &Service{WSL: wsl.Client{Runner: commandRunner}}
	report, err := service.Diagnose(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if report.Selected != netip.MustParseAddr("127.0.0.1") || len(report.Probes) != 1 || report.Probes[0].Error != "" {
		t.Fatalf("wrong report: %+v", report)
	}
	if len(commandRunner.calls) != 1 {
		t.Fatalf("manual IP unexpectedly ran discovery: %v", commandRunner.calls)
	}
}

func TestRejectSelfProxy(t *testing.T) {
	cfg := config.Default()
	cfg.Distribution = "Ubuntu"
	cfg.ManualTargetIP = "127.0.0.1"
	cfg.TargetPort = cfg.ListenPort
	service := &Service{WSL: wsl.Client{Runner: &runner{}}}
	report, err := service.Diagnose(context.Background(), cfg)
	if err == nil || len(report.Probes) != 1 || report.Probes[0].Error == "" {
		t.Fatalf("self proxy accepted: %+v %v", report, err)
	}
}
