//go:build linux

package device

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/unix"
)

func init() {
	RegisterProvider(func() DeviceProvider {
		return &gridSerialProviderLinux{}
	})
}

// gridSerialProviderLinux discovers and connects to Grid CDC serial ports on Linux.
// Serial connections are output-only (LED feedback) — they don't emit input events.
type gridSerialProviderLinux struct {
	mu    sync.Mutex
	conns map[DeviceID]*gridSerialConnectionLinux
}

func (p *gridSerialProviderLinux) Name() string             { return "grid_serial" }
func (p *gridSerialProviderLinux) DeviceTypes() []DeviceType { return []DeviceType{TypeGenericHID} }

func (p *gridSerialProviderLinux) Enumerate(ctx context.Context) ([]Info, error) {
	// Walk /sys/class/tty/ttyACM* to find CDC-ACM serial devices.
	matches, err := filepath.Glob("/sys/class/tty/ttyACM*")
	if err != nil {
		return nil, err
	}

	var infos []Info
	for _, sysPath := range matches {
		vid, pid := readSysfsUSBIDs(sysPath)
		if !IsGridDevice(vid, pid) {
			continue
		}

		devName := filepath.Base(sysPath)
		devPath := "/dev/" + devName

		infos = append(infos, Info{
			ID:           DeviceID("grid_serial:" + devPath),
			Name:         "Intech Grid CDC device",
			Type:         TypeGenericHID,
			Connection:   ConnUSB,
			VendorID:     vid,
			ProductID:    pid,
			Manufacturer: "Intech Studio",
			Capabilities: Capabilities{HasLEDs: true, Encoders: 16},
			PlatformPath: devPath,
			ProviderName: "grid_serial",
		})
	}

	return infos, nil
}

// readSysfsUSBIDs walks up from a /sys/class/tty/ttyACM* path to find
// the USB device's idVendor and idProduct in sysfs.
func readSysfsUSBIDs(ttyPath string) (uint16, uint16) {
	// Resolve symlink to find actual device path.
	resolved, err := filepath.EvalSymlinks(ttyPath)
	if err != nil {
		return 0, 0
	}

	// Walk up to find the USB device directory containing idVendor/idProduct.
	dir := resolved
	for i := 0; i < 8; i++ {
		dir = filepath.Dir(dir)
		if dir == "/" || dir == "." {
			break
		}
		vidStr := readSysfsFile(filepath.Join(dir, "idVendor"))
		pidStr := readSysfsFile(filepath.Join(dir, "idProduct"))
		if vidStr != "" && pidStr != "" {
			var vid, pid uint16
			if _, err := fmt.Sscanf(vidStr, "%x", &vid); err != nil {
				continue
			}
			if _, err := fmt.Sscanf(pidStr, "%x", &pid); err != nil {
				continue
			}
			return vid, pid
		}
	}
	return 0, 0
}

// readSysfsFile reads a single-line sysfs attribute file.
func readSysfsFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func (p *gridSerialProviderLinux) Open(ctx context.Context, id DeviceID) (DeviceConnection, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.conns == nil {
		p.conns = make(map[DeviceID]*gridSerialConnectionLinux)
	}
	if existing, ok := p.conns[id]; ok {
		return existing, nil
	}

	infos, err := p.Enumerate(ctx)
	if err != nil {
		return nil, err
	}
	var info Info
	found := false
	for _, inf := range infos {
		if inf.ID == id {
			info = inf
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("grid serial device %s not found", id)
	}

	conn, err := openGridSerialLinux(info)
	if err != nil {
		return nil, err
	}
	p.conns[id] = conn
	return conn, nil
}

func (p *gridSerialProviderLinux) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, conn := range p.conns {
		conn.Close()
	}
	p.conns = nil
	return nil
}

// ---------------------------------------------------------------------------
// Serial connection (Linux)
// ---------------------------------------------------------------------------

type gridSerialConnectionLinux struct {
	info    Info
	fd      *os.File
	mu      sync.Mutex
	closed  bool
	events  chan Event
	readBuf []byte
}

func openGridSerialLinux(info Info) (*gridSerialConnectionLinux, error) {
	fd, err := os.OpenFile(info.PlatformPath, os.O_RDWR|unix.O_NOCTTY|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, fmt.Errorf("open serial %s: %w", info.PlatformPath, err)
	}

	// Configure raw serial: 115200 baud, 8N1, no flow control.
	rawFD := int(fd.Fd())
	var termios unix.Termios
	if _, err := unix.IoctlGetTermios(rawFD, unix.TCGETS); err != nil {
		fd.Close()
		return nil, fmt.Errorf("get termios: %w", err)
	}

	// Raw mode.
	termios.Iflag &^= unix.IGNBRK | unix.BRKINT | unix.PARMRK | unix.ISTRIP |
		unix.INLCR | unix.IGNCR | unix.ICRNL | unix.IXON
	termios.Oflag &^= unix.OPOST
	termios.Lflag &^= unix.ECHO | unix.ECHONL | unix.ICANON | unix.ISIG | unix.IEXTEN
	termios.Cflag &^= unix.CSIZE | unix.PARENB
	termios.Cflag |= unix.CS8 | unix.CLOCAL | unix.CREAD

	// Set baud rate.
	termios.Ispeed = unix.B115200
	termios.Ospeed = unix.B115200

	// VMIN=0, VTIME=1 (100ms read timeout).
	termios.Cc[unix.VMIN] = 0
	termios.Cc[unix.VTIME] = 1

	if err := unix.IoctlSetTermios(rawFD, unix.TCSETS, &termios); err != nil {
		fd.Close()
		return nil, fmt.Errorf("set termios: %w", err)
	}

	// Clear nonblock now that termios is set.
	if err := unix.SetNonblock(rawFD, false); err != nil {
		fd.Close()
		return nil, fmt.Errorf("clear nonblock: %w", err)
	}

	slog.Info("grid serial opened", "path", info.PlatformPath, "device", info.ID)

	return &gridSerialConnectionLinux{
		info:    info,
		fd:      fd,
		events:  make(chan Event),
		readBuf: make([]byte, 0, 512),
	}, nil
}

func (c *gridSerialConnectionLinux) Info() Info { return c.info }

func (c *gridSerialConnectionLinux) Start(ctx context.Context) error {
	go c.readLoop(ctx)
	return nil
}

func (c *gridSerialConnectionLinux) readLoop(ctx context.Context) {
	buf := make([]byte, 256)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		n, err := c.fd.Read(buf)
		if err != nil {
			if c.closed {
				return
			}
			continue
		}
		if n > 0 {
			c.readBuf = append(c.readBuf, buf[:n]...)
			msgs, remainder := GridParseFrames(c.readBuf)
			c.readBuf = remainder
			_ = msgs
		}
	}
}

func (c *gridSerialConnectionLinux) Events() <-chan Event { return c.events }

func (c *gridSerialConnectionLinux) Feedback() DeviceFeedback {
	return &gridSerialFeedbackLinux{conn: c}
}

func (c *gridSerialConnectionLinux) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	close(c.events)
	return c.fd.Close()
}

func (c *gridSerialConnectionLinux) Alive() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return !c.closed
}

func (c *gridSerialConnectionLinux) writeFrame(frame []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return ErrDeviceDisconnected
	}
	_, err := c.fd.Write(frame)
	return err
}

// ---------------------------------------------------------------------------
// Feedback — LED control via Grid Protocol
// ---------------------------------------------------------------------------

type gridSerialFeedbackLinux struct {
	conn *gridSerialConnectionLinux
}

func (f *gridSerialFeedbackLinux) SetLED(index int, r, g, b, a uint8) error {
	if err := f.conn.writeFrame(GridEncodeLEDColor(index, GridLEDLayer1, r, g, b)); err != nil {
		return err
	}
	return f.conn.writeFrame(GridEncodeLEDValue(index, GridLEDLayer1, a))
}

func (f *gridSerialFeedbackLinux) SetRumble(motor int, intensity float64, duration time.Duration) error {
	return ErrNotSupported
}

func (f *gridSerialFeedbackLinux) SendMIDI(data []byte) error {
	return ErrNotSupported
}

func (f *gridSerialFeedbackLinux) SendRaw(data []byte) error {
	return f.conn.writeFrame(data)
}

// Ensure interface compliance.
var _ DeviceProvider = (*gridSerialProviderLinux)(nil)
var _ DeviceConnection = (*gridSerialConnectionLinux)(nil)
var _ DeviceFeedback = (*gridSerialFeedbackLinux)(nil)
