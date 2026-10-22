package config

import (
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"time"
	"unicode"
)

type Config struct {
	ListenAddress    string `json:"listen_address"`
	ListenPort       int    `json:"listen_port"`
	Distribution     string `json:"wsl_distribution"`
	TargetPort       int    `json:"target_port"`
	ManualTargetIP   string `json:"manual_target_ip,omitempty"`
	ConnectTimeoutMS int    `json:"connect_timeout_ms"`
	ConnectRetries   int    `json:"connect_retries"`
	RetryDelayMS     int    `json:"retry_delay_ms"`
}

func Default() Config {
	return Config{ListenAddress: "0.0.0.0", ListenPort: 8080, TargetPort: 8001,
		ConnectTimeoutMS: 2000, ConnectRetries: 1, RetryDelayMS: 100}
}

func (c Config) Validate() error {
	listen, err := netip.ParseAddr(c.ListenAddress)
	if err != nil || !listen.Is4() || listen.IsMulticast() || listen == netip.MustParseAddr("255.255.255.255") {
		return fmt.Errorf("listen address must be a local IPv4 address or 0.0.0.0")
	}
	if c.ListenPort < 1 || c.ListenPort > 65535 || c.TargetPort < 1 || c.TargetPort > 65535 {
		return fmt.Errorf("ports must be between 1 and 65535")
	}
	if err := ValidateDistribution(c.Distribution); err != nil {
		return err
	}
	if c.ManualTargetIP != "" {
		ip, err := netip.ParseAddr(c.ManualTargetIP)
		if err != nil || !ip.Is4() || ip.IsUnspecified() || ip.IsMulticast() || ip == netip.MustParseAddr("255.255.255.255") {
			return fmt.Errorf("manual target must be a unicast IPv4 address")
		}
	}
	if c.ConnectTimeoutMS < 100 || c.ConnectTimeoutMS > 60000 {
		return fmt.Errorf("connect timeout must be between 100 and 60000 ms")
	}
	if c.ConnectRetries < 0 || c.ConnectRetries > 10 {
		return fmt.Errorf("connect retries must be between 0 and 10")
	}
	if c.RetryDelayMS < 0 || c.RetryDelayMS > 10000 {
		return fmt.Errorf("retry delay must be between 0 and 10000 ms")
	}
	return nil
}

func ValidateDistribution(name string) error {
	if name == "" || len(name) > 256 || strings.TrimSpace(name) != name || strings.HasPrefix(name, "-") || strings.ContainsFunc(name, unicode.IsControl) {
		return fmt.Errorf("select a valid installed WSL distribution")
	}
	return nil
}

func (c Config) ListenEndpoint() string {
	return net.JoinHostPort(c.ListenAddress, strconv.Itoa(c.ListenPort))
}
func (c Config) TargetEndpoint(ip netip.Addr) string {
	return net.JoinHostPort(ip.String(), strconv.Itoa(c.TargetPort))
}
func (c Config) ConnectTimeout() time.Duration {
	return time.Duration(c.ConnectTimeoutMS) * time.Millisecond
}
func (c Config) RetryDelay() time.Duration { return time.Duration(c.RetryDelayMS) * time.Millisecond }

func (c Config) CheckTarget(ip netip.Addr) error {
	if !ip.Is4() || ip.IsUnspecified() || ip.IsMulticast() || ip == netip.MustParseAddr("255.255.255.255") {
		return fmt.Errorf("upstream must be a unicast IPv4 address")
	}
	listen := netip.MustParseAddr(c.ListenAddress)
	if c.ListenPort != c.TargetPort {
		return nil
	}
	if listen == ip {
		return fmt.Errorf("upstream points to the proxy listener")
	}
	if listen.IsUnspecified() {
		if ip.IsLoopback() {
			return fmt.Errorf("loopback upstream would point to the proxy listener")
		}
		addrs, err := net.InterfaceAddrs()
		if err != nil {
			return fmt.Errorf("check local interfaces: %w", err)
		}
		for _, addr := range addrs {
			prefix, err := netip.ParsePrefix(addr.String())
			if err == nil && prefix.Addr().Unmap() == ip {
				return fmt.Errorf("upstream is a local address on the proxy port")
			}
		}
	}
	return nil
}
