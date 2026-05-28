package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	ClientID string `json:"client_id"`
}

func path() string {
	cfg, _ := os.UserConfigDir()
	return filepath.Join(cfg, "spotui", "config.json")
}

func Load() (*Config, error) {
	f, err := os.Open(path())
	if err != nil {
		return &Config{}, nil // no config file yet — not an error
	}
	defer f.Close()
	var c Config
	if err := json.NewDecoder(f).Decode(&c); err != nil {
		return nil, err
	}
	return &c, nil
}

func Save(c *Config) error {
	p := path()
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(c)
}
