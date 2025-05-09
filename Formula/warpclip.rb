class Warpclip < Formula
  desc "Remote-to-local clipboard integration for terminal users"
  homepage "https://github.com/mquinnv/warpclip"
  url "https://github.com/mquinnv/warpclip/archive/refs/tags/v2.1.1.tar.gz"
  sha256 "1da8468757e966ac716fba7db048155dffd68752b362c4dad7ef24c3fe7a34d2"
  license "MIT"
  head "https://github.com/mquinnv/warpclip.git", branch: "main"

  livecheck do
    url :stable
    regex(/^v(\d+\.\d+\.\d+)$/i)
  end

  depends_on :macos
  depends_on "go" => :build

  def install
    # Build the Go server daemon
    system "go", "build", "-o", bin/"warpclipd", 
           "-ldflags", "-X main.Version=#{version}",
           "./cmd/warpclipd"
    
    # Build the Go client
    system "go", "build", "-o", bin/"warpclip",
           "-ldflags", "-X main.Version=#{version}",
           "./cmd/warpclip"
    
    # Set the proper permissions
    chmod 0755, bin/"warpclip"
    chmod 0755, bin/"warpclipd"

    # Install example files to share directory
    share.install "etc/com.user.warpclip.plist"
    share.install "examples/ssh_config" => "warpclip-ssh-config-example"
  end

  def post_install
    # Create log files with proper permissions
    ["#{Dir.home}/.warpclip.log",
     "#{Dir.home}/.warpclip.debug.log",
     "#{Dir.home}/.warpclip.out.log",
     "#{Dir.home}/.warpclip.error.log"].each do |f|
      unless File.exist?(f)
        touch f
        chmod 0600, f
      end
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
    # Run the warpclipd daemon (with built-in localhost-only binding)
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
         • Implemented in Go for improved security and reliability
         
      2. REMOTE COMPONENT (warpclip):
         • Needs to be installed on remote servers you connect to
         • Now implemented in Go with no external dependencies
         • Sends data back to your Mac through SSH tunnel

      To use WarpClip on a remote server:
      1. Copy the warpclip binary to your remote server:
         scp #{opt_bin}/warpclip user@remote-server:~/bin/

      2. Connect to your remote server with SSH forwarding:
         ssh user@remote-server
         (This works automatically if you use the default SSH config)

      3. On the remote server, pipe content to warpclip:
         cat remote-file.txt | warpclip

      The content will be copied to your local Mac clipboard!
      Available commands:

      • Copy data to clipboard (on remote server):
        cat file.txt | warpclip
        echo "text" | warpclip
        warpclip < file.txt

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

    # Check version output for both binaries
    assert_match "v#{version}", shell_output("#{opt_bin}/warpclip --help")
    assert_match "v#{version}", shell_output("#{opt_bin}/warpclipd --version")

    # Basic syntax check for warpclipd
    system "#{opt_bin}/warpclipd", "--help"
  end
end
