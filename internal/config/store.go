package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const maxConfigSize = 64 * 1024

type Store struct{ Path string }

func UserStore() (Store, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return Store{}, fmt.Errorf("locate user configuration: %w", err)
	}
	return Store{Path: filepath.Join(dir, "WSLProxy", "config.json")}, nil
}

func (s Store) Load() (Config, error) {
	configFile, err := os.Open(s.Path)
	if err != nil {
		return Config{}, err
	}
	defer configFile.Close()
	data, err := io.ReadAll(io.LimitReader(configFile, maxConfigSize+1))
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	if len(data) > maxConfigSize {
		return Config{}, fmt.Errorf("config exceeds 64 KiB")
	}
	// Decode into a zero value: missing mandatory values must not silently become defaults.
	var cfg Config
	if err := decode(data, &cfg); err != nil {
		return Config{}, err
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, fmt.Errorf("invalid config: %w", err)
	}
	return cfg, nil
}

func (s Store) Save(cfg Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(s.Path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	tempFile, err := os.CreateTemp(dir, ".config-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary config: %w", err)
	}
	defer os.Remove(tempFile.Name())
	if _, err = tempFile.Write(append(data, '\n')); err != nil {
		tempFile.Close()
		return fmt.Errorf("write config: %w", err)
	}
	if err = tempFile.Sync(); err != nil {
		tempFile.Close()
		return fmt.Errorf("flush config: %w", err)
	}
	if err = tempFile.Close(); err != nil {
		return fmt.Errorf("close config: %w", err)
	}
	if err = replaceFile(tempFile.Name(), s.Path); err != nil {
		return fmt.Errorf("replace config: %w", err)
	}
	return nil
}

func decode(data []byte, cfg *Config) error {
	// Reject duplicates as well as unknown keys so the displayed configuration is unambiguous.
	var fields map[string]json.RawMessage
	seen := make(map[string]bool)
	decoder := json.NewDecoder(bytes.NewReader(data))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return fmt.Errorf("config must be a JSON object")
	}
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return fmt.Errorf("decode config: %w", err)
		}
		key, ok := token.(string)
		if !ok || seen[key] {
			return fmt.Errorf("duplicate or invalid config key: %v", token)
		}
		seen[key] = true
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return fmt.Errorf("decode config: %w", err)
		}
	}
	if _, err := decoder.Token(); err != nil {
		return fmt.Errorf("decode config: %w", err)
	}
	if err := decoder.Decode(&fields); !errors.Is(err, io.EOF) {
		return fmt.Errorf("config must contain exactly one JSON object")
	}
	strictDecoder := json.NewDecoder(bytes.NewReader(data))
	strictDecoder.DisallowUnknownFields()
	if err := strictDecoder.Decode(cfg); err != nil {
		return fmt.Errorf("decode config: %w", err)
	}
	return nil
}
