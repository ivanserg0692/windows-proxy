package wsl

import (
	"context"
	"encoding/json"
	"fmt"
	"net/netip"
	"sort"
	"strings"
)

type Candidate struct {
	IP        netip.Addr
	Interface string
	Preferred bool
}

type addressInterface struct {
	Name      string `json:"ifname"`
	Addresses []struct {
		Family string `json:"family"`
		Local  string `json:"local"`
		Scope  string `json:"scope"`
	} `json:"addr_info"`
}

func (c Client) Candidates(ctx context.Context, distribution string) ([]Candidate, error) {
	if err := c.CheckDistribution(ctx, distribution); err != nil {
		return nil, err
	}
	data, err := c.Runner.Run(ctx, "-d", distribution, "--", "ip", "-j", "-4", "address", "show")
	if err != nil {
		return nil, fmt.Errorf("list WSL addresses (iproute2 is required): %w", err)
	}
	text, err := decodeOutput(data)
	if err != nil {
		return nil, err
	}
	addresses, err := parseAddresses([]byte(text))
	if err != nil {
		return nil, err
	}
	// A failed route lookup is not fatal: disconnected WSL can still be reachable from Windows.
	routeData, routeErr := c.Runner.Run(ctx, "-d", distribution, "--", "ip", "-4", "route", "get", "1.1.1.1")
	if routeErr == nil {
		route, err := decodeOutput(routeData)
		if err == nil {
			ip, device := parseRoute(route)
			for i := range addresses {
				addresses[i].Preferred = addresses[i].IP == ip && addresses[i].Interface == device
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	sort.SliceStable(addresses, func(i, j int) bool { return addresses[i].Preferred && !addresses[j].Preferred })
	if len(addresses) == 0 {
		return nil, fmt.Errorf("no usable WSL IPv4 address; use manual target IP")
	}
	return addresses, nil
}

func parseAddresses(data []byte) ([]Candidate, error) {
	var interfaces []addressInterface
	if err := json.Unmarshal(data, &interfaces); err != nil {
		return nil, fmt.Errorf("parse ip address JSON: %w", err)
	}
	result := []Candidate{}
	seen := map[netip.Addr]bool{}
	for _, iface := range interfaces {
		if excludedInterface(iface.Name) {
			continue
		}
		for _, addr := range iface.Addresses {
			ip, err := netip.ParseAddr(addr.Local)
			if err != nil || addr.Family != "inet" || addr.Scope != "global" || !ip.Is4() || !ip.IsGlobalUnicast() || ip.IsLinkLocalUnicast() || seen[ip] {
				continue
			}
			seen[ip] = true
			result = append(result, Candidate{IP: ip, Interface: iface.Name})
		}
	}
	return result, nil
}

func excludedInterface(name string) bool {
	for _, prefix := range []string{"lo", "docker", "br-", "veth", "virbr", "cni", "flannel", "podman"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func parseRoute(text string) (netip.Addr, string) {
	fields := strings.Fields(text)
	var ip netip.Addr
	var device string
	for i := 0; i+1 < len(fields); i++ {
		switch fields[i] {
		case "src":
			ip, _ = netip.ParseAddr(fields[i+1])
		case "dev":
			device = fields[i+1]
		}
	}
	return ip, device
}
