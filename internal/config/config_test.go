package config

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func validConfig() Config { c := Default(); c.Distribution = "Ubuntu"; return c }

func TestValidation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*Config)
	}{
		{"zero port", func(c *Config) { c.ListenPort = 0 }},
		{"large port", func(c *Config) { c.TargetPort = 65536 }},
		{"invalid listen", func(c *Config) { c.ListenAddress = "example.com" }},
		{"multicast", func(c *Config) { c.ManualTargetIP = "224.0.0.1" }},
		{"unspecified target", func(c *Config) { c.ManualTargetIP = "0.0.0.0" }},
		{"empty distro", func(c *Config) { c.Distribution = "" }},
		{"option distro", func(c *Config) { c.Distribution = "--exec" }},
		{"control distro", func(c *Config) { c.Distribution = "Ubuntu\x1b[31m" }},
		{"negative retries", func(c *Config) { c.ConnectRetries = -1 }},
		{"excessive timeout", func(c *Config) { c.ConnectTimeoutMS = 60001 }},
		{"negative delay", func(c *Config) { c.RetryDelayMS = -1 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := validConfig()
			tc.change(&c)
			if c.Validate() == nil {
				t.Fatal("expected validation error")
			}
		})
	}
	if err := validConfig().Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestStoreRoundTripAndReplacement(t *testing.T) {
	s := Store{Path: filepath.Join(t.TempDir(), "WSLProxy", "config.json")}
	c := validConfig()
	c.Distribution = "Ubuntu 日本語"
	c.ManualTargetIP = "172.28.1.4"
	if err := s.Save(c); err != nil {
		t.Fatal(err)
	}
	c.TargetPort = 9000
	if err := s.Save(c); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got != c {
		t.Fatalf("got %+v, want %+v", got, c)
	}
	entries, err := os.ReadDir(filepath.Dir(s.Path))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("temporary files left behind: %v", entries)
	}
	bad := c
	bad.TargetPort = 0
	if s.Save(bad) == nil {
		t.Fatal("invalid config was saved")
	}
	got, err = s.Load()
	if err != nil || got != c {
		t.Fatalf("previous config changed: %+v %v", got, err)
	}
}

func TestRejectInvalidJSON(t *testing.T) {
	for _, data := range []string{
		`null`, `[]`, `{}`, `{"unknown":1}`, `{"listen_port":8080,"listen_port":8081}`, `{} {}`, `{`,
		strings.Repeat(" ", maxConfigSize+1),
	} {
		s := Store{Path: filepath.Join(t.TempDir(), "config.json")}
		if err := os.WriteFile(s.Path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Load(); err == nil {
			t.Fatalf("accepted invalid data %.100s", data)
		}
	}
}

func TestConcurrentSaveProducesWholeConfig(t *testing.T) {
	s := Store{Path: filepath.Join(t.TempDir(), "config.json")}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			c := validConfig()
			c.TargetPort += i
			if err := s.Save(c); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	if _, err := s.Load(); err != nil {
		t.Fatal(err)
	}
}
