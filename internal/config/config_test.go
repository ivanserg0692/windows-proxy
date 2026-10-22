package config

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func validConfig() Config { cfg := Default(); cfg.Distribution = "Ubuntu"; return cfg }

func TestValidation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*Config)
	}{
		{"zero port", func(cfg *Config) { cfg.ListenPort = 0 }},
		{"large port", func(cfg *Config) { cfg.TargetPort = 65536 }},
		{"invalid listen", func(cfg *Config) { cfg.ListenAddress = "example.com" }},
		{"multicast", func(cfg *Config) { cfg.ManualTargetIP = "224.0.0.1" }},
		{"unspecified target", func(cfg *Config) { cfg.ManualTargetIP = "0.0.0.0" }},
		{"empty distro", func(cfg *Config) { cfg.Distribution = "" }},
		{"option distro", func(cfg *Config) { cfg.Distribution = "--exec" }},
		{"control distro", func(cfg *Config) { cfg.Distribution = "Ubuntu\x1b[31m" }},
		{"negative retries", func(cfg *Config) { cfg.ConnectRetries = -1 }},
		{"excessive timeout", func(cfg *Config) { cfg.ConnectTimeoutMS = 60001 }},
		{"negative delay", func(cfg *Config) { cfg.RetryDelayMS = -1 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validConfig()
			tc.change(&cfg)
			if cfg.Validate() == nil {
				t.Fatal("expected validation error")
			}
		})
	}
	if err := validConfig().Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestStoreRoundTripAndReplacement(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "WSLProxy", "config.json")}
	cfg := validConfig()
	cfg.Distribution = "Ubuntu 日本語"
	cfg.ManualTargetIP = "172.28.1.4"
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	cfg.TargetPort = 9000
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got != cfg {
		t.Fatalf("got %+v, want %+v", got, cfg)
	}
	entries, err := os.ReadDir(filepath.Dir(store.Path))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("temporary files left behind: %v", entries)
	}
	invalidConfig := cfg
	invalidConfig.TargetPort = 0
	if store.Save(invalidConfig) == nil {
		t.Fatal("invalid config was saved")
	}
	got, err = store.Load()
	if err != nil || got != cfg {
		t.Fatalf("previous config changed: %+v %v", got, err)
	}
}

func TestRejectInvalidJSON(t *testing.T) {
	for _, data := range []string{
		`null`, `[]`, `{}`, `{"unknown":1}`, `{"listen_port":8080,"listen_port":8081}`, `{} {}`, `{`,
		strings.Repeat(" ", maxConfigSize+1),
	} {
		store := Store{Path: filepath.Join(t.TempDir(), "config.json")}
		if err := os.WriteFile(store.Path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := store.Load(); err == nil {
			t.Fatalf("accepted invalid data %.100s", data)
		}
	}
}

func TestConcurrentSaveProducesWholeConfig(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "config.json")}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			cfg := validConfig()
			cfg.TargetPort += i
			if err := store.Save(cfg); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	if _, err := store.Load(); err != nil {
		t.Fatal(err)
	}
}
