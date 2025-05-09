# WarpClip 📋

<div align="center">

[![Version](https://img.shields.io/badge/version-2.1.0-blue.svg)](https://github.com/mquinnv/warpclip/releases)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![Platform](https://img.shields.io/badge/platform-macOS-lightgrey.svg)]()
[![Homebrew](https://img.shields.io/badge/homebrew-coming%20soon-orange.svg)]()
[![Release Status](https://img.shields.io/github/workflow/status/mquinnv/warpclip/release?label=release)]()

**Remote-to-local clipboard integration for Warp terminal users**

<img src="https://raw.githubusercontent.com/mquinnv/warpclip/main/assets/logo.txt" alt="WarpClip" width="180"/>

</div>

## 📖 Overview

WarpClip creates a seamless bridge between your remote SSH sessions and your local macOS clipboard. It enables you to pipe content from remote servers directly to your local clipboard with a simple command, similar to `pbcopy` on macOS or iTerm2's built-in clipboard integration.

```bash
# On a remote server
cat remote_file.txt | warpclip
```

Instantly, the content of `remote_file.txt` appears in your local clipboard, ready to paste anywhere on your macOS machine.

## ✨ Features

- **📋 Seamless Clipboard Integration**: Copy text from remote servers directly to your local clipboard
- **🔒 Secure Transmission**: All data is transmitted over your existing SSH connection
- **🛠️ Automatic Setup**: SSH forwarding is configured automatically
- **⚡ Near-Zero Latency**: Copies happen almost instantly, even over slow connections
- **📊 Status Monitoring**: Check service status and view copy history
- **🔄 Persistent Service**: Runs in the background and auto-restarts if needed
- **🧩 Portable**: Go-based remote client works on almost any Linux/Unix system without dependencies
- **⚙️ Robust**: Signal handling and error recovery for reliable operation

## 🔍 Requirements

- **macOS** (uses `pbcopy` for clipboard operations)
- **SSH client** with port forwarding capabilities
- **Go 1.18+** (only if building from source)

> **Note:** As of version 2.1.0, the remote client is written in Go and no longer requires netcat!

## 🚀 Installation

### Homebrew Installation (Coming Soon)

```bash
# Install via Homebrew (coming soon)
brew install mquinnv/tap/warpclip

# This will automatically set up all components
```

### Manual Installation (From Source)

```bash
# Clone the repository
git clone https://github.com/mquinnv/warpclip.git
cd warpclip

# Build the Go binaries
go build -o bin/warpclip cmd/warpclip/main.go
go build -o bin/warpclipd cmd/warpclipd/main.go

# Run the installer
./install.sh
```

### Manual Installation

If you prefer to install components manually:

1. **Create necessary directories:**

   ```bash
   mkdir -p ~/bin
   mkdir -p ~/Library/LaunchAgents
   ```

2. **Install the local server component:**

   ```bash
   cp src/warpclip-server.sh ~/bin/
   chmod +x ~/bin/warpclip-server.sh
   ```

3. **Set up the LaunchAgent:**

   ```bash
   # Copy the plist file (make sure to replace /Users/michael with your home directory)
   cp etc/com.user.warpclip.plist ~/Library/LaunchAgents/
   # Edit the file to replace the home directory path
   sed -i '' "s|/Users/michael|$HOME|g" ~/Library/LaunchAgents/com.user.warpclip.plist
   # Load the agent
   launchctl load ~/Library/LaunchAgents/com.user.warpclip.plist
   ```

4. **Update your SSH config:**

   Add to your `~/.ssh/config`:

   ```
   Host *
       RemoteForward 9999 localhost:8888
   ```

5. **Copy the remote client for future use:**

   ```bash
   cp bin/warpclip ~/bin/
   ```

> **Note:** The client is now a compiled Go binary instead of a shell script.

## 🛠️ How to Use

### Setting Up a Remote Server

Before you can use WarpClip on a remote server, you need to copy the `warpclip` binary to that server:

```bash
# Copy the binary to your remote server
scp ~/bin/warpclip user@remote-server:~/bin/

# Make it executable (if needed)
ssh user@remote-server "chmod +x ~/bin/warpclip"
```

### Copying Content to Your Clipboard

Once the script is on your remote server, you can use it to copy content:

```bash
# Pipe content to warpclip
cat file.txt | warpclip

# Or redirect input
warpclip < file.txt

# Copy command output directly
grep "important" large-log.txt | warpclip

# Copy multiline output
find . -name "*.js" | warpclip
```

The content will be instantly available in your local clipboard!

## 🔍 How It Works

WarpClip consists of three main components:

1. **Local Server**: A persistent Go service (`warpclipd`) that runs on your Mac and listens on port 8888
2. **SSH Tunnel**: Automatically set up when you connect to a remote server, forwarding port 9999 on the remote to port 8888 on your local machine
3. **Remote Client**: The `warpclip` Go binary that sends data to the forwarded port

When you pipe content to `warpclip` on a remote server, it securely transmits the data through the SSH tunnel to your local WarpClip server, which then copies it to your clipboard using `pbcopy`.

> **Version 2.1.0 Update:** The remote client has been completely rewritten in Go, eliminating the need for netcat and providing improved error handling, signal management, and reliability.

## 🔧 Troubleshooting

### Check Service Status

```bash
~/bin/warpclip-server.sh status
```

This will tell you if the service is running and when the last clipboard operation occurred.

### View Logs

```bash
# View main log
cat ~/.warpclip.log

# View debug log for more details
cat ~/.warpclip.debug.log
```

### Restart the Service

If the service isn't responding correctly:

```bash
launchctl unload ~/Library/LaunchAgents/com.user.warpclip.plist
launchctl load ~/Library/LaunchAgents/com.user.warpclip.plist
```

### Common Issues

**Connection Refused**

```
Error: SSH tunnel not detected on port 9999.
```

This usually means the SSH port forwarding isn't set up correctly. Check your SSH config and try reconnecting to the server.

**No Data Copied**

If data isn't appearing in your clipboard, check:
- Is the WarpClip service running? (`~/bin/warpclip-server.sh status` or `warpclipd status`)
- Did you connect to the server with SSH port forwarding enabled?
- Is the connection timing out? Check for firewall issues or network connectivity problems.

## 🔐 Security Considerations

### SSH Tunneling Security

WarpClip uses SSH's secure tunneling for all data transfer, which means:

- All clipboard data is encrypted during transmission
- No new network ports are exposed to the internet
- Data is transmitted through your existing SSH session

### Clipboard Security

When using WarpClip, be aware of these clipboard-related security considerations:

- Content copied to your clipboard persists until replaced, potentially leading to unintentional sharing
- The clipboard is a system-wide resource accessible to all applications on your computer
- Consider using a clipboard manager with auto-clear functionality for sensitive data

### Port Forwarding Considerations

The default configuration uses automatic port forwarding for all SSH connections, which has some implications:

- Anyone with access to a remote server could potentially send data to your clipboard
- The `warpclip` client doesn't encrypt data before sending it (relies on SSH encryption)
- Clipboard tunneling works even from jump hosts or nested SSH sessions

For enhanced security:

1. **Limit port forwarding to specific hosts**:
   ```
   # Instead of using the wildcard Host *
   Host trusted-server-1 trusted-server-2
       RemoteForward 9999 localhost:8888
   ```

2. **Use non-standard ports** to reduce collision risks:
   ```
   Host production-server
       RemoteForward 12345 localhost:8888
   ```

3. **Add authentication** to the local clipboard service (consider submitting a PR!)

### Network Considerations

- The local service listens only on `localhost` interface, not exposing network ports externally
- SSH tunnels are established only when you initiate an SSH connection
- Check corporate security policies regarding automatic port forwarding

## 👥 Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

### Development Setup

1. Fork the repository
2. Create your feature branch: `git checkout -b feature/amazing-feature`
3. Commit your changes: `git commit -am 'Add some amazing feature'`
4. Push to the branch: `git push origin feature/amazing-feature`
5. Submit a pull request

### Coding Standards

- Follow Go best practices and idioms for Go code
- Use standard Go formatting (`go fmt`)
- Include comprehensive error handling
- Add comments for complex operations
- Write tests for critical components
- Update documentation when adding features

## 📝 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

<div align="center">
  Developed with ❤️ for terminal users everywhere
</div>

