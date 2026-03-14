package sanitize

import (
	"testing"
)

func TestUsername(t *testing.T) {
	good := []string{"alice", "bob_123", "user.name", "dj-mix", "A", "user_name.123-test"}
	for _, s := range good {
		if err := Username(s); err != nil {
			t.Errorf("Username(%q) should pass, got: %v", s, err)
		}
	}

	bad := []string{"", "user name", "user;rm -rf", "user'drop", "../etc", "a/b", "user\nname", "$(cmd)"}
	for _, s := range bad {
		if err := Username(s); err == nil {
			t.Errorf("Username(%q) should fail", s)
		}
	}
}

func TestDriveLetter(t *testing.T) {
	good := []string{"C", "D", "a", "z"}
	for _, s := range good {
		if err := DriveLetter(s); err != nil {
			t.Errorf("DriveLetter(%q) should pass, got: %v", s, err)
		}
	}

	bad := []string{"", "CC", "1", "C:", "/", ".."}
	for _, s := range bad {
		if err := DriveLetter(s); err == nil {
			t.Errorf("DriveLetter(%q) should fail", s)
		}
	}
}

func TestSafePath(t *testing.T) {
	base := "/tmp/scdl"

	good := []string{"user/likes", "alice", "a/b/c"}
	for _, s := range good {
		if err := SafePath(base, s); err != nil {
			t.Errorf("SafePath(%q, %q) should pass, got: %v", base, s, err)
		}
	}

	bad := []string{"", "../../etc/passwd", "../../../root", "/absolute/path"}
	for _, s := range bad {
		if err := SafePath(base, s); err == nil {
			t.Errorf("SafePath(%q, %q) should fail", base, s)
		}
	}
}

func TestDevicePath(t *testing.T) {
	good := []string{"/dev/sda", "/dev/sda1", "/dev/nvme0n1p1", "/dev/mapper/unraid_crypt", `\\.\PHYSICALDRIVE0`, `\\.\PHYSICALDRIVE12`}
	for _, s := range good {
		if err := DevicePath(s); err != nil {
			t.Errorf("DevicePath(%q) should pass, got: %v", s, err)
		}
	}

	bad := []string{
		"", "/dev/", "sda", "/dev/sd a", "/dev/sda;rm -rf /",
		`\\.\PHYSICALDRIVE`, `\\.\PHYSICALDRIVE-1`,
		"/etc/passwd", "$(malicious)", "/dev/../etc/shadow",
	}
	for _, s := range bad {
		if err := DevicePath(s); err == nil {
			t.Errorf("DevicePath(%q) should fail", s)
		}
	}
}

func TestFileSystemType(t *testing.T) {
	good := []string{"ext4", "xfs", "btrfs", "ntfs", "vfat", "exfat", "zfs", "auto", "EXT4", "XFS"}
	for _, s := range good {
		if err := FileSystemType(s); err != nil {
			t.Errorf("FileSystemType(%q) should pass, got: %v", s, err)
		}
	}

	bad := []string{"", "ext4; rm -rf /", "$(cmd)", "fakefs", "reiserfs"}
	for _, s := range bad {
		if err := FileSystemType(s); err == nil {
			t.Errorf("FileSystemType(%q) should fail", s)
		}
	}
}

func TestMountPoint(t *testing.T) {
	good := []string{"/mnt/unraid", "/mnt/recovery", "/media/disk1", "/tmp/test-mount"}
	for _, s := range good {
		if err := MountPoint(s); err != nil {
			t.Errorf("MountPoint(%q) should pass, got: %v", s, err)
		}
	}

	bad := []string{"", "relative/path", "/mnt/../etc/shadow", "/mnt/unraid;rm -rf /", "/mnt/un raid"}
	for _, s := range bad {
		if err := MountPoint(s); err == nil {
			t.Errorf("MountPoint(%q) should fail", s)
		}
	}
}

func TestOperatorPath(t *testing.T) {
	good := []string{"/project1", "/project1/base1", "/project1/timer1", "/a/b/c/d"}
	for _, s := range good {
		if err := OperatorPath(s); err != nil {
			t.Errorf("OperatorPath(%q) should pass, got: %v", s, err)
		}
	}

	bad := []string{"", "project1", "/path with spaces", "/path;inject", "/path'quote", "../traversal"}
	for _, s := range bad {
		if err := OperatorPath(s); err == nil {
			t.Errorf("OperatorPath(%q) should fail", s)
		}
	}
}

func TestMediaPath(t *testing.T) {
	good := []string{
		"/Users/alice/Music/track.mp3",
		"/home/user/Videos/my-video.mp4",
		"/tmp/audio_file.wav",
		"/mnt/media/DJ Sets/live set.aiff",
	}
	for _, s := range good {
		if err := MediaPath(s); err != nil {
			t.Errorf("MediaPath(%q) should pass, got: %v", s, err)
		}
	}

	bad := []string{
		"",
		"relative/path.mp3",
		"/music/../etc/passwd",
		"/music/track;rm -rf /.mp3",
		"/music/track$(cmd).mp3",
		"/music/track`cmd`.mp3",
		"/music/track|cat.mp3",
		"/music/track&bg.mp3",
		"/music/track'quote.mp3",
	}
	for _, s := range bad {
		if err := MediaPath(s); err == nil {
			t.Errorf("MediaPath(%q) should fail", s)
		}
	}

	longPath := "/" + string(make([]byte, 4096))
	if err := MediaPath(longPath); err == nil {
		t.Error("MediaPath with >4096 chars should fail")
	}
}

func TestRclonePath(t *testing.T) {
	good := []string{
		"gdrive:backups/myfiles",
		"gdrive:",
		"s3:bucket/key",
		"my-remote:path/to/folder",
		"/mnt/data/folder",
		"/home/user/documents",
		"/data/vj clips/set1",
	}
	for _, s := range good {
		if err := RclonePath(s); err != nil {
			t.Errorf("RclonePath(%q) should pass, got: %v", s, err)
		}
	}

	bad := []string{
		"",
		"relative/path",
		"../etc/passwd",
		"/mnt/../etc/shadow",
		"gdrive:../../../etc/passwd",
		"/path;rm -rf /",
		"/path$(cmd)",
		"bad remote:path",
		":path",
	}
	for _, s := range bad {
		if err := RclonePath(s); err == nil {
			t.Errorf("RclonePath(%q) should fail", s)
		}
	}
}
