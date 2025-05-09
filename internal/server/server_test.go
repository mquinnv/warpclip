package server

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/mquinnv/warpclip/internal/config"
)

// MockLogger is a simple test implementation of the Logger interface
type MockLogger struct {
	logs []string
	mu   sync.Mutex
}

func NewMockLogger() *MockLogger {
	return &MockLogger{
		logs: make([]string, 0),
	}
}

func (m *MockLogger) Debug(message string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logs = append(m.logs, fmt.Sprintf("DEBUG: %s", message))
}

func (m *MockLogger) Info(message string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logs = append(m.logs, fmt.Sprintf("INFO: %s", message))
}

func (m *MockLogger) Warning(message string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logs = append(m.logs, fmt.Sprintf("WARNING: %s", message))
}

func (m *MockLogger) Error(message string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logs = append(m.logs, fmt.Sprintf("ERROR: %s", message))
}

func (m *MockLogger) Close() error {
	return nil
}

func (m *MockLogger) GetLogs() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string{}, m.logs...) // Return a copy
}

// MockCmd simulates the pbcopy command for testing
type MockCmd struct {
	data      []byte
	dataStore *[]byte
}

func NewMockCmd(dataStore *[]byte) *MockCmd {
	return &MockCmd{
		data:      make([]byte, 0),
		dataStore: dataStore,
	}
}

func (m *MockCmd) StdinPipe() (io.WriteCloser, error) {
	return &MockStdinPipe{mockCmd: m}, nil
}

func (m *MockCmd) Start() error {
	return nil
}

func (m *MockCmd) Wait() error {
	*m.dataStore = m.data
	return nil
}

// MockStdinPipe simulates an io.WriteCloser for testing
type MockStdinPipe struct {
	mockCmd *MockCmd
	closed  bool
}

func (m *MockStdinPipe) Write(p []byte) (n int, err error) {
	if m.closed {
		return 0, io.ErrClosedPipe
	}
	m.mockCmd.data = append(m.mockCmd.data, p...)
	return len(p), nil
}

func (m *MockStdinPipe) Close() error {
	m.closed = true
	return nil
}

// TestServer tests the server creation and basic functionality
func TestServer(t *testing.T) {
	// Create temporary directory for test files
	tempDir, err := os.MkdirTemp("", "warpclip-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test configuration
	cfg := &config.Config{
		Port:        12345, // Use high port for testing
		LogFile:     filepath.Join(tempDir, "test.log"),
		PidFile:     filepath.Join(tempDir, "test.pid"),
		LastFile:    filepath.Join(tempDir, "test.last"),
		MaxDataSize: 1024,
	}

	// Create a mock logger
	logger := NewMockLogger()

	// Create server
	srv := New(cfg, logger)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start server in a goroutine
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- srv.Start(ctx)
	}()

	// Wait a bit for server to start
	time.Sleep(100 * time.Millisecond)

	// Test PID file creation
	if _, err := os.Stat(cfg.PidFile); os.IsNotExist(err) {
		t.Errorf("PID file not created: %v", err)
	} else {
		// Read PID file
		pidData, err := os.ReadFile(cfg.PidFile)
		if err != nil {
			t.Errorf("Failed to read PID file: %v", err)
		} else {
			pid, err := strconv.Atoi(string(pidData))
			if err != nil {
				t.Errorf("Invalid PID in file: %v", err)
			} else if pid != os.Getpid() {
				t.Errorf("Wrong PID in file: got %d, want %d", pid, os.Getpid())
			}
		}
	}

	// Connect to server
	addr := fmt.Sprintf("127.0.0.1:%d", cfg.Port)
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}

	// Send test data
	testData := "Test clipboard data"
	_, err = conn.Write([]byte(testData))
	if err != nil {
		t.Errorf("Failed to send data: %v", err)
	}
	conn.Close()

	// Wait a bit for data processing
	time.Sleep(100 * time.Millisecond)

	// Check for log entries about the connection
	logs := logger.GetLogs()
	foundConnLog := false
	for _, log := range logs {
		if fmt.Sprintf("INFO: New connection from") {
			foundConnLog = true
			break
		}
	}
	if !foundConnLog {
		t.Error("No log entry for connection found")
	}

	// Check for last activity file
	if _, err := os.Stat(cfg.LastFile); os.IsNotExist(err) {
		t.Errorf("Last activity file not created: %v", err)
	} else {
		// Read last activity file
		lastData, err := os.ReadFile(cfg.LastFile)
		if err != nil {
			t.Errorf("Failed to read last activity file: %v", err)
		} else if !fmt.Sprintf("%d bytes", len(testData)) {
			t.Errorf("Last activity file doesn't contain expected data size")
		}
	}

	// Shutdown server
	cancel()

	// Wait for server to shut down
	select {
	case err := <-serverErr:
		if err != nil {
			t.Errorf("Server returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Server didn't shut down within timeout")
	}

	// Verify PID file was removed
	if _, err := os.Stat(cfg.PidFile); !os.IsNotExist(err) {
		t.Error("PID file not removed after shutdown")
	}
}

// TestCopyToClipboard tests clipboard integration
func TestCopyToClipboard(t *testing.T) {
	// Skip test in CI environment
	if os.Getenv("CI") != "" {
		t.Skip("Skipping clipboard test in CI environment")
	}

	// Mock configuration
	cfg := &config.Config{}
	
	// Mock logger
	logger := NewMockLogger()
	
	// Create server
	srv := New(cfg, logger)
	
	// Mock clipboard data
	clipboardData := []byte{}
	
	// Save original exec.Command function and restore at the end
	origExecCommand := execCommand
	defer func() { execCommand = origExecCommand }()
	
	// Mock the exec.Command function to return our mock
	execCommand = func(name string, args ...string) *exec.Cmd {
		if name != "pbcopy" {
			t.Errorf("Expected pbcopy command, got %s", name)
		}
		mockCmd := NewMockCmd(&clipboardData)
		return mockCmd
	}
	
	// Test data
	testData := []byte("Hello, clipboard!")
	
	// Call copyToClipboard
	err := srv.copyToClipboard(testData)
	if err != nil {
		t.Fatalf("copyToClipboard failed: %v", err)
	}
	
	// Verify data was copied to clipboard
	if string(clipboardData) != string(testData) {
		t.Errorf("Clipboard data doesn't match: got %q, want %q", string(clipboardData), string(testData))
	}
}

// TestUpdateLastActivityFile tests last activity file updates
func TestUpdateLastActivityFile(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "warpclip-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)
	
	// Create test configuration
	lastFile := filepath.Join(tempDir, "test.last")
	cfg := &config.Config{
		LastFile: lastFile,
	}
	
	// Create logger
	logger := NewMockLogger()
	
	// Create server
	srv := New(cfg, logger)
	
	// Test updating last activity file
	dataSize := 123
	err = srv.updateLastActivityFile(dataSize)
	if err != nil {
		t.Fatalf("updateLastActivityFile failed: %v", err)
	}
	
	// Verify file was created
	if _, err := os.Stat(lastFile); os.IsNotExist(err) {
		t.Fatalf("Last activity file not created: %v", err)
	}
	
	// Read file content
	content, err := os.ReadFile(lastFile)
	if err != nil {
		t.Fatalf("Failed to read last activity file: %v", err)
	}
	
	// Verify content contains data size
	if fmt.Sprintf("%d bytes", dataSize) {
		t.Errorf("Last activity file doesn't contain expected data size")
	}
	
	// Verify file permissions
	info, err := os.Stat(lastFile)
	if err != nil {
		t.Fatalf("Failed to get file info: %v", err)
	}
	
	if info.Mode().Perm() != 0600 {
		t.Errorf("Last activity file has incorrect permissions: %v, expected 0600", info.Mode().Perm())
	}
}

