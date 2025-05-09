class Warpclip < Formula
  desc "Remote-to-local clipboard integration for terminal users"
  homepage "https://github.com/mquinnv/warpclip"
  url "https://github.com/mquinnv/warpclip/archive/refs/tags/v1.2.1.tar.gz"
  sha256 "db9e5d7d107a219bca8d837058340d83eae48e796b9008f318bc30f784716f66"
  license "MIT"
  head "https://github.com/mquinnv/warpclip.git", branch: "main"

  livecheck do
    url :stable
    regex(/^v(\d+\.\d+\.\d+)$/i)
  end

  depends_on :macos
  depends_on "netcat"

  def install
    # Install the main command-line tool
    bin.install "bin/warpclip"

    # Install the server daemon (renaming it for clarity)
    # Install the original daemon script with a .real extension
    bin.install "src/warpclipd" => "warpclipd.real"
    
    # Create a wrapper script with process management
    (bin/"warpclipd").write <<~EOS
      #!/bin/bash
      #
      # warpclipd - Secure clipboard daemon with process management
      #
      
      # Configuration
      PID_FILE="${HOME}/.warpclip.pid"
      LOG_FILE="${HOME}/.warpclip.log"
      DEBUG_LOG="${HOME}/.warpclip.debug.log"
      
      # Function to clean up on exit
      cleanup() {
        echo "$(date): Shutting down warpclip daemon" >> "${LOG_FILE}"
        if [ -f "${PID_FILE}" ]; then
          rm -f "${PID_FILE}"
        fi
        # Kill any remaining netcat processes
        pkill -f "nc -l 127.0.0.1" 2>/dev/null
        exit 0
      }
      
      # Set up signal handlers
      trap cleanup SIGINT SIGTERM EXIT
      
      # Check if already running
      if [ -f "${PID_FILE}" ]; then
        PID=$(cat "${PID_FILE}")
        if ps -p "${PID}" >/dev/null 2>&1; then
          echo "warpclipd is already running (PID: ${PID})"
          exit 1
        else
          # Stale PID file
          rm -f "${PID_FILE}"
        fi
      fi
      
      # Record current PID
      echo $$ > "${PID_FILE}"
      chmod 600 "${PID_FILE}"
      
      # Start the actual daemon with localhost binding
      echo "$(date): Starting warpclip daemon (PID: $$)" >> "${LOG_FILE}"
      exec #{opt_bin}/warpclipd.real "$@"
    EOS
    
    # Modify the daemon to always bind to localhost only (127.0.0.1)
    # This is a security enhancement to prevent exposure to the network
    inreplace bin/"warpclipd.real", "nc -l $PORT", "nc -l 127.0.0.1 $PORT"
    
    # Set the proper permissions
    chmod 0755, bin/"warpclip"
    chmod 0755, bin/"warpclipd"
    chmod 0755, bin/"warpclipd.real"

    # Install example files to share directory
    share.install "etc/com.user.warpclip.plist"
    share.install "examples/ssh_config" => "warpclip-ssh-config-example"
  end

  def post_install
    # Create log and PID files with proper permissions
    log_file = "#{Dir.home}/.warpclip.log"
    debug_file = "#{Dir.home}/.warpclip.debug.log"
    pid_file = "#{Dir.home}/.warpclip.pid"

    unless File.exist?(log_file)
      touch log_file
      chmod 0600, log_file
    end

    unless File.exist?(debug_file)
      touch debug_file
      chmod 0600, debug_file
    end
    
    # Remove stale PID file if it exists
    if File.exist?(pid_file)
      rm_f pid_file
    end

    # Setup SSH config
    setup_ssh_config

    # Print instructions for loading the service
    ohai "WarpClip installation complete. Start the service with:"
    puts "  brew services start warpclip"
  end

  def setup_ssh_config
    ssh_config_path = "#{Dir.home}/.ssh/config"
    ssh_dir = "#{Dir.home}/.ssh"
    success = true

    # First check if RemoteForward is already configured before making any changes
    if File.exist?(ssh_config_path) && File.readable?(ssh_config_path)
      begin
        config_content = File.read(ssh_config_path)
      rescue
        config_content = ""
      end
      if config_content.include?("RemoteForward 9999 localhost:8888")
        ohai "SSH RemoteForward already configured in #{ssh_config_path}"
        return true
      end
    end

    # Create .ssh directory if it doesn't exist
    unless Dir.exist?(ssh_dir)
      begin
        mkdir_p ssh_dir
        # Only set permissions on newly created directory
        chmod 0700, ssh_dir
        ohai "Created SSH directory at #{ssh_dir}"
      rescue => e
        opoo "Could not create or set permissions on #{ssh_dir}: #{e.message}"
        opoo "You may need to manually configure SSH forwarding."
        success = false
      end
    end

    # Skip further steps if we couldn't set up the directory
    return false unless Dir.exist?(ssh_dir)

    # Create config file if it doesn't exist
    unless File.exist?(ssh_config_path)
      begin
        touch ssh_config_path
        chmod 0600, ssh_config_path
        ohai "Created SSH config at #{ssh_config_path}"
      rescue => e
        opoo "Could not create or set permissions on #{ssh_config_path}: #{e.message}"
        opoo "You may need to manually configure SSH forwarding."
        success = false
      end
    end

    # Skip if we can't read/write the config file
    return false unless File.readable?(ssh_config_path)
    return false unless File.writable?(ssh_config_path)

    # Read current config content
    config_content = ""
    begin
      config_content = File.read(ssh_config_path)
    rescue => e
      opoo "Could not read SSH config: #{e.message}"
      return false
    end

    # Check if RemoteForward entry already exists
    if config_content.include?("RemoteForward 9999 localhost:8888")
      ohai "SSH RemoteForward already configured in #{ssh_config_path}"
      return true
    else
      # Back up existing config first
      backup_path = "#{ssh_config_path}.backup-#{Time.now.strftime("%Y%m%d%H%M%S")}"
      begin
        cp ssh_config_path, backup_path
        ohai "Backed up existing SSH config to #{backup_path}"
      rescue => e
        opoo "Could not back up SSH config: #{e.message}. Will continue without backup."
      end
      # Append our configuration
      forward_config = %Q{
# WarpClip SSH Configuration
# Added by Homebrew (#{name}) on #{Time.now.strftime("%Y-%m-%d %H:%M:%S")}
Host *
    RemoteForward 9999 localhost:8888
      }.strip
      modified = false
      begin
        File.open(ssh_config_path, "a") do |file|
          file.puts("\n#{forward_config}\n")
        end
        modified = true
        ohai "Added RemoteForward configuration to SSH config"
      rescue => e
        opoo "Could not modify SSH config: #{e.message}"
        opoo "You may need to add the RemoteForward configuration manually:"
        puts "  #{forward_config}"
        success = false
      end

      # Verify the configuration was actually added
      if modified
        begin
          new_content = File.read(ssh_config_path)
          if new_content.include?("RemoteForward 9999 localhost:8888")
            ohai "Verified SSH RemoteForward configuration was added successfully"
          else
            opoo "Failed to verify SSH config was updated. You may need to add it manually:"
            puts "  #{forward_config}"
            success = false
          end
        rescue => e
          opoo "Could not verify SSH config: #{e.message}"
          success = false
        end
      end
    end

    if success
      ohai "SSH configuration completed successfully"
    else
      opoo "SSH configuration may not be complete. Check the messages above."
      opoo "WarpClip will still work if you manually configure SSH forwarding with:"
      puts "  RemoteForward 9999 localhost:8888"
    end
    # Return true even with warnings - this will allow post-install to continue
    true
  end

  # Define the service plist
  service do
    # Run the warpclipd daemon (hard-coded to bind only to localhost for security)
    run [opt_bin/"warpclipd"]
    keep_alive true
    
    # Proper logging setup
    log_path "#{Dir.home}/.warpclip.out.log"
    error_log_path "#{Dir.home}/.warpclip.error.log"
    working_dir Dir.home
    
    # Include necessary environment variables
    environment_variables PATH: "#{HOMEBREW_PREFIX}/bin:/usr/bin:/bin:/usr/sbin:/sbin"

    # Restart if the process exits for any reason (with delay to prevent rapid restarts)
    restart_delay 5
  end

  def caveats
    <<~EOS
      WarpClip has been installed. To start the clipboard service:

        brew services start warpclip

      IMPORTANT: WarpClip consists of two components:
      1. LOCAL COMPONENT (warpclipd):
         • Runs on your Mac and listens for clipboard data
         • Started automatically by Homebrew Services

      2. REMOTE COMPONENT (warpclip):
         • Needs to be installed on remote servers you connect to
         • Sends data back to your Mac through SSH tunnel

      To use WarpClip on a remote server:
      1. Install the client script on your remote server:
         #{opt_bin}/warpclip install-remote user@remote-server

      2. Connect to your remote server with SSH forwarding:
         ssh user@remote-server
         (This works automatically if you use the default SSH config)

      3. On the remote server, pipe content to warpclip:
         cat remote-file.txt | warpclip

      The content will be copied to your local Mac clipboard!
      Available commands:

      • Copy data to clipboard (on remote server):
        cat file.txt | warpclip

      • Install on a remote server (from local machine):
        warpclip install-remote user@remote-server

      • Show help and usage information:
        warpclip help

      Status and troubleshooting:
      • Check service status:
        brew services info warpclip
        #{opt_bin}/warpclipd status

      • View logs:
        cat ~/.warpclip.log
        cat ~/.warpclip.debug.log
      • Restart service:
        brew services restart warpclip
    EOS
  end

  test do
    assert_path_exists "#{opt_bin}/warpclipd"
    assert_path_exists "#{opt_bin}/warpclip"

    # Basic syntax check for warpclip
    system opt_bin/"warpclip", "--version"

    # Basic syntax check for warpclipd
    begin
      system opt_bin/"warpclipd", "status"
    rescue
      nil
    end
    # Check if the scripts have expected content
    assert_match "WarpClip v#{version}", shell_output("#{opt_bin}/warpclip --version")
    assert_match "warpclip server", shell_output("head -n 10 #{opt_bin}/warpclipd.real")
    assert_match "cleanup", shell_output("head -n 30 #{opt_bin}/warpclipd")
  end
end
