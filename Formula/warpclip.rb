class Warpclip < Formula
  desc "Remote-to-local clipboard integration for Warp terminal users"
  homepage "https://github.com/mquinnv/warpclip"
  url "https://github.com/mquinnv/warpclip/archive/refs/tags/v1.0.0.tar.gz"
  sha256 "00000000000000000000000000000000000000000000000000000000deadbeef" # This will need to be updated with the actual hash after the first release
  license "MIT"
  head "https://github.com/mquinnv/warpclip.git", branch: "main"

  depends_on "netcat"

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

    # Install LaunchAgent template
    prefix.install "etc/com.user.warpclip.plist"
  end

  def post_install
    # Create user LaunchAgent directory if it doesn't exist
    user_launch_agents_path = "#{Dir.home}/Library/LaunchAgents"
    mkdir_p user_launch_agents_path unless Dir.exist?(user_launch_agents_path)

    # Customize and install the plist file
    plist_path = "#{Dir.home}/Library/LaunchAgents/com.user.warpclip.plist"
    
    # Make a backup of existing file if it exists
    if File.exist?(plist_path)
      backup_path = "#{plist_path}.backup-#{Time.now.strftime("%Y%m%d%H%M%S")}"
      system "cp", plist_path, backup_path
      ohai "Backed up existing LaunchAgent to #{backup_path}"
    end

    # Copy plist template
    plist_template = "#{prefix}/com.user.warpclip.plist"
    plist_content = File.read(plist_template)
    
    # Replace home directory path with user's actual home directory
    plist_content.gsub!("/Users/michael", Dir.home)
    
    # Replace binary path with Homebrew binary path
    plist_content.gsub!("~/bin/warpclip-server.sh", "#{bin}/warpclip-server.sh")
    
    # Write the customized plist
    File.write(plist_path, plist_content)
    chmod 0644, plist_path
    
    # Setup SSH config
    setup_ssh_config
    
    # Load the LaunchAgent
    system "launchctl", "unload", plist_path rescue nil
    system "launchctl", "load", plist_path
    ohai "Loaded and started the WarpClip LaunchAgent"
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
# Added by Homebrew on #{Time.now.strftime("%Y-%m-%d %H:%M:%S")}
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

  def caveats
    <<~EOS
      WarpClip has been installed and the service has been started.

      ✓ Server script: #{bin}/warpclip-server.sh
      ✓ Client script: #{bin}/warp-copy
      ✓ LaunchAgent: ~/Library/LaunchAgents/com.user.warpclip.plist
      ✓ SSH configuration: RemoteForward added to ~/.ssh/config

      To use warp-copy on a remote server:
      
      1. Copy the client script to your remote server:
         scp #{bin}/warp-copy user@remote-server:~/bin/
      
      2. Make it executable:
         ssh user@remote-server "chmod +x ~/bin/warp-copy"
      
      3. Use it on the remote server:
         cat file.txt | warp-copy
      
      Status and troubleshooting:
      
      • Check service status:
        #{bin}/warpclip-server.sh status
        
      • View logs:
        cat ~/.warpclip.log
        
      • Restart service:
        launchctl unload ~/Library/LaunchAgents/com.user.warpclip.plist
        launchctl load ~/Library/LaunchAgents/com.user.warpclip.plist
    EOS
  end

  test do
    assert_predicate bin/"warpclip-server.sh", :exist?
    assert_predicate bin/"warp-copy", :exist?
    
    # Basic syntax check
    system bin/"warpclip-server.sh", "status"
    
    # Check if the script has expected content
    assert_match "warpclip server", shell_output("head -n 5 #{bin}/warpclip-server.sh")
  end
end

