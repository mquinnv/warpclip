# Changelog

## [2.1.0] - 2025-05-09

### Fixed
- Resolved issues with double connections from SSH port forwarding
- Fixed empty data transmission issues
- Improved clipboard data handling reliability

### Added
- Better connection identification (data vs. control connections)
- Improved debug logging
- Retry logic for clipboard operations
- Maximum data size enforcement
- Connection timeouts and deadlines

### Changed
- More efficient buffered data reading
- Enhanced error reporting
- Simplified client stdin handling

## [2.0.0] - 2025-05-09

### Changed
- Reimplemented warpclipd in Go, removing netcat dependency
- Improved error handling and logging
- Added proper signal handling and graceful shutdown
- Better connection management and security

### Added
- Comprehensive logging with multiple log levels
- Log rotation capability
- Proper PID file management
- Status tracking for clipboard operations
- Native TCP server implementation

### Removed
- Dependency on netcat (nc) command
- Shell script implementation of warpclipd

### Security
- Server now enforces localhost-only binding
- Secure file permissions for all created files
- Input sanitization for logging
- Maximum data size limits

