package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

const (
	Version = "2.1.0" // Increment from previous warp-copy version (1.0.0) and warpclipd (2.0.0)
	DefaultPort = 9999
	Timeout = 5 * time.Second
)

func main() {
	// Define command line flags
	var port int
	var showHelp bool

	flag.IntVar(&port, "port", DefaultPort, "Specify custom port")
	flag.IntVar(&port, "p", DefaultPort, "Specify custom port (shorthand)")
	flag.BoolVar(&showHelp, "help", false, "Show help message")
	flag.BoolVar(&showHelp, "h", false, "Show help message (shorthand)")
	
	// Parse flags
	flag.Parse()
	
	// Show help and exit if requested
	if showHelp {
		printHelp()
		os.Exit(0)
	}
	
// Check if we have any input
	if isEmpty(os.Stdin) {
		fmt.Fprintln(os.Stderr, "Error: No input provided. Please provide content via stdin.")
		fmt.Fprintln(os.Stderr, "Examples:")
		fmt.Fprintln(os.Stderr, "  cat file.txt | warp-copy")
		fmt.Fprintln(os.Stderr, "  echo 'text' | warp-copy")
		fmt.Fprintln(os.Stderr, "  warp-copy < file.txt")
		os.Exit(1)
	}

	// Check if SSH tunnel is available
	if !checkTunnel(port) {
		fmt.Fprintf(os.Stderr, "Error: SSH tunnel not detected on port %d.\n", port)
		fmt.Fprintln(os.Stderr, "Make sure you connected with SSH using RemoteForward option:")
		fmt.Fprintf(os.Stderr, "  ssh -R %d:localhost:8888 user@%s\n", port, getHostname())
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Or add to your ~/.ssh/config:")
		fmt.Fprintf(os.Stderr, "  Host %s\n", getHostname())
		fmt.Fprintf(os.Stderr, "      RemoteForward %d localhost:8888\n", port)
		os.Exit(1)
	}
	
	fmt.Fprintln(os.Stderr, "Sending input to clipboard...")
	
	// Set up context with signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	// Set up signal handling for graceful shutdown
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)
	
	// Create a WaitGroup to ensure we clean up properly
	var wg sync.WaitGroup
	
	// Start a goroutine to handle signals
	wg.Add(1)
	var interruptReceived bool
	go func() {
		defer wg.Done()
		select {
		case sig := <-signalCh:
			fmt.Fprintf(os.Stderr, "\nReceived signal: %v. Canceling operation...\n", sig)
			interruptReceived = true
			cancel()
		case <-ctx.Done():
			// Context was canceled elsewhere, just exit
		}
	}()
	
	// Send data from stdin to the clipboard
	err := sendToClipboard(ctx, port)
	
	// Cancel the context in case sendToClipboard returned naturally
	cancel()
	
	// Wait for signal handler to complete
	wg.Wait()
	
	// Handle the result
	if interruptReceived {
		fmt.Fprintln(os.Stderr, "Operation canceled by user.")
		os.Exit(1)
	} else if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		fmt.Fprintln(os.Stderr, "Failed to copy content to clipboard.")
		os.Exit(1)
	}
	
	fmt.Fprintln(os.Stderr, "Content copied to clipboard successfully!")
}

// checkTunnel verifies if the SSH tunnel is properly set up
func checkTunnel(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), 1*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// isEmpty checks if there is any data available on the reader
func isEmpty(r io.Reader) bool {
	// Create a bufio.Reader to peek at the first byte
	stdin := bufio.NewReader(r)
	
	// Try to peek at the first byte
	_, err := stdin.Peek(1)
	
	// If we got an EOF, the input is empty
	if err == io.EOF {
		return true
	}
	
	// If we got some other error, we can't determine if it's empty
	// For safety, assume it's not empty
	if err != nil {
		return false
	}
	
	// If we got no error, there's at least one byte, so not empty
	return false
}

// sendToClipboard sends data from stdin to the clipboard service
func sendToClipboard(ctx context.Context, port int) error {
	// Set up the connection with timeout
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), Timeout)
	if err != nil {
		return fmt.Errorf("failed to connect to localhost:%d: %w", port, err)
	}
	defer conn.Close()
	
	// Set deadline for the entire operation
	deadline := time.Now().Add(Timeout)
	if err := conn.SetDeadline(deadline); err != nil {
		return fmt.Errorf("failed to set connection deadline: %w", err)
	}
	
	// Create a pipe for coordinated reading and writing
	pr, pw := io.Pipe()
	defer pr.Close()
	defer pw.Close()
	
	// Create a channel to capture errors from the goroutine
	errCh := make(chan error, 1)
	
	// Start a goroutine to copy data from stdin to the pipe
	go func() {
		_, err := io.Copy(pw, os.Stdin)
		if err != nil {
			errCh <- fmt.Errorf("failed to read from stdin: %w", err)
		}
		pw.Close()
		errCh <- nil
	}()
	
	// Copy data from the pipe reader to the network connection
	copyDone := make(chan error, 1)
	go func() {
		_, err := io.Copy(conn, pr)
		if err != nil {
			copyDone <- fmt.Errorf("failed to send data: %w", err)
		}
		copyDone <- nil
	}()
	
	// Wait for either completion, error, or context cancellation
	select {
	case err := <-errCh:
		if err != nil {
			return err
		}
		// Wait for copy to complete
		return <-copyDone
	case err := <-copyDone:
		return err
	case <-ctx.Done():
		// Context canceled (probably due to a signal)
		return fmt.Errorf("operation canceled")
	}
}

// getHostname returns the hostname of the current system
func getHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "remote-host"
	}
	return hostname
}

// printHelp prints the help message
func printHelp() {
	fmt.Printf("WarpClip Remote Client v%s\n", Version)
	fmt.Println("Usage: cat file.txt | warp-copy [options]")
	fmt.Println("   or: warp-copy [options] < file.txt")
	fmt.Println("")
	fmt.Println("Options:")
	fmt.Println("  --port, -p PORT    Specify custom port (default: 9999)")
	fmt.Println("  --help, -h         Show this help message")
	fmt.Println("")
	fmt.Println("WarpClip copies content from the remote server to your local macOS clipboard")
	fmt.Println("via a secure SSH tunnel. Make sure you connected with port forwarding enabled.")
}

