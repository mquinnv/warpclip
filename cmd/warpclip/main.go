package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	Version = "2.1.3" // Increment from previous versions
	DefaultPort = 9999
	Timeout = 5 * time.Second
)

func main() {
	// Define command line flags
	var port int
	var showHelp bool
	var showVersion bool

	flag.IntVar(&port, "port", DefaultPort, "Specify custom port")
	flag.IntVar(&port, "p", DefaultPort, "Specify custom port (shorthand)")
	flag.BoolVar(&showHelp, "help", false, "Show help message")
	flag.BoolVar(&showHelp, "h", false, "Show help message (shorthand)")
	flag.BoolVar(&showVersion, "version", false, "Show version information")
	flag.BoolVar(&showVersion, "v", false, "Show version information (shorthand)")
	
	// Parse flags
	flag.Parse()
	
	// Show version and exit if requested
	if showVersion {
		fmt.Printf("WarpClip Remote Client v%s\n", Version)
		os.Exit(0)
	}
	
	// Show help and exit if requested
	if showHelp {
		printHelp()
		os.Exit(0)
	}
	
	// Check for commands
	if len(flag.Args()) > 0 {
		cmd := flag.Args()[0]
		switch cmd {
		case "help":
			printHelp()
			os.Exit(0)
		case "install-remote":
			if len(flag.Args()) < 2 {
				fmt.Fprintf(os.Stderr, "Error: Missing remote host argument\n")
				fmt.Fprintf(os.Stderr, "Usage: warpclip install-remote user@host\n")
				os.Exit(1)
			}
			host := flag.Args()[1]
			if err := installRemote(host); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			fmt.Fprintf(os.Stderr, "WarpClip successfully installed on the remote host!\n")
			os.Exit(0)
		}
	}
	
// We're going to skip the isEmpty check to avoid consuming stdin data
// This check was causing problems because it consumed data from stdin
// that was then not available to sendToClipboard

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
    // Read all input into a buffer first (simpler and more reliable)
    var buf bytes.Buffer
    _, err := io.Copy(&buf, os.Stdin)
    if err != nil {
        return fmt.Errorf("error reading stdin: %w", err)
    }
    
    data := buf.Bytes()
    
    // Print debug information
    fmt.Fprintf(os.Stderr, "Read %d bytes from stdin\n", len(data))
    
    // Verify we have data
    if len(data) == 0 {
        fmt.Fprintln(os.Stderr, "Error: No input provided. Please provide content via stdin.")
        fmt.Fprintln(os.Stderr, "Examples:")
        fmt.Fprintln(os.Stderr, "  cat file.txt | warpclip")
        fmt.Fprintln(os.Stderr, "  echo 'text' | warpclip")
        fmt.Fprintln(os.Stderr, "  warpclip < file.txt")
        return fmt.Errorf("no data received from stdin")
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
        return fmt.Errorf("SSH tunnel not available")
    }
	
	// Set up the connection with timeout
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), Timeout)
	if err != nil {
		return fmt.Errorf("failed to connect to localhost:%d: %w", port, err)
	}
	defer conn.Close()
	
	// Set deadlines for writing
	deadline := time.Now().Add(Timeout)
	if err := conn.SetWriteDeadline(deadline); err != nil {
		return fmt.Errorf("failed to set write deadline: %w", err)
	}
	
	// Write data directly for simplicity
    fmt.Fprintf(os.Stderr, "Sending %d bytes to clipboard...\n", len(data))
    if _, err := conn.Write(data); err != nil {
        return fmt.Errorf("failed to write data: %w", err)
    }
	
	// Try to close write side if this is a TCPConn
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.CloseWrite()
	}
	
	// Wait for either completion or context cancellation
	select {
	case <-ctx.Done():
		return fmt.Errorf("operation canceled")
	default:
		// Operation completed successfully
		return nil
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
	fmt.Println("Usage: cat file.txt | warpclip [options]")
	fmt.Println("   or: warpclip [options] < file.txt")
	fmt.Println("   or: warpclip install-remote user@host")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  install-remote HOST  Install warpclip on a remote host")
	fmt.Println("  help                 Show this help message")
	fmt.Println("")
	fmt.Println("Options:")
	fmt.Println("  --port, -p PORT      Specify custom port (default: 9999)")
	fmt.Println("  --help, -h           Show this help message")
	fmt.Println("")
	fmt.Println("WarpClip copies content from the remote server to your local macOS clipboard")
	fmt.Println("via a secure SSH tunnel. Make sure you connected with port forwarding enabled.")
}

// installRemote installs warpclip on a remote host
func installRemote(host string) error {
    // First, detect the remote OS
    osType, err := detectRemoteOS(host)
    if err != nil {
        return fmt.Errorf("failed to detect remote OS: %w", err)
    }

    fmt.Fprintf(os.Stderr, "Detected remote OS: %s\n", osType)

    switch osType {
    case "Linux":
        return installLinuxRemote(host)
    case "Darwin":
        return installDarwinRemote(host)
    default:
        return fmt.Errorf("unsupported remote OS: %s", osType)
    }
}

// detectRemoteOS determines the OS type of the remote host
func detectRemoteOS(host string) (string, error) {
    cmd := exec.Command("ssh", host, "uname -s")
    output, err := cmd.Output()
    if err != nil {
        return "", fmt.Errorf("failed to detect remote OS: %w", err)
    }
    return strings.TrimSpace(string(output)), nil
}

// Release represents a GitHub release
type Release struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name        string `json:"name"`
		DownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// installLinuxRemote installs warpclip on a Linux remote host
func installLinuxRemote(host string) error {
    fmt.Fprintf(os.Stderr, "Installing warpclip on Linux host %s...\n", host)

    // Check if already installed
    if checkRemoteFile(host, "/usr/local/bin/warpclip") {
        fmt.Fprintf(os.Stderr, "WarpClip is already installed. Updating...\n")
    }

    // Create temporary directory on remote host
    tmpDir := fmt.Sprintf("/tmp/warpclip-%d", time.Now().UnixNano())
    if err := executeRemoteCommand(host, fmt.Sprintf("mkdir -p %s", tmpDir)); err != nil {
        return fmt.Errorf("failed to create temporary directory: %w", err)
    }
    defer executeRemoteCommand(host, fmt.Sprintf("rm -rf %s", tmpDir)) // Clean up

    // Fetch latest release info from GitHub
    fmt.Fprintf(os.Stderr, "Fetching latest release from GitHub...\n")
    releaseInfo, err := getLatestRelease()
    if err != nil {
        return fmt.Errorf("failed to fetch release info: %w", err)
    }

    // Find Linux binary in assets
    var downloadURL string
    for _, asset := range releaseInfo.Assets {
        if asset.Name == "warpclip-linux-amd64" {
            downloadURL = asset.DownloadURL
            break
        }
    }
    
    if downloadURL == "" {
        return fmt.Errorf("could not find Linux binary in release assets")
    }

    // Download the binary to the remote host
    fmt.Fprintf(os.Stderr, "Downloading binary from GitHub release: %s\n", downloadURL)
    downloadCmd := fmt.Sprintf("curl -L '%s' -o %s/warpclip", downloadURL, tmpDir)
    if err := executeRemoteCommand(host, downloadCmd); err != nil {
        return fmt.Errorf("failed to download binary: %w", err)
    }

    // Verify download was successful
    if err := executeRemoteCommand(host, fmt.Sprintf("test -f %s/warpclip", tmpDir)); err != nil {
        return fmt.Errorf("binary download appears to have failed: %w", err)
    }
    
    // Calculate and verify checksum (if available)
    checksumResult, err := verifyBinaryChecksum(host, tmpDir, releaseInfo.TagName)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Warning: Checksum verification failed: %v\n", err)
        fmt.Fprintf(os.Stderr, "Continuing with installation anyway...\n")
    } else if checksumResult {
        fmt.Fprintf(os.Stderr, "Checksum verification successful\n")
    }

    // Install commands (adjusted for fish shell compatibility)
    commands := []string{
        "sudo mkdir -p /usr/local/bin",
        fmt.Sprintf("sudo mv %s/warpclip /usr/local/bin/warpclip", tmpDir),
        "sudo chmod +x /usr/local/bin/warpclip",
    }

    // Execute commands
    for _, cmd := range commands {
        fmt.Fprintf(os.Stderr, "Running: %s\n", cmd)
        if err := executeRemoteCommand(host, cmd); err != nil {
            return fmt.Errorf("installation failed during command '%s': %w", cmd, err)
        }
    }

    // Verify installation
    if err := executeRemoteCommand(host, "which warpclip"); err != nil {
        return fmt.Errorf("installation verification failed: %w", err)
    }

    // Verify version
    if err := executeRemoteCommand(host, "warpclip --help | grep -q 'v" + Version + "'"); err != nil {
        return fmt.Errorf("version verification failed: binary might be corrupted")
    }

    fmt.Fprintf(os.Stderr, "Successfully installed warpclip v%s on %s\n", Version, host)
    return nil
}

// getLatestRelease fetches the latest release information from GitHub
func getLatestRelease() (*Release, error) {
    url := "https://api.github.com/repos/mquinnv/warpclip/releases/latest"
    
    // Create HTTP client with timeout
    client := &http.Client{Timeout: 30 * time.Second}
    
    // Create request with user agent (required by GitHub API)
    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        return nil, fmt.Errorf("failed to create request: %w", err)
    }
    req.Header.Set("User-Agent", "WarpClip-Installer")
    
    // Make the request
    resp, err := client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("failed to fetch release info: %w", err)
    }
    defer resp.Body.Close()
    
    // Check response status
    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
    }
    
    // Parse the response
    var release Release
    if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
        return nil, fmt.Errorf("failed to parse release info: %w", err)
    }
    
    return &release, nil
}

// verifyBinaryChecksum verifies the checksum of the downloaded binary
func verifyBinaryChecksum(host, tmpDir, version string) (bool, error) {
    // Try to download the checksums file
    checksumURL := fmt.Sprintf("https://github.com/mquinnv/warpclip/releases/download/%s/checksums.txt", version)
    checksumPath := fmt.Sprintf("%s/checksums.txt", tmpDir)
    
    // Download checksums file to remote host
    downloadCmd := fmt.Sprintf("curl -L '%s' -o %s || echo 'Not found'", checksumURL, checksumPath)
    if err := executeRemoteCommand(host, downloadCmd); err != nil {
        return false, fmt.Errorf("failed to download checksums file: %w", err)
    }
    
    // Check if checksums file exists
    if err := executeRemoteCommand(host, fmt.Sprintf("test -f %s", checksumPath)); err != nil {
        return false, fmt.Errorf("checksums file not found")
    }
    
    // Calculate SHA256 checksum of the binary
    calcSumCmd := fmt.Sprintf("sha256sum %s/warpclip | cut -d ' ' -f 1", tmpDir)
    calcSumCmdOutput, err := exec.Command("ssh", host, calcSumCmd).Output()
    if err != nil {
        return false, fmt.Errorf("failed to calculate checksum: %w", err)
    }
    
    calculatedSum := strings.TrimSpace(string(calcSumCmdOutput))
    
    // Extract expected checksum from checksums file
    grepCmd := fmt.Sprintf("grep 'warpclip-linux-amd64' %s | cut -d ' ' -f 1", checksumPath)
    expectedSumOutput, err := exec.Command("ssh", host, grepCmd).Output()
    if err != nil {
        return false, fmt.Errorf("failed to extract expected checksum: %w", err)
    }
    
    expectedSum := strings.TrimSpace(string(expectedSumOutput))
    
    // Verify checksums match
    if calculatedSum == "" || expectedSum == "" {
        return false, fmt.Errorf("failed to get checksums for comparison")
    }
    
    if calculatedSum != expectedSum {
        return false, fmt.Errorf("checksum mismatch. Expected: %s, got: %s", expectedSum, calculatedSum)
    }
    
    return true, nil
}

// installDarwinRemote installs warpclip on a macOS remote host
func installDarwinRemote(host string) error {
    fmt.Fprintf(os.Stderr, "Installing warpclip on macOS host %s...\n", host)

    // Check if Homebrew is installed
    hasHomebrew, err := checkRemoteHomebrew(host)
    if err != nil {
        return err
    }

    if !hasHomebrew {
        return fmt.Errorf("Homebrew not found on remote macOS host. Please install Homebrew first")
    }

    // Install via Homebrew
    commands := []string{
        "brew update",
        "brew install mquinnv/tap/warpclip",
        "brew services start warpclip",
    }

    for _, cmd := range commands {
        fmt.Fprintf(os.Stderr, "Running: %s\n", cmd)
        if err := executeRemoteCommand(host, cmd); err != nil {
            return fmt.Errorf("installation failed: %w", err)
        }
    }

    fmt.Fprintf(os.Stderr, "Successfully installed warpclip on %s\n", host)
    return nil
}

// checkRemoteHomebrew checks if Homebrew is installed on the remote host
func checkRemoteHomebrew(host string) (bool, error) {
    err := executeRemoteCommand(host, "which brew")
    return err == nil, nil
}

// executeRemoteCommand executes a command on the remote host
func executeRemoteCommand(host, command string) error {
    cmd := exec.Command("ssh", host, command)
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    return cmd.Run()
}

// checkRemoteFile checks if a file exists on the remote host
func checkRemoteFile(host, path string) bool {
    err := executeRemoteCommand(host, fmt.Sprintf("test -f %s", path))
    return err == nil
}
