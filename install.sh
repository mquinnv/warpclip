#!/usr/bin/env bash

# WarpClip Installer Script
# This script installs WarpClip components on your local macOS system

# Set to exit immediately if a command fails
set -e

# Colors for prettier output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
BOLD='\033[1m'
NC='\033[0m' # No Color

# Configuration
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
SRC_DIR="$SCRIPT_DIR/src"
ETC_DIR="$SCRIPT_DIR/etc"
BIN_DIR="$HOME/bin"
LAUNCH_AGENTS_DIR="$HOME/Library/LaunchAgents"
SSH_CONFIG="$HOME/.ssh/config"
PLIST_FILE="$LAUNCH_AGENTS_DIR/com.user.warpclip.plist"
VERSION="1.0.0"
TIMESTAMP=$(date +"%Y%m%d%H%M%S")

# Function to print section headers
print_header() {
    echo -e "\n${BLUE}${BOLD}==== $1 ====${NC}\n"
}

# Function to print success messages
print_success() {
    echo -e "${GREEN}✓ $1${NC}"
}

# Function to print warning messages
print_warning() {
    echo -e "${YELLOW}⚠ $1${NC}"
}

# Function to print error messages
print_error() {
    echo -e "${RED}✗ $1${NC}"
}

# Function to back up a file
backup_file() {
    local file="$1"
    if [ -f "$file" ]; then
        local backup="${file}.${TIMESTAMP}.bak"
        cp "$file" "$backup"
        print_warning "Backed up existing file to: $backup"
    fi
}

# Display welcome banner
cat <<EOF
${BLUE}${BOLD}
 __      __                     .__                  
/  \    /  \_____ _______  ____ |  | _______  ______
\   \/\/   /\__  \\_  __ \/  _ \|  |/ /\__  \ \____ \\
 \        /  / __ \|  | \(  <_> )    <  / __ \|  |_> >
  \__/\  /  (____  /__|   \____/|__|_ \(____  /   __/ 
       \/        \/                  \/     \/|__|    
${NC}
${BOLD}WarpClip Installer v${VERSION}${NC}
Seamless remote-to-local clipboard integration

EOF

# Check if running on macOS
if [[ "$OSTYPE" != "darwin"* ]]; then
    print_error "This script requires macOS to run. Exiting."
    exit 1
fi

# 1. Create necessary directories
print_header "Setting up directories"
if [ ! -d "$BIN_DIR" ]; then
    mkdir -p "$BIN_DIR"
    print_success "Created $BIN_DIR directory"
else
    print_success "$BIN_DIR directory already exists"
fi

if [ ! -d "$LAUNCH_AGENTS_DIR" ]; then
    mkdir -p "$LAUNCH_AGENTS_DIR"
    print_success "Created $LAUNCH_AGENTS_DIR directory"
else
    print_success "$LAUNCH_AGENTS_DIR directory already exists"
fi

# Make sure ~/.ssh exists
if [ ! -d "$HOME/.ssh" ]; then
    mkdir -p "$HOME/.ssh"
    chmod 700 "$HOME/.ssh"
    print_success "Created $HOME/.ssh directory"
else
    print_success "$HOME/.ssh directory already exists"
fi

# 2. Install server script
print_header "Installing WarpClip server script"
backup_file "$BIN_DIR/warpclip-server.sh"
cp "$SRC_DIR/warpclip-server.sh" "$BIN_DIR/"
chmod +x "$BIN_DIR/warpclip-server.sh"
print_success "Installed and set permissions on warpclip-server.sh"

# 3. Install client script
print_header "Installing WarpClip client script"
backup_file "$BIN_DIR/warp-copy"
cp "$SRC_DIR/warp-copy" "$BIN_DIR/"
chmod +x "$BIN_DIR/warp-copy"
print_success "Installed and set permissions on warp-copy"

# 4. Set up LaunchAgent
print_header "Setting up LaunchAgent"
    backup_file "$PLIST_FILE"
    # Copy plist file but replace any instances of /Users/michael with the actual home directory
    sed "s|/Users/michael|$HOME|g" "$ETC_DIR/com.user.warpclip.plist" > "$PLIST_FILE"
    print_success "Created LaunchAgent plist file"

# 5. Update SSH config if it doesn't already have the RemoteForward entry
print_header "Updating SSH configuration"
backup_file "$SSH_CONFIG"

if grep -q "RemoteForward 9999 localhost:8888" "$SSH_CONFIG" 2>/dev/null; then
    print_warning "SSH RemoteForward already configured"
else
    echo -e "\n# WarpClip SSH Configuration" >> "$SSH_CONFIG"
    echo "# Added automatically by WarpClip installer on $(date)" >> "$SSH_CONFIG"
    echo "Host *" >> "$SSH_CONFIG"
    echo "    RemoteForward 9999 localhost:8888" >> "$SSH_CONFIG"
    print_success "Added RemoteForward to SSH config"
fi

# 6. Load LaunchAgent
print_header "Loading LaunchAgent"
launchctl unload "$PLIST_FILE" 2>/dev/null || true
if launchctl load "$PLIST_FILE"; then
    print_success "LaunchAgent loaded successfully"
else
    print_error "Failed to load LaunchAgent. You may need to load it manually."
fi

# Add PATH to include ~/bin if not already there
if [[ ":$PATH:" != *":$BIN_DIR:"* ]]; then
    print_warning "$BIN_DIR is not in your PATH. Add the following to your shell profile:"
    echo 'export PATH="$HOME/bin:$PATH"'
    echo ""
fi

# 7. Display instructions
print_header "WarpClip Installation Complete"
cat <<EOT
${GREEN}${BOLD}Local Setup:${NC}
  ✓ Server script installed at: $BIN_DIR/warpclip-server.sh
  ✓ LaunchAgent installed at: $PLIST_FILE 
  ✓ SSH configured for automatic port forwarding

${GREEN}${BOLD}Usage Instructions:${NC}

1. ${BLUE}${BOLD}On Remote Servers:${NC}
   - Copy the warp-copy script to your remote server:
     scp $BIN_DIR/warp-copy user@remote-server:~/bin/

   - Make it executable:
     ssh user@remote-server "chmod +x ~/bin/warp-copy"

2. ${BLUE}${BOLD}How to Use:${NC}
   - On the remote server, use warp-copy to send content to your local clipboard:
     cat file.txt | warp-copy
     OR
     warp-copy < file.txt

3. ${BLUE}${BOLD}Troubleshooting:${NC}
   - Check WarpClip status:
     $BIN_DIR/warpclip-server.sh status
   
   - Check logs at:
     ~/.warpclip.log
     ~/.warpclip.debug.log
   
   - Restart the service:
     launchctl unload $PLIST_FILE
     launchctl load $PLIST_FILE
   
   - Verify SSH tunnel is working:
     ssh -v user@remote-server

${GREEN}${BOLD}Enjoy using WarpClip!${NC}
EOT

# Run a status check to verify installation
echo ""
$BIN_DIR/warpclip-server.sh status || true

