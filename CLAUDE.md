# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

WarpClip is a macOS tool that enables remote-to-local clipboard integration for SSH sessions. It allows users to pipe content from remote servers directly to their local clipboard through SSH tunneling.

## Architecture

The system consists of three main components:

1. **warpclipd** (`cmd/warpclipd/main.go`) - Local daemon that runs on macOS, listens on port 8888, and handles clipboard operations
2. **warpclip** (`cmd/warpclip/main.go`) - CLI client that runs on remote servers and sends data through SSH tunnels to the local daemon
3. **SSH tunnel configuration** - Automatically forwards remote port 9999 to local port 8888

Key internal packages:
- `internal/config/` - Configuration management with environment variable support
- `internal/log/` - Structured logging functionality
- `internal/server/` - Core server implementation for clipboard operations

## Development Commands

### Build
```bash
# Build both binaries
go build -o bin/warpclip cmd/warpclip/main.go
go build -o bin/warpclipd cmd/warpclipd/main.go

# Or build all with cross-compilation for releases
go build -o dist/warpclip-darwin-amd64 cmd/warpclip/main.go
go build -o dist/warpclipd-darwin-amd64 cmd/warpclipd/main.go
```

### Testing
```bash
# Run all tests
go test ./...

# Run tests for specific packages
go test ./internal/config/
go test ./internal/log/
go test ./internal/server/
```

### Development Setup
```bash
# Install locally for development
./install.sh

# Check daemon status
~/bin/warpclip-server.sh status
# Or if using Go binary directly:
./bin/warpclipd status
```

## Key Configuration

- Default local port: 8888 (configurable via WARPCLIP_LOCAL_PORT)
- Default remote tunnel port: 9999
- Log files: `~/.warpclip.log`, `~/.warpclip.debug.log`
- SSH config automatically adds: `RemoteForward 9999 localhost:8888`

## Version Management

The current version is defined in:
- `cmd/warpclip/main.go` (const Version)
- `cmd/warpclipd/main.go` (const Version)
- `VERSION` file
- Homebrew formula at `homebrew-tap/Formula/warpclip.rb`

When updating versions, ensure all locations are synchronized.