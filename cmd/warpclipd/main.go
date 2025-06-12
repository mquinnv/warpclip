package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mquinnv/warpclip/v2/internal/config"
	"github.com/mquinnv/warpclip/v2/internal/log"
	"github.com/mquinnv/warpclip/v2/internal/server"
)

const Version = "2.1.11"

func main() {
	// Define the command line flags
	versionFlag := flag.Bool("version", false, "Show version information")
	helpFlag := flag.Bool("help", false, "Show help message")
	
	// Parse command line arguments
	flag.Parse()
	
	// Get the command
	command := "start" // Default command
	if flag.NArg() > 0 {
		command = flag.Arg(0)
	}
	
	// Handle version flag
	if *versionFlag {
		fmt.Printf("warpclipd v%s\n", Version)
		return
	}
	
	// Handle help flag or help command
	if *helpFlag || command == "help" {
		showHelp()
		return
	}
	
	// Initialize configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
		os.Exit(1)
	}
	
	// Process commands
	switch command {
	case "start":
		startServer(cfg)
	case "stop":
		stopServer(cfg)
	case "restart":
		stopServer(cfg)
		startServer(cfg)
	case "status":
		showStatus(cfg)
	case "version":
		fmt.Printf("warpclipd v%s\n", Version)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		showHelp()
		os.Exit(1)
	}
}

func startServer(cfg *config.Config) {
	// Initialize logger
	logger, err := log.New(cfg.LogFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Close()

	logger.Info("Starting warpclipd")

	// Create and start the server
	srv := server.New(cfg, logger)

	// Set up signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-signalCh
		logger.Info(fmt.Sprintf("Received signal: %v", sig))
		cancel()
	}()

	// Start the server
	if err := srv.Start(ctx); err != nil {
		logger.Error(fmt.Sprintf("Server error: %v", err))
		os.Exit(1)
	}

	logger.Info("Server shutdown complete")
}

func stopServer(cfg *config.Config) {
	// Check if PID file exists
	if _, err := os.Stat(cfg.PidFile); os.IsNotExist(err) {
		fmt.Println("Server is not running (no PID file found)")
		return
	}
	
	// Read PID from file
	pidBytes, err := os.ReadFile(cfg.PidFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading PID file: %v\n", err)
		os.Exit(1)
	}
	
	// Parse PID
	pid := 0
	_, err = fmt.Sscanf(string(pidBytes), "%d", &pid)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid PID in PID file: %v\n", err)
		os.Exit(1)
	}
	
	// Send SIGTERM to process
	process, err := os.FindProcess(pid)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error finding process with PID %d: %v\n", pid, err)
		os.Exit(1)
	}
	
	fmt.Printf("Stopping warpclipd (PID: %d)...\n", pid)
	
	// Send signal
	err = process.Signal(syscall.SIGTERM)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error sending signal to process: %v\n", err)
		os.Exit(1)
	}
	
	// Wait briefly for process to terminate
	fmt.Println("Waiting for process to terminate...")
	for i := 0; i < 5; i++ {
		// Check if process still exists
		if err := process.Signal(syscall.Signal(0)); err != nil {
			fmt.Println("Server stopped successfully")
			// Remove PID file if it still exists
			os.Remove(cfg.PidFile)
			return
		}
		
		// Wait a bit
		time.Sleep(500 * time.Millisecond)
	}
	
	fmt.Println("Server may still be running, consider using 'kill -9' if needed")
}

func showStatus(cfg *config.Config) {
	// Check if PID file exists
	if _, err := os.Stat(cfg.PidFile); os.IsNotExist(err) {
		fmt.Println("Server status: Not running (no PID file found)")
		return
	}
	
	// Read PID from file
	pidBytes, err := os.ReadFile(cfg.PidFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading PID file: %v\n", err)
		os.Exit(1)
	}
	
	// Parse PID
	pid := 0
	_, err = fmt.Sscanf(string(pidBytes), "%d", &pid)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid PID in PID file: %v\n", err)
		os.Exit(1)
	}
	
	// Check if process is running
	process, err := os.FindProcess(pid)
	if err != nil {
		fmt.Printf("Server status: Not running (PID %d not found)\n", pid)
		return
	}
	
	// On Unix, FindProcess always succeeds, so we need to check if the process exists
	err = process.Signal(syscall.Signal(0))
	if err != nil {
		fmt.Printf("Server status: Not running (PID %d exists but process is dead)\n", pid)
		return
	}
	
	fmt.Printf("Server status: Running (PID: %d)\n", pid)
	fmt.Printf("Listening on: %s:%d\n", cfg.BindAddress, cfg.Port)
	
	// Show last clipboard activity if available
	if _, err := os.Stat(cfg.LastFile); err == nil {
		lastBytes, err := os.ReadFile(cfg.LastFile)
		if err == nil {
			fmt.Println("\nLast clipboard activity:")
			fmt.Println(string(lastBytes))
		}
	}
	
	fmt.Println("\nLog file: " + cfg.LogFile)
}

func showHelp() {
	fmt.Println("WarpClip Daemon - Local clipboard service")
	fmt.Println("")
	fmt.Println("USAGE:")
	fmt.Println("  warpclipd [COMMAND]")
	fmt.Println("")
	fmt.Println("COMMANDS:")
	fmt.Println("  start    Start the clipboard daemon (default if no command specified)")
	fmt.Println("  stop     Stop a running daemon")
	fmt.Println("  restart  Restart the daemon")
	fmt.Println("  status   Check daemon status")
	fmt.Println("  help     Show this help message")
	fmt.Println("  version  Show version information")
	fmt.Println("")
	fmt.Println("ENVIRONMENT VARIABLES:")
	fmt.Println("  WARPCLIP_LOCAL_PORT  Override default port (8888)")
	fmt.Println("  WARPCLIP_LOG_FILE    Override log file location")
	fmt.Println("  WARPCLIP_DEBUG_FILE  Override debug log file location")
	fmt.Println("")
	fmt.Println("EXAMPLES:")
	fmt.Println("  warpclipd start      # Start the daemon")
	fmt.Println("  warpclipd status     # Check status")
	fmt.Println("  warpclipd restart    # Restart the daemon")
	fmt.Println("")
	fmt.Println("NOTES:")
	fmt.Println("  This daemon listens on localhost:8888 and copies received data to the clipboard.")
	fmt.Println("  It is designed to be used with the warpclip command on remote servers.")
	fmt.Println("  ")
	fmt.Println("  When installed via Homebrew, the service is managed with:")
	fmt.Println("    brew services start warpclip")
	fmt.Println("    brew services stop warpclip")
	fmt.Println("    brew services restart warpclip")
}

