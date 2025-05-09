# Changelog

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

