// Package sanitize provides input validation for user-supplied MCP parameters
// that get interpolated into shell commands or external API calls.
package sanitize

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	mediaPathRe         = regexp.MustCompile(`^[a-zA-Z0-9/_.\- ]+$`)
	usernameRe          = regexp.MustCompile(`^[a-zA-Z0-9_.\-]+$`)
	driveLetterRe       = regexp.MustCompile(`^[A-Za-z]$`)
	operatorPathRe      = regexp.MustCompile(`^/[a-zA-Z0-9_/]+$`)
	devicePathLinuxRe   = regexp.MustCompile(`^/dev/[a-zA-Z0-9_/]+$`)
	devicePathWindowsRe = regexp.MustCompile(`^\\\\\.\\PHYSICALDRIVE\d+$`)
	mountPointRe        = regexp.MustCompile(`^/[a-zA-Z0-9_/.\-]+$`)
	rcloneRemoteNameRe  = regexp.MustCompile(`^[a-zA-Z0-9_-]+:`)
	rcloneRemotePathRe  = regexp.MustCompile(`^[a-zA-Z0-9_\-./@ ]+$`)
	rcloneLocalPathRe   = regexp.MustCompile(`^[a-zA-Z0-9_\-./@ ]+$`)
	allowedFileSystems  = map[string]bool{
		"ext4": true, "ext3": true, "ext2": true,
		"xfs": true, "btrfs": true, "zfs": true,
		"ntfs": true, "vfat": true, "exfat": true,
		"fat32": true, "hfs+": true, "apfs": true,
		"auto": true,
	}
)

// Username validates a username string.
// Allows alphanumeric characters, underscores, dots, and hyphens.
func Username(s string) error {
	if s == "" {
		return fmt.Errorf("username must not be empty")
	}
	if len(s) > 255 {
		return fmt.Errorf("username too long (max 255 chars)")
	}
	if !usernameRe.MatchString(s) {
		return fmt.Errorf("username contains invalid characters (allowed: a-z, 0-9, _, ., -)")
	}
	return nil
}

// DriveLetter validates a single drive letter.
func DriveLetter(s string) error {
	if !driveLetterRe.MatchString(s) {
		return fmt.Errorf("invalid drive letter %q (must be a single letter A-Z)", s)
	}
	return nil
}

// SafePath validates that the resolved path stays within the given base directory.
// Prevents directory traversal attacks.
func SafePath(base, path string) error {
	if path == "" {
		return fmt.Errorf("path must not be empty")
	}
	if filepath.IsAbs(path) {
		return fmt.Errorf("path %q must be relative", path)
	}
	absBase, err := filepath.Abs(base)
	if err != nil {
		return fmt.Errorf("invalid base path: %w", err)
	}
	joined := filepath.Join(absBase, path)
	resolved, err := filepath.Abs(joined)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}
	if !strings.HasPrefix(resolved, absBase+string(filepath.Separator)) && resolved != absBase {
		return fmt.Errorf("path %q escapes base directory", path)
	}
	return nil
}

// DevicePath validates a block device path.
// Linux: /dev/sdX, /dev/nvmeXnYpZ, /dev/mapper/name
// Windows: \\.\PHYSICALDRIVEX
func DevicePath(s string) error {
	if s == "" {
		return fmt.Errorf("device path must not be empty")
	}
	if devicePathLinuxRe.MatchString(s) || devicePathWindowsRe.MatchString(s) {
		return nil
	}
	return fmt.Errorf("invalid device path %q (expected /dev/... or \\\\.\\PHYSICALDRIVEN)", s)
}

// FileSystemType validates a filesystem type string against an allowlist.
func FileSystemType(s string) error {
	if !allowedFileSystems[strings.ToLower(s)] {
		return fmt.Errorf("invalid filesystem type %q (allowed: ext4, xfs, btrfs, ntfs, vfat, exfat, zfs)", s)
	}
	return nil
}

// MountPoint validates a mount point path.
func MountPoint(s string) error {
	if s == "" {
		return fmt.Errorf("mount point must not be empty")
	}
	if !mountPointRe.MatchString(s) {
		return fmt.Errorf("invalid mount point %q (must be absolute path with safe characters)", s)
	}
	if strings.Contains(s, "..") {
		return fmt.Errorf("mount point %q must not contain '..'", s)
	}
	return nil
}

// OperatorPath validates a TouchDesigner operator path.
func OperatorPath(s string) error {
	if s == "" {
		return fmt.Errorf("operator path must not be empty")
	}
	if len(s) > 512 {
		return fmt.Errorf("operator path too long (max 512 chars)")
	}
	if !operatorPathRe.MatchString(s) {
		return fmt.Errorf("operator path contains invalid characters (must match /[a-zA-Z0-9_/]+)")
	}
	return nil
}

// MediaPath validates a file path for use with ffmpeg/ffprobe or similar media tools.
// Must be absolute, no ".." components, safe characters only, max 4096 chars.
func MediaPath(s string) error {
	if s == "" {
		return fmt.Errorf("media path must not be empty")
	}
	if len(s) > 4096 {
		return fmt.Errorf("media path too long (max 4096 chars)")
	}
	if !filepath.IsAbs(s) {
		return fmt.Errorf("media path %q must be absolute", s)
	}
	if strings.Contains(s, "..") {
		return fmt.Errorf("media path %q must not contain '..'", s)
	}
	if !mediaPathRe.MatchString(s) {
		return fmt.Errorf("media path %q contains invalid characters (allowed: alphanumeric, /, ., -, _, space)", s)
	}
	return nil
}

// RclonePath validates an rclone path, which may be local or remote.
// Remote paths have the form "remotename:path/to/thing".
func RclonePath(s string) error {
	if s == "" {
		return fmt.Errorf("rclone path must not be empty")
	}
	if len(s) > 4096 {
		return fmt.Errorf("rclone path too long (max 4096 chars)")
	}

	if idx := strings.Index(s, ":"); idx > 0 {
		remoteName := s[:idx+1]
		if !rcloneRemoteNameRe.MatchString(remoteName) {
			return fmt.Errorf("invalid rclone remote name in %q (must match [a-zA-Z0-9_-]+:)", s)
		}
		remotePath := s[idx+1:]
		if remotePath != "" {
			if strings.Contains(remotePath, "..") {
				return fmt.Errorf("rclone remote path %q must not contain '..'", s)
			}
			if !rcloneRemotePathRe.MatchString(remotePath) {
				return fmt.Errorf("rclone remote path %q contains invalid characters", s)
			}
		}
		return nil
	}

	if !filepath.IsAbs(s) {
		return fmt.Errorf("local rclone path %q must be absolute", s)
	}
	if strings.Contains(s, "..") {
		return fmt.Errorf("rclone path %q must not contain '..'", s)
	}
	if !rcloneLocalPathRe.MatchString(s) {
		return fmt.Errorf("rclone path %q contains invalid characters", s)
	}
	return nil
}
