package log

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoggerCreation(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "warpclip-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Test log file path
	logPath := filepath.Join(tmpDir, "test.log")

	// Create logger
	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Write a test message
	logger.Info("Test message")

	// Check if log file was created
	info, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("Log file was not created: %v", err)
	}

	// Check file permissions
	if info.Mode().Perm() != 0600 {
		t.Errorf("Log file has incorrect permissions: %v, expected 0600", info.Mode().Perm())
	}

	// Check debug file creation
	debugPath := logPath + ".debug"
	if _, err := os.Stat(debugPath); err != nil {
		t.Errorf("Debug log file was not created: %v", err)
	}
}

func TestLogLevels(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "warpclip-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Test log file paths
	logPath := filepath.Join(tmpDir, "test.log")
	debugPath := filepath.Join(tmpDir, "test.log.debug")

	// Create logger
	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Log messages at different levels
	logger.Debug("Debug message")
	logger.Info("Info message")
	logger.Warning("Warning message")
	logger.Error("Error message")

	// Ensure files are written by closing logger
	logger.Close()

	// Read main log file
	logContent, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	// Read debug log file
	debugContent, err := os.ReadFile(debugPath)
	if err != nil {
		t.Fatalf("Failed to read debug log file: %v", err)
	}

	// Convert to strings for easier checking
	logStr := string(logContent)
	debugStr := string(debugContent)

	// Main log should have INFO, WARNING, ERROR but not DEBUG
	if !strings.Contains(logStr, "[INFO]") {
		t.Error("Main log file missing INFO level messages")
	}
	if !strings.Contains(logStr, "[WARNING]") {
		t.Error("Main log file missing WARNING level messages")
	}
	if !strings.Contains(logStr, "[ERROR]") {
		t.Error("Main log file missing ERROR level messages")
	}
	if strings.Contains(logStr, "[DEBUG]") {
		t.Error("Main log file should not contain DEBUG level messages")
	}

	// Debug log should have DEBUG messages
	if !strings.Contains(debugStr, "[DEBUG]") {
		t.Error("Debug log file missing DEBUG level messages")
	}
}

func TestLogRotation(t *testing.T) {
	// Skip this test by default since it involves file system operations
	// that might be platform-dependent or slow
	if testing.Short() {
		t.Skip("Skipping log rotation test in short mode")
	}

	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "warpclip-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Test log file path
	logPath := filepath.Join(tmpDir, "rotation.log")

	// Create logger with small max file size for testing
	logger := &FileLogger{
		logFile:     nil,
		debugFile:   nil,
		maxFileSize: 100, // Very small max size to trigger rotation quickly
		mutex:       sync.Mutex{},
	}

	// Manually open log files
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		t.Fatalf("Failed to create log file: %v", err)
	}
	logger.logFile = logFile

	debugPath := logPath + ".debug"
	debugFile, err := os.OpenFile(debugPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		t.Fatalf("Failed to create debug log file: %v", err)
	}
	logger.debugFile = debugFile

	// Log enough data to trigger rotation
	for i := 0; i < 10; i++ {
		logger.Info("This is a test message that should be long enough to trigger log rotation")
	}

	// Ensure files are closed
	logger.Close()

	// Check if rotated log files exist
	files, err := filepath.Glob(logPath + ".*")
	if err != nil {
		t.Fatalf("Failed to list log files: %v", err)
	}

	// Should have at least one rotated log file (not counting the debug file)
	rotatedCount := 0
	for _, file := range files {
		if strings.HasSuffix(file, ".debug") {
			continue
		}
		rotatedCount++
	}

	if rotatedCount == 0 {
		t.Error("No rotated log files found, rotation may not be working")
	}
}

func TestInputSanitization(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{
			input:    "Normal text",
			expected: "Normal text",
		},
		{
			input:    "Text with\nnewline",
			expected: "Text with\nnewline",
		},
		{
			input:    "Text with\ttab",
			expected: "Text with\ttab",
		},
		{
			input:    "Text with \x00null\x01 bytes",
			expected: "Text with ?null? bytes",
		},
		{
			input:    "Text with escape sequences \x1b[31m red \x1b[0m",
			expected: "Text with escape sequences ?[31m red ?[0m",
		},
	}

	for _, tc := range testCases {
		t.Run("Sanitize: "+tc.input, func(t *testing.T) {
			result := sanitizeInput(tc.input)
			if result != tc.expected {
				t.Errorf("sanitizeInput(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}

