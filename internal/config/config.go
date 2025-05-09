package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config holds the configuration for the warpclipd service
type Config struct {
	// Port to listen on
	Port int
	// Bind address for the server (always localhost)
	BindAddress string
	// Log file path
	LogFile string
	// Debug log file path
	DebugFile string
	// Output log file path
	OutLogFile string
	// Error log file path
	ErrorLogFile string
	// PID file path
	PidFile string
	// Last activity file path
	LastFile string
	// Maximum data size (in bytes)
	MaxDataSize int64
}

// Load loads the configuration from environment variables
func Load() (*Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}

	// Default configuration
	cfg := &Config{
		Port:         8888,
		BindAddress:  "127.0.0.1",
		LogFile:      filepath.Join(homeDir, ".warpclip.log"),
		DebugFile:    filepath.Join(homeDir, ".warpclip.debug.log"),
		OutLogFile:   filepath.Join(homeDir, ".warpclip.out.log"),
		ErrorLogFile: filepath.Join(homeDir, ".warpclip.error.log"),
		PidFile:      filepath.Join(homeDir, ".warpclip.pid"),
		LastFile:     filepath.Join(homeDir, ".warpclip.last"),
		MaxDataSize:  1048576, // 1MB
	}

	// Override with environment variables if present
	if portStr := os.Getenv("WARPCLIP_LOCAL_PORT"); portStr != "" {
		port, err := strconv.Atoi(portStr)
		if err != nil {
			return nil, fmt.Errorf("invalid WARPCLIP_LOCAL_PORT value: %w", err)
		}
		if port < 1024 || port > 65535 {
			return nil, fmt.Errorf("WARPCLIP_LOCAL_PORT must be between 1024 and 65535")
		}
		cfg.Port = port
	}

	if logFile := os.Getenv("WARPCLIP_LOG_FILE"); logFile != "" {
		cfg.LogFile = expandPath(logFile, homeDir)
	}

	if debugFile := os.Getenv("WARPCLIP_DEBUG_FILE"); debugFile != "" {
		cfg.DebugFile = expandPath(debugFile, homeDir)
	}

	if outLogFile := os.Getenv("WARPCLIP_OUT_LOG"); outLogFile != "" {
		cfg.OutLogFile = expandPath(outLogFile, homeDir)
	}

	if errorLogFile := os.Getenv("WARPCLIP_ERROR_LOG"); errorLogFile != "" {
		cfg.ErrorLogFile = expandPath(errorLogFile, homeDir)
	}

	if maxDataSizeStr := os.Getenv("WARPCLIP_MAX_DATA_SIZE"); maxDataSizeStr != "" {
		maxDataSize, err := strconv.ParseInt(maxDataSizeStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid WARPCLIP_MAX_DATA_SIZE value: %w", err)
		}
		// Set reasonable limits - minimum 1KB, maximum 100MB
		if maxDataSize < 1024 || maxDataSize > 104857600 {
			return nil, fmt.Errorf("WARPCLIP_MAX_DATA_SIZE must be between 1024 and 104857600 bytes")
		}
		cfg.MaxDataSize = maxDataSize
	}

	// Validate configuration
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// expandPath expands the path with home directory if needed
func expandPath(path string, homeDir string) string {
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(homeDir, path[2:])
	}
	return path
}

// validateConfig performs validation on the configuration
func validateConfig(cfg *Config) error {
	// Validate port is in valid range
	if cfg.Port < 1024 || cfg.Port > 65535 {
		return fmt.Errorf("port must be between 1024 and 65535")
	}

	// Validate bind address is localhost
	if cfg.BindAddress != "127.0.0.1" && cfg.BindAddress != "localhost" {
		return fmt.Errorf("bind address must be localhost for security")
	}

	// Validate max data size
	if cfg.MaxDataSize < 1024 {
		return fmt.Errorf("maximum data size must be at least 1024 bytes")
	}

	// Ensure parent directories for log files exist
	filePaths := []string{
		cfg.LogFile,
		cfg.DebugFile,
		cfg.OutLogFile,
		cfg.ErrorLogFile,
		cfg.PidFile,
		cfg.LastFile,
	}

	for _, path := range filePaths {
		dir := filepath.Dir(path)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			if err := os.MkdirAll(dir, 0700); err != nil {
				return fmt.Errorf("failed to create directory for %s: %w", path, err)
			}
		}
	}

	return nil
}

