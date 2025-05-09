# WarpClip 📋

<div align="center">

[![Version](https://img.shields.io/badge/version-1.0.0-blue.svg)](https://github.com/michael/warpclip/releases)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![Platform](https://img.shields.io/badge/platform-macOS-lightgrey.svg)]()

**Remote-to-local clipboard integration for terminal users**

<img src="https://raw.githubusercontent.com/michael/warpclip/assets/logo.png" alt="WarpClip Logo" width="180"/>

</div>

## 📖 Overview

WarpClip creates a seamless bridge between your remote SSH sessions and your local macOS clipboard. It enables you to pipe content from remote servers directly to your local clipboard with a simple command, similar to `pbcopy` on macOS or iTerm2's built-in clipboard integration.

```bash
# On a remote server
cat remote_file.txt | warp-copy
```

Instantly, the content of `remote_file.txt` appears in your local clipboard, ready to paste anywhere on your macOS machine.

## ✨ Features

- **📋 Seamless Clipboard Integration**: Copy text from remote servers directly to your local clipboard
- **🔒 Secure Transmission**: All data is transmitted over your existing SSH connection
- **🛠️ Automatic Setup**: SSH forwarding is configured automatically
- **⚡ Near-Zero Latency**: Copies happen almost instantly, even over slow connections
- **📊 Status Monitoring**: Check service status and view copy history
- **🔄 Persistent Service**: Runs in the background and auto-restarts if needed
- **🧩 Portable**: Remote script works on almost any Linux/Unix system

## 🔍 Requirements

- **macOS** (uses `pbcopy` for clipboard operations)
- **Bash 3.2+**
- **SSH client** with port forwarding capabilities
- **Netcat (`nc`)** on both local and remote machines

## 🚀 Installation

### Quick Installation (Recommended)

```bash
# Clone the repository
git clone https://github.com/michael/warpclip.git
cd warpclip

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

5. **Copy the remote script for future use:**

   ```bash
   cp src/warp-copy ~/bin/
   ```

## 🛠️ How to Use

### Setting Up a Remote Server

Before you can use WarpClip on a remote server, you need to copy the `warp-copy` script to that server:

```bash
# Copy the script to your remote server
scp ~/bin/warp-copy user@remote-server:~/bin/

# Make it executable
ssh user@remote-server "chmod +x ~/bin/warp-copy"
```

### Copying Content to Your Clipboard

Once the script is on your remote server, you can use it to copy content:

```bash
# Pipe content to warp-copy
cat file.txt | warp-copy

# Or redirect input
warp-copy < file.txt

# Copy command output directly
grep "important" large-log.txt | warp-copy

# Copy multiline output
find . -name "*.js" | warp-copy
```

The content will be instantly available in your local clipboard!

## 🔍 How It Works

WarpClip consists of three main components:

1. **Local Server**: A persistent service that runs on your Mac and listens on port 8888
2. **SSH Tunnel**: Automatically set up when you connect to a remote server, forwarding port 9999 on the remote to port 8888 on your local machine
3. **Remote Client**: The `warp-copy` script that sends data to the forwarded port

When you pipe content to `warp-copy` on a remote server, it securely transmits the data through the SSH tunnel to your local WarpClip server, which then copies it to your clipboard using `pbcopy`.

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
- Is the WarpClip service running? (`~/bin/warpclip-server.sh status`)
- Did you connect to the server with SSH port forwarding enabled?
- Is netcat (`nc`) installed on the remote server?

## 🔐 Security Considerations

WarpClip uses SSH's secure tunneling for all data transfer, which means:

- All clipboard data is encrypted during transmission
- No new network ports are exposed to the internet
- Data is transmitted through your existing SSH session

However, there are some security considerations:

- Anyone with access to your remote server could potentially send data to your clipboard
- The `warp-copy` script doesn't encrypt data before sending it (relies on SSH encryption)
- The port forwarding is automatic for all SSH connections unless customized

For sensitive environments, consider restricting port forwarding to specific hosts instead of using the wildcard `Host *` configuration.

## 👥 Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

### Development Setup

1. Fork the repository
2. Create your feature branch: `git checkout -b feature/amazing-feature`
3. Commit your changes: `git commit -am 'Add some amazing feature'`
4. Push to the branch: `git push origin feature/amazing-feature`
5. Submit a pull request

### Coding Standards

- Keep shell scripts POSIX-compatible where possible
- Include error handling for edge cases
- Add comments for complex operations
- Update documentation when adding features

## 📝 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

<div align="center">
  Developed with ❤️ for terminal users everywhere
</div>

