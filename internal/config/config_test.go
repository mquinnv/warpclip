package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	// Load default configuration
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Failed to load default config: %v", err)
	}

	// Test default values
	if cfg.Port != 8888 {
		t.Errorf("Expected default port 8888, got %d", cfg.Port)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}

	expectedLogFile := filepath.Join(homeDir, ".warpclip.log")
	if cfg.LogFile != expectedLogFile {
		t.Errorf("Expected log file %s, got %s", expectedLogFile, cfg.LogFile)
	}

	// Check max data size is 1MB
	if cfg.MaxDataSize != 1048576 {
		t.Errorf("Expected max data size 1048576, got %d", cfg.MaxDataSize)
	}
}

func TestEnvironmentOverrides(t *testing.T) {
	// Save original environment to restore later
	origPort := os.Getenv("WARPCLIP_LOCAL_PORT")
	origLogFile := os.Getenv("WARPCLIP_LOG_FILE")
	defer func() {
		os.Setenv("WARPCLIP_LOCAL_PORT", origPort)
		os.Setenv("WARPCLIP_LOG_FILE", origLogFile)
	}()

	// Set environment variables
	os.Setenv("WARPCLIP_LOCAL_PORT", "9999")
	os.Setenv("WARPCLIP_LOG_FILE", "/tmp/custom.log")

	// Load config
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Failed to load config with environment overrides: %v", err)
	}

	// Test overridden values
	if cfg.Port != 9999 {
		t.Errorf("Expected port 9999, got %d", cfg.Port)
	}

	if cfg.LogFile != "/tmp/custom.log" {
		t.Errorf("Expected log file /tmp/custom.log, got %s", cfg.LogFile)
	}

	// Test invalid port
	os.Setenv("WARPCLIP_LOCAL_PORT", "invalid")
	_, err = Load()
	if err == nil {
		t.Error("Expected error with invalid port, got nil")
	}
}

func TestExpandPath(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}

	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "home directory",
			input:    "~/test.log",
			expected: filepath.Join(homeDir, "test.log"),
		},
		{
			name:     "absolute path",
			input:    "/tmp/test.log",
			expected: "/tmp/test.log",
		},
		{
			name:     "relative path",
			input:    "test.log",
			expected: "test.log",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := expandPath(tc.input, homeDir)
			if result != tc.expected {
				t.Errorf("expandPath(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestValidateConfig(t *testing.T) {
	testCases := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: &Config{
				Port:        8888,
				BindAddress: "127.0.0.1",
				MaxDataSize: 1024,
			},
			wantErr: false,
		},
		{
			name: "port too low",
			cfg: &Config{
				Port:        123,
				BindAddress: "127.0.0.1",
				MaxDataSize: 1024,
			},
			wantErr: true,
		},
		{
			name: "port too high",
			cfg: &Config{
				Port:        70000,
				BindAddress: "127.0.0.1",
				MaxDataSize: 1024,
			},
			wantErr: true,
		},
		{
			name: "data size too small",
			cfg: &Config{
				Port:        8888,
				BindAddress: "127.0.0.1",
				MaxDataSize: 100,
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateConfig(tc.cfg)
			if (err != nil) != tc.wantErr {
				t.Errorf("validateConfig() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

