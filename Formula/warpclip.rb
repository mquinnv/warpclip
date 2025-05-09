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
    bin.install "src/warpclipd" => "warpclipd"

    # Set the proper permissions
    chmod 0755, bin/"warpclip"
    chmod 0755, bin/"warpclipd"

    # Install example files to share directory
    share.install "etc/com.user.warpclip.plist"
    share.install "examples/ssh_config" => "warpclip-ssh-config-example"
  end

  def post_install
    # Create log directory and files with proper permissions
    log_file = "#{Dir.home}/.warpclip.log"
    debug_file = "#{Dir.home}/.warpclip.debug.log"

    unless File.exist?(log_file)
      touch log_file
      chmod 0600, log_file
    end

    unless File.exist?(debug_file)
      touch debug_file
      chmod 0600, debug_file
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

    # Create .ssh directory if it doesn't exist, and set secure permissions only if we create it
    if !Dir.exist?(ssh_dir)
      mkdir_p ssh_dir
      # Only set permissions on newly created directory
      begin
        chmod 0700, ssh_dir
      rescue Errno::EPERM
        opoo "Could not set permissions on #{ssh_dir}. This is not critical."
      end
    end

    # Create config file if it doesn't exist, and set permissions only if we create it
    if !File.exist?(ssh_config_path)
      touch ssh_config_path
      begin
        chmod 0600, ssh_config_path
      rescue Errno::EPERM
        opoo "Could not set permissions on #{ssh_config_path}. This is not critical."
      end
    end

    # Check if RemoteForward entry exists
    config_content = File.read(ssh_config_path)

    if config_content.include?("RemoteForward 9999 localhost:8888")
      ohai "SSH RemoteForward already configured"
    else
      # Back up existing config first
      backup_path = "#{ssh_config_path}.backup-#{Time.now.strftime("%Y%m%d%H%M%S")}"
      begin
        FileUtils.cp ssh_config_path, backup_path
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

      begin
        File.open(ssh_config_path, "a") do |file|
          file.puts("\n#{forward_config}\n")
        end
        ohai "Added RemoteForward configuration to SSH config"
      rescue => e
        opoo "Could not modify SSH config: #{e.message}. You may need to add the RemoteForward configuration manually."
      end

    end
  end

  # Define the service plist
  service do
    run [opt_bin/"warpclipd"]
    keep_alive true
    log_path "#{Dir.home.to_s}/.warpclip.out.log"
    error_log_path "#{Dir.home.to_s}/.warpclip.error.log"
    working_dir Dir.home.to_s
    environment_variables PATH: "#{HOMEBREW_PREFIX}/bin:/usr/bin:/bin:/usr/sbin:/sbin"

    # Restart if the process exits for any reason
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
    assert_match "warpclip server", shell_output("head -n 10 #{opt_bin}/warpclipd")
  end
end
