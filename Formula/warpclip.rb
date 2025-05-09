class Warpclip < Formula
  desc "Remote-to-local clipboard integration for Warp terminal users"
  homepage "https://github.com/mquinnv/warpclip"
  url "https://github.com/mquinnv/warpclip/archive/refs/tags/v1.0.0.tar.gz"
  sha256 "a8bc2dc70074b47fce49b22b51e4e3e6f4fbedd55a88806b5081ab9b9efd677a"
  license "MIT"
  head "https://github.com/mquinnv/warpclip.git", branch: "main"

  depends_on "netcat"
  depends_on :macos

  livecheck do
    url :stable
    regex(/^v?(\d+(?:\.\d+)+)$/i)
  end

  def install
    # Install the binaries
    bin.install "src/warpclip-server.sh"
    bin.install "src/warp-copy"

    # Set the proper permissions
    chmod 0755, bin/"warpclip-server.sh"
    chmod 0755, bin/"warp-copy"

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
    
    # Create .ssh directory if it doesn't exist
    mkdir_p "#{Dir.home}/.ssh" unless Dir.exist?("#{Dir.home}/.ssh")
    chmod 0700, "#{Dir.home}/.ssh" # Secure permissions for .ssh directory
    
    # Create config file if it doesn't exist
    touch ssh_config_path unless File.exist?(ssh_config_path)
    chmod 0600, ssh_config_path # Secure permissions for SSH config
    
    # Check if RemoteForward entry exists
    config_content = File.read(ssh_config_path)
    
    unless config_content.include?("RemoteForward 9999 localhost:8888")
      # Back up existing config first
      backup_path = "#{ssh_config_path}.backup-#{Time.now.strftime("%Y%m%d%H%M%S")}"
      system "cp", ssh_config_path, backup_path
      ohai "Backed up existing SSH config to #{backup_path}"
      
      # Append our configuration
      forward_config = %Q{
# WarpClip SSH Configuration
# Added by Homebrew (#{name}) on #{Time.now.strftime("%Y-%m-%d %H:%M:%S")}
Host *
    RemoteForward 9999 localhost:8888
      }.strip

      File.open(ssh_config_path, "a") do |file|
        file.puts("\n#{forward_config}\n")
      end
      
      ohai "Added RemoteForward configuration to SSH config"
    else
      ohai "SSH RemoteForward already configured"
    end
  end

  # Define the service plist
  service do
    run [opt_bin/"warpclip-server.sh"]
    keep_alive true
    log_path "#{Dir.home}/.warpclip.out.log"
    error_log_path "#{Dir.home}/.warpclip.error.log"
    working_dir "#{Dir.home}"
    environment_variables PATH: "#{HOMEBREW_PREFIX}/bin:/usr/bin:/bin:/usr/sbin:/sbin"

    # Restart if the process exits for any reason
    restart_delay 5
  end

  def caveats
    <<~EOS
      WarpClip has been installed. To start the service:
      
        brew services start warpclip
      
      IMPORTANT: WarpClip consists of two components:
      
      1. LOCAL COMPONENT (warpclip-server.sh):
         • Runs on your Mac and listens for clipboard data
         • Started automatically by Homebrew Services
      
      2. REMOTE COMPONENT (warp-copy):
         • Needs to be copied to remote servers you connect to
         • Sends data back to your Mac through SSH tunnel
      
      To use WarpClip on a remote server:
      
      1. Copy the client script to your remote server:
         scp #{opt_bin}/warp-copy user@remote-server:~/bin/
      
      2. Make it executable:
         ssh user@remote-server "chmod +x ~/bin/warp-copy"
      
      3. Connect to your remote server with SSH forwarding:
         ssh user@remote-server
         (This works automatically if you use the default SSH config)
      
      4. On the remote server, pipe content to warp-copy:
         cat remote-file.txt | warp-copy
      
      The content will be copied to your local Mac clipboard!
      
      Status and troubleshooting:
      
      • Check service status:
        brew services info warpclip
        #{opt_bin}/warpclip-server.sh status
        
      • View logs:
        cat ~/.warpclip.log
        cat ~/.warpclip.debug.log
        
      • Restart service:
        brew services restart warpclip
    EOS
  end

  test do
    assert_predicate opt_bin/"warpclip-server.sh", :exist?
    assert_predicate opt_bin/"warp-copy", :exist?
    
    # Basic syntax check
    system opt_bin/"warpclip-server.sh", "status" rescue nil
    
    # Check if the script has expected content
    assert_match "warpclip server", shell_output("head -n 5 #{opt_bin}/warpclip-server.sh")
  end
end

