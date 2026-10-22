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
	c := config.Default()
	c.Distribution = "Ubuntu"
	c.ManualTargetIP = "127.0.0.1"
	c.TargetPort, _ = strconv.Atoi(port)
	r := &runner{}
	s := &Service{WSL: wsl.Client{Runner: r}}
	report, err := s.Diagnose(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	if report.Selected != netip.MustParseAddr("127.0.0.1") || len(report.Probes) != 1 || report.Probes[0].Error != "" {
		t.Fatalf("wrong report: %+v", report)
	}
	if len(r.calls) != 1 {
		t.Fatalf("manual IP unexpectedly ran discovery: %v", r.calls)
	}
}

func TestRejectSelfProxy(t *testing.T) {
	c := config.Default()
	c.Distribution = "Ubuntu"
	c.ManualTargetIP = "127.0.0.1"
	c.TargetPort = c.ListenPort
	s := &Service{WSL: wsl.Client{Runner: &runner{}}}
	report, err := s.Diagnose(context.Background(), c)
	if err == nil || len(report.Probes) != 1 || report.Probes[0].Error == "" {
		t.Fatalf("self proxy accepted: %+v %v", report, err)
	}
}
