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
	f, err := os.Open(s.Path)
	if err != nil {
		return Config{}, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxConfigSize+1))
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	if len(data) > maxConfigSize {
		return Config{}, fmt.Errorf("config exceeds 64 KiB")
	}
	// Decode into a zero value: missing mandatory values must not silently become defaults.
	var c Config
	if err := decode(data, &c); err != nil {
		return Config{}, err
	}
	if err := c.Validate(); err != nil {
		return Config{}, fmt.Errorf("invalid config: %w", err)
	}
	return c, nil
}

func (s Store) Save(c Config) error {
	if err := c.Validate(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(s.Path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	f, err := os.CreateTemp(dir, ".config-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary config: %w", err)
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(append(data, '\n')); err != nil {
		f.Close()
		return fmt.Errorf("write config: %w", err)
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return fmt.Errorf("flush config: %w", err)
	}
	if err = f.Close(); err != nil {
		return fmt.Errorf("close config: %w", err)
	}
	if err = replaceFile(f.Name(), s.Path); err != nil {
		return fmt.Errorf("replace config: %w", err)
	}
	return nil
}

func decode(data []byte, c *Config) error {
	// Reject duplicates as well as unknown keys so the displayed configuration is unambiguous.
	var fields map[string]json.RawMessage
	seen := make(map[string]bool)
	d := json.NewDecoder(bytes.NewReader(data))
	token, err := d.Token()
	if err != nil || token != json.Delim('{') {
		return fmt.Errorf("config must be a JSON object")
	}
	for d.More() {
		token, err := d.Token()
		if err != nil {
			return fmt.Errorf("decode config: %w", err)
		}
		key, ok := token.(string)
		if !ok || seen[key] {
			return fmt.Errorf("duplicate or invalid config key: %v", token)
		}
		seen[key] = true
		var value json.RawMessage
		if err := d.Decode(&value); err != nil {
			return fmt.Errorf("decode config: %w", err)
		}
	}
	if _, err := d.Token(); err != nil {
		return fmt.Errorf("decode config: %w", err)
	}
	if err := d.Decode(&fields); !errors.Is(err, io.EOF) {
		return fmt.Errorf("config must contain exactly one JSON object")
	}
	strict := json.NewDecoder(bytes.NewReader(data))
	strict.DisallowUnknownFields()
	if err := strict.Decode(c); err != nil {
		return fmt.Errorf("decode config: %w", err)
	}
	return nil
}
