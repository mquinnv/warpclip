package server

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"github.com/mquinnv/warpclip/v2/internal/config"
	"github.com/mquinnv/warpclip/v2/internal/log"
)

// Server represents the warpclipd TCP server
type Server struct {
	cfg            *config.Config
	logger         log.Logger
	listener       net.Listener
	activeConns    sync.WaitGroup
	shutdownSignal chan struct{}
	
	// Track connections by remote address to handle multiple connections
	connMutex      sync.Mutex
	activeAddrs    map[string]time.Time
}

// New creates a new Server instance
func New(cfg *config.Config, logger log.Logger) *Server {
	return &Server{
		cfg:            cfg,
		logger:         logger,
		shutdownSignal: make(chan struct{}),
		activeAddrs:    make(map[string]time.Time),
	}
}

// Start starts the TCP server
func (s *Server) Start(ctx context.Context) error {
	// Create a TCP listener
	address := fmt.Sprintf("%s:%d", s.cfg.BindAddress, s.cfg.Port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to create listener: %w", err)
	}
	s.listener = listener
	defer s.listener.Close()

	s.logger.Info(fmt.Sprintf("Server listening on %s", address))

	// Write PID file
	if err := s.writePidFile(); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}
	defer os.Remove(s.cfg.PidFile)

	// Channel for accept errors
	errorCh := make(chan error, 1)

	// Channel for new connections
	connCh := make(chan net.Conn, 10)

	// Start accepting connections in a separate goroutine
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				// Check if we're shutting down
				select {
				case <-s.shutdownSignal:
					return
				case <-ctx.Done():
					return
				default:
					errorCh <- fmt.Errorf("accept error: %w", err)
					return
				}
			}

			select {
			case connCh <- conn:
				// Connection sent for processing
			case <-ctx.Done():
				conn.Close()
				return
			case <-s.shutdownSignal:
				conn.Close()
				return
			}
		}
	}()

	// Process connections and handle shutdown
	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Context cancelled, shutting down server...")
			close(s.shutdownSignal)
			s.listener.Close()
			s.activeConns.Wait() // Wait for active connections to finish
			s.logger.Info("Server shutdown complete")
			return nil

		case err := <-errorCh:
			s.logger.Error(fmt.Sprintf("Error accepting connection: %v", err))
			return err

		case conn := <-connCh:
			s.activeConns.Add(1)
			go func(c net.Conn) {
				defer s.activeConns.Done()
				s.handleConnection(c)
			}(conn)
		}
	}
}

// handleConnection processes a single client connection
func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	remoteAddr := conn.RemoteAddr().String()
	s.logger.Info(fmt.Sprintf("New connection from %s", remoteAddr))

	// Set read deadline to prevent hanging
	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		s.logger.Error(fmt.Sprintf("Failed to set read deadline: %v", err))
		return
	}

	// Create a buffer with some capacity to avoid reallocations
	buf := make([]byte, 1024)
	var data []byte

	// Read the first chunk
	n, err := conn.Read(buf)
	if err != nil {
		if err != io.EOF {
			s.logger.Error(fmt.Sprintf("Error reading initial data: %v", err))
		} else {
			// EOF on first read means this is likely the control connection
			s.logger.Info(fmt.Sprintf("Control connection from %s (no data), closing", remoteAddr))
		}
		return
	}

	// If we got data in the first read, this is our data connection
	if n > 0 {
		data = append(data, buf[:n]...)

		// Continue reading until EOF or error (up to size limit)
		totalRead := int64(n)
		for totalRead < s.cfg.MaxDataSize {
			n, err = conn.Read(buf)
			if err == io.EOF {
				break
			}
			if err != nil {
				s.logger.Error(fmt.Sprintf("Error reading data: %v", err))
				return
			}
			if n > 0 {
				data = append(data, buf[:n]...)
				totalRead += int64(n)
			}
		}

		// Process the data
		if len(data) > 0 {
			// Check if we hit the size limit
			if totalRead >= s.cfg.MaxDataSize {
				s.logger.Warning(fmt.Sprintf("Data exceeded maximum size limit (%d bytes), truncated", s.cfg.MaxDataSize))
			}

			// Copy data to clipboard
			if err := s.copyToClipboard(data); err != nil {
				s.logger.Error(fmt.Sprintf("Failed to copy to clipboard: %v", err))
				return
			}

			// Update last activity file
			if err := s.updateLastActivityFile(len(data)); err != nil {
				s.logger.Warning(fmt.Sprintf("Failed to update last activity file: %v", err))
			}

			s.logger.Info(fmt.Sprintf("Successfully copied %d bytes to clipboard", len(data)))
		} else {
			s.logger.Warning("Received empty data, nothing to copy")
		}
	} else {
		// Empty first read (0 bytes) - likely secondary connection
		s.logger.Info(fmt.Sprintf("Secondary connection from %s (empty read), closing", remoteAddr))
	}
}

// cleanupOldConnections removes stale connection records periodically
func (s *Server) cleanupOldConnections() {
	s.connMutex.Lock()
	defer s.connMutex.Unlock()
	
	now := time.Now()
	for addr, timestamp := range s.activeAddrs {
		if now.Sub(timestamp) > 30*time.Second {
			delete(s.activeAddrs, addr)
		}
	}
}

// copyToClipboard copies data to the system clipboard using pbcopy
func (s *Server) copyToClipboard(data []byte) error {
	// Add retry logic for reliability
	maxRetries := 3
	var lastErr error
	
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			s.logger.Warning(fmt.Sprintf("Retrying clipboard operation (attempt %d/%d)", attempt+1, maxRetries))
			time.Sleep(time.Duration(100*attempt) * time.Millisecond) // Backoff
		}
		
		if err := s.copyToClipboardOnce(data); err != nil {
			lastErr = err
			s.logger.Warning(fmt.Sprintf("Clipboard operation failed: %v", err))
			continue
		}
		
		return nil // Success
	}
	
	return fmt.Errorf("failed after %d attempts: %w", maxRetries, lastErr)
}

// copyToClipboardOnce performs a single clipboard operation
func (s *Server) copyToClipboardOnce(data []byte) error {
	// Create pbcopy command
	cmd := exec.Command("pbcopy")
	
	// Get stdin pipe
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdin pipe: %w", err)
	}
	
	// Start the command
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start pbcopy: %w", err)
	}
	
	// Create a buffered writer for better performance
	writer := bufio.NewWriter(stdin)
	
	// Write data to stdin
	_, err = writer.Write(data)
	if err != nil {
		stdin.Close()
		return fmt.Errorf("failed to write data to pbcopy: %w", err)
	}
	
	// Flush the buffer
	if err := writer.Flush(); err != nil {
		stdin.Close()
		return fmt.Errorf("failed to flush data to pbcopy: %w", err)
	}
	
	// Close stdin
	if err := stdin.Close(); err != nil {
		return fmt.Errorf("failed to close stdin: %w", err)
	}
	
	// Wait for the command to finish with timeout
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()
	
	// Wait for completion or timeout
	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("pbcopy command failed: %w", err)
		}
	case <-time.After(5 * time.Second):
		// Kill the process if it takes too long
		cmd.Process.Kill()
		return fmt.Errorf("pbcopy operation timed out after 5 seconds")
	}
	
	return nil
}

// updateLastActivityFile updates the last activity file with timestamp and data size
func (s *Server) updateLastActivityFile(dataSize int) error {
	file, err := os.OpenFile(s.cfg.LastFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to open last activity file: %w", err)
	}
	defer file.Close()
	
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	content := fmt.Sprintf("%d bytes copied\n%s\n", dataSize, timestamp)
	
	_, err = file.WriteString(content)
	if err != nil {
		return fmt.Errorf("failed to write to last activity file: %w", err)
	}
	
	return nil
}

// writePidFile writes the current process ID to the PID file
func (s *Server) writePidFile() error {
	// Get current process ID
	pid := os.Getpid()
	
	// Create a temporary file with a unique name
	tempFile := fmt.Sprintf("%s.%d", s.cfg.PidFile, pid)
	
	// Write PID to the temporary file with secure permissions
	err := os.WriteFile(tempFile, []byte(strconv.Itoa(pid)), 0600)
	if err != nil {
		return fmt.Errorf("failed to write temporary PID file: %w", err)
	}
	
	// Atomically rename the temporary file to the actual PID file
	err = os.Rename(tempFile, s.cfg.PidFile)
	if err != nil {
		// Clean up the temporary file if rename fails
		os.Remove(tempFile)
		return fmt.Errorf("failed to rename PID file: %w", err)
	}
	
	s.logger.Info(fmt.Sprintf("PID file created at %s (PID: %d)", s.cfg.PidFile, pid))
	return nil
}

