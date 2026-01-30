package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigDefaults(t *testing.T) {
	// Use a temp directory for testing
	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if cfg.DefaultHost != DefaultHost {
		t.Errorf("DefaultHost = %v, want %v", cfg.DefaultHost, DefaultHost)
	}

	if cfg.OutputFormat != DefaultOutputFormat {
		t.Errorf("OutputFormat = %v, want %v", cfg.OutputFormat, DefaultOutputFormat)
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	cfg := &Config{
		DefaultHost:  "https://custom.example.com",
		OutputFormat: "json",
	}

	if err := SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}

	// Verify file was created with correct permissions
	configPath := filepath.Join(tmpDir, ".scraps", "config.json")
	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("Config file not created: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("Config file permissions = %v, want %v", info.Mode().Perm(), 0600)
	}

	// Load and verify
	loaded, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if loaded.DefaultHost != cfg.DefaultHost {
		t.Errorf("DefaultHost = %v, want %v", loaded.DefaultHost, cfg.DefaultHost)
	}
	if loaded.OutputFormat != cfg.OutputFormat {
		t.Errorf("OutputFormat = %v, want %v", loaded.OutputFormat, cfg.OutputFormat)
	}
}

func TestSetHost(t *testing.T) {
	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	newHost := "https://new.example.com"
	if err := SetHost(newHost); err != nil {
		t.Fatalf("SetHost() error = %v", err)
	}

	if got := GetHost(); got != newHost {
		t.Errorf("GetHost() = %v, want %v", got, newHost)
	}
}

func TestSetOutputFormat(t *testing.T) {
	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	if err := SetOutputFormat("json"); err != nil {
		t.Fatalf("SetOutputFormat() error = %v", err)
	}

	if got := GetOutputFormat(); got != "json" {
		t.Errorf("GetOutputFormat() = %v, want %v", got, "json")
	}
}
