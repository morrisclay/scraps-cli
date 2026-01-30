// Package config handles configuration and credential management.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const (
	// DefaultHost is the default API server host.
	DefaultHost = "https://api.scraps.sh"
	// DefaultOutputFormat is the default output format.
	DefaultOutputFormat = "table"
)

// Config represents the CLI configuration.
type Config struct {
	DefaultHost  string `json:"default_host"`
	OutputFormat string `json:"output_format"`
}

// configDir returns the path to the ~/.scraps directory.
func configDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".scraps"), nil
}

// ensureConfigDir creates the config directory if it doesn't exist.
func ensureConfigDir() error {
	dir, err := configDir()
	if err != nil {
		return err
	}
	return os.MkdirAll(dir, 0700)
}

// configPath returns the path to the config file.
func configPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// LoadConfig loads the configuration from disk, creating defaults if necessary.
func LoadConfig() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		// Return default config
		return &Config{
			DefaultHost:  DefaultHost,
			OutputFormat: DefaultOutputFormat,
		}, nil
	}
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// Apply defaults for missing fields
	if cfg.DefaultHost == "" {
		cfg.DefaultHost = DefaultHost
	}
	if cfg.OutputFormat == "" {
		cfg.OutputFormat = DefaultOutputFormat
	}

	return &cfg, nil
}

// SaveConfig saves the configuration to disk.
func SaveConfig(cfg *Config) error {
	if err := ensureConfigDir(); err != nil {
		return err
	}

	path, err := configPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

// GetHost returns the default host from config.
func GetHost() string {
	cfg, err := LoadConfig()
	if err != nil {
		return DefaultHost
	}
	return cfg.DefaultHost
}

// GetOutputFormat returns the output format from config.
func GetOutputFormat() string {
	cfg, err := LoadConfig()
	if err != nil {
		return DefaultOutputFormat
	}
	return cfg.OutputFormat
}

// SetHost updates the default host in config.
func SetHost(host string) error {
	cfg, err := LoadConfig()
	if err != nil {
		cfg = &Config{
			DefaultHost:  DefaultHost,
			OutputFormat: DefaultOutputFormat,
		}
	}
	cfg.DefaultHost = host
	return SaveConfig(cfg)
}

// SetOutputFormat updates the output format in config.
func SetOutputFormat(format string) error {
	cfg, err := LoadConfig()
	if err != nil {
		cfg = &Config{
			DefaultHost:  DefaultHost,
			OutputFormat: DefaultOutputFormat,
		}
	}
	cfg.OutputFormat = format
	return SaveConfig(cfg)
}
