package log

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// LogLevel defines the severity of log messages
type LogLevel int

const (
	// DEBUG level for detailed diagnostic information
	DEBUG LogLevel = iota
	// INFO level for general operational information
	INFO
	// WARNING level for potentially harmful situations
	WARNING
	// ERROR level for error events that might still allow the application to continue
	ERROR
)

func (l LogLevel) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARNING:
		return "WARNING"
	case ERROR:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// Logger defines the interface for logging operations
type Logger interface {
	// Debug logs a message at DEBUG level
	Debug(message string)
	// Info logs a message at INFO level
	Info(message string)
	// Warning logs a message at WARNING level
	Warning(message string)
	// Error logs a message at ERROR level
	Error(message string)
	// Close flushes and closes all log files
	Close() error
}

// FileLogger implements the Logger interface with file-based logging
type FileLogger struct {
	logFile    *os.File
	debugFile  *os.File
	maxFileSize int64
	mutex      sync.Mutex
}

// New creates a new FileLogger that writes to the specified file
func New(logFilePath string) (*FileLogger, error) {
	// Get the directory from the log file path
	dir := filepath.Dir(logFilePath)
	
	// Ensure the directory exists
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}
	
	// Open the log file with secure permissions
	logFile, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}
	
	// Create a default debug file path based on the log file path
	debugFilePath := logFilePath
	if ext := filepath.Ext(logFilePath); ext != "" {
		debugFilePath = logFilePath[:len(logFilePath)-len(ext)] + ".debug" + ext
	} else {
		debugFilePath = logFilePath + ".debug"
	}
	
	// Open the debug file with secure permissions
	debugFile, err := os.OpenFile(debugFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		// Close the already opened log file
		logFile.Close()
		return nil, fmt.Errorf("failed to open debug log file: %w", err)
	}
	
	logger := &FileLogger{
		logFile:    logFile,
		debugFile:  debugFile,
		maxFileSize: 10 * 1024 * 1024, // 10MB default max file size
		mutex:      sync.Mutex{},
	}
	
	return logger, nil
}

// Debug logs a message at DEBUG level
func (l *FileLogger) Debug(message string) {
	l.log(DEBUG, sanitizeInput(message))
}

// Info logs a message at INFO level
func (l *FileLogger) Info(message string) {
	l.log(INFO, sanitizeInput(message))
}

// Warning logs a message at WARNING level
func (l *FileLogger) Warning(message string) {
	l.log(WARNING, sanitizeInput(message))
}

// Error logs a message at ERROR level
func (l *FileLogger) Error(message string) {
	l.log(ERROR, sanitizeInput(message))
}

// Close flushes and closes all log files
func (l *FileLogger) Close() error {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	
	var errs []error
	
	if l.logFile != nil {
		if err := l.logFile.Sync(); err != nil {
			errs = append(errs, fmt.Errorf("failed to sync log file: %w", err))
		}
		if err := l.logFile.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close log file: %w", err))
		}
		l.logFile = nil
	}
	
	if l.debugFile != nil {
		if err := l.debugFile.Sync(); err != nil {
			errs = append(errs, fmt.Errorf("failed to sync debug file: %w", err))
		}
		if err := l.debugFile.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close debug file: %w", err))
		}
		l.debugFile = nil
	}
	
	if len(errs) > 0 {
		return fmt.Errorf("errors closing logger: %v", errs)
	}
	
	return nil
}

// log writes a log message with timestamp and level
func (l *FileLogger) log(level LogLevel, message string) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	logLine := fmt.Sprintf("[%s] [%s] %s\n", timestamp, level.String(), message)
	
	// Check if files exist, recreate if needed
	l.ensureLogFilesExist()
	
	// Check if log rotation is needed
	l.checkRotation()
	
	// Write to appropriate file(s)
	if level == DEBUG {
		// Debug messages go only to debug file
		if l.debugFile != nil {
			_, err := l.debugFile.WriteString(logLine)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error writing to debug log: %v\n", err)
			}
		}
	} else {
		// All other messages go to main log file
		if l.logFile != nil {
			_, err := l.logFile.WriteString(logLine)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error writing to log: %v\n", err)
			}
		}
		
		// Errors also go to stderr
		if level == ERROR {
			fmt.Fprint(os.Stderr, logLine)
		}
	}
}

// ensureLogFilesExist checks if log files exist and recreates them if needed
func (l *FileLogger) ensureLogFilesExist() {
	if l.logFile == nil {
		// Try to recreate the log file
		logFile, err := os.OpenFile(l.logFile.Name(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err == nil {
			l.logFile = logFile
		}
	}
	
	if l.debugFile == nil {
		// Try to recreate the debug file
		debugFile, err := os.OpenFile(l.debugFile.Name(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err == nil {
			l.debugFile = debugFile
		}
	}
}

// checkRotation checks if log files need rotation and rotates them if necessary
func (l *FileLogger) checkRotation() {
	// Check main log file size
	if l.logFile != nil {
		info, err := l.logFile.Stat()
		if err == nil && info.Size() > l.maxFileSize {
			// Close current file
			l.logFile.Close()
			
			// Create new name with timestamp
			timestamp := time.Now().Format("20060102150405")
			newName := fmt.Sprintf("%s.%s", l.logFile.Name(), timestamp)
			
			// Rename old file
			os.Rename(l.logFile.Name(), newName)
			
			// Create new file
			newFile, err := os.OpenFile(l.logFile.Name(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
			if err == nil {
				l.logFile = newFile
			}
		}
	}
	
	// Check debug log file size
	if l.debugFile != nil {
		info, err := l.debugFile.Stat()
		if err == nil && info.Size() > l.maxFileSize {
			// Close current file
			l.debugFile.Close()
			
			// Create new name with timestamp
			timestamp := time.Now().Format("20060102150405")
			newName := fmt.Sprintf("%s.%s", l.debugFile.Name(), timestamp)
			
			// Rename old file
			os.Rename(l.debugFile.Name(), newName)
			
			// Create new file
			newFile, err := os.OpenFile(l.debugFile.Name(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
			if err == nil {
				l.debugFile = newFile
			}
		}
	}
}

// sanitizeInput removes control characters from the log message to prevent log injection
func sanitizeInput(input string) string {
	// Remove or replace control characters
	clean := ""
	for _, r := range input {
		if r >= 32 && r != 127 || r == '\t' || r == '\n' || r == '\r' {
			clean += string(r)
		} else {
			// Replace control chars with a safe placeholder
			clean += "?"
		}
	}
	return clean
}

