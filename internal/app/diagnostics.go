package app

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"time"

	"wslproxy/internal/config"
	"wslproxy/internal/wsl"
)

type Probe struct {
	Candidate wsl.Candidate
	Duration  time.Duration
	Error     string
}

type Report struct {
	Probes   []Probe
	Selected netip.Addr
}

func (s *Service) candidates(ctx context.Context, c config.Config) ([]wsl.Candidate, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	if c.ManualTargetIP != "" {
		if err := s.WSL.CheckDistribution(ctx, c.Distribution); err != nil {
			return nil, err
		}
		return []wsl.Candidate{{IP: netip.MustParseAddr(c.ManualTargetIP), Interface: "manual", Preferred: true}}, nil
	}
	return s.WSL.Candidates(ctx, c.Distribution)
}

func (s *Service) Diagnose(ctx context.Context, c config.Config) (Report, error) {
	candidates, err := s.candidates(ctx, c)
	if err != nil {
		return Report{}, err
	}
	var report Report
	var reachable []wsl.Candidate
	for _, candidate := range candidates {
		if err := ctx.Err(); err != nil {
			return report, err
		}
		probe := Probe{Candidate: candidate}
		start := time.Now()
		err := c.CheckTarget(candidate.IP)
		if err == nil {
			var conn net.Conn
			conn, err = (&net.Dialer{Timeout: c.ConnectTimeout()}).DialContext(ctx, "tcp4", c.TargetEndpoint(candidate.IP))
			if conn != nil {
				conn.Close()
			}
		}
		probe.Duration = time.Since(start)
		if err != nil {
			probe.Error = err.Error()
		} else {
			reachable = append(reachable, candidate)
		}
		report.Probes = append(report.Probes, probe)
	}
	if len(reachable) == 0 {
		return report, fmt.Errorf("no reachable WSL target on port %d; inspect diagnostics, service bind address and firewall", c.TargetPort)
	}
	for _, candidate := range reachable {
		if candidate.Preferred {
			report.Selected = candidate.IP
			return report, nil
		}
	}
	if len(reachable) == 1 {
		report.Selected = reachable[0].IP
		return report, nil
	}
	var ips []string
	for _, candidate := range reachable {
		ips = append(ips, candidate.IP.String())
	}
	return report, fmt.Errorf("multiple reachable WSL addresses (%s); set manual target IP", strings.Join(ips, ", "))
}

// Refresh chooses by interface/route evidence only. It must not probe TCP, because
// a refresh runs inside the request's existing TCP-connect retry budget.
func (s *Service) refresh(ctx context.Context, c config.Config) (netip.Addr, error) {
	candidates, err := s.candidates(ctx, c)
	if err != nil {
		return netip.Addr{}, err
	}
	for _, candidate := range candidates {
		if candidate.Preferred {
			return candidate.IP, nil
		}
	}
	if len(candidates) == 1 {
		return candidates[0].IP, nil
	}
	return netip.Addr{}, fmt.Errorf("WSL address is ambiguous after network failure; set manual target IP")
}
