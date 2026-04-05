//go:build darwin

package device

/*
#cgo LDFLAGS: -framework IOKit -framework CoreFoundation

#include <IOKit/IOKitLib.h>
#include <IOKit/usb/IOUSBLib.h>
#include <IOKit/serial/IOSerialKeys.h>
#include <CoreFoundation/CoreFoundation.h>

// gridSerialEnumerate finds serial ports for Grid devices by matching VID/PID
// in the IOKit registry. Returns the callout device path (e.g., /dev/cu.usbmodem1234561).
static int gridFindSerialPorts(int vid, int pid, char paths[][256], int maxPorts) {
    CFMutableDictionaryRef matchDict = IOServiceMatching(kIOSerialBSDServiceValue);
    if (!matchDict) return 0;
    CFDictionarySetValue(matchDict, CFSTR(kIOSerialBSDTypeKey), CFSTR(kIOSerialBSDModemType));

    io_iterator_t iter;
    kern_return_t kr = IOServiceGetMatchingServices(kIOMainPortDefault, matchDict, &iter);
    if (kr != KERN_SUCCESS) return 0;

    int count = 0;
    io_service_t service;
    while ((service = IOIteratorNext(iter)) && count < maxPorts) {
        // Walk up to the USB device node to check VID/PID.
        io_service_t parent = service;
        io_service_t usbDevice = 0;
        for (int depth = 0; depth < 8; depth++) {
            io_service_t next;
            kr = IORegistryEntryGetParentEntry(parent, kIOServicePlane, &next);
            if (depth > 0) IOObjectRelease(parent);
            if (kr != KERN_SUCCESS) break;
            parent = next;

            CFNumberRef vidRef = IORegistryEntryCreateCFProperty(parent, CFSTR("idVendor"), NULL, 0);
            CFNumberRef pidRef = IORegistryEntryCreateCFProperty(parent, CFSTR("idProduct"), NULL, 0);
            if (vidRef && pidRef) {
                int v = 0, p = 0;
                CFNumberGetValue(vidRef, kCFNumberIntType, &v);
                CFNumberGetValue(pidRef, kCFNumberIntType, &p);
                CFRelease(vidRef);
                CFRelease(pidRef);
                if (v == vid && p == pid) {
                    usbDevice = parent;
                    break;
                }
            }
            if (vidRef) CFRelease(vidRef);
            if (pidRef) CFRelease(pidRef);
        }

        if (usbDevice) {
            CFStringRef pathRef = IORegistryEntryCreateCFProperty(service,
                CFSTR(kIOCalloutDeviceKey), NULL, 0);
            if (pathRef) {
                CFStringGetCString(pathRef, paths[count], 256, kCFStringEncodingUTF8);
                CFRelease(pathRef);
                count++;
            }
            if (usbDevice != service) IOObjectRelease(usbDevice);
        }
        IOObjectRelease(service);
    }
    IOObjectRelease(iter);
    return count;
}
*/
import "C"

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

func init() {
	RegisterProvider(func() DeviceProvider {
		return &gridSerialProvider{}
	})
}

// gridSerialProvider discovers and connects to Grid CDC serial ports.
// Serial connections are output-only (LED feedback) — they don't emit input events.
type gridSerialProvider struct {
	mu    sync.Mutex
	conns map[DeviceID]*gridSerialConnection
}

func (p *gridSerialProvider) Name() string               { return "grid_serial" }
func (p *gridSerialProvider) DeviceTypes() []DeviceType   { return []DeviceType{TypeGenericHID} }

func (p *gridSerialProvider) Enumerate(ctx context.Context) ([]Info, error) {
	var paths [8][256]C.char

	// Search for both Gen2 and Gen1 devices.
	vids := []C.int{C.int(GridVIDGen2), C.int(GridVIDGen1)}
	pids := []C.int{C.int(GridPIDGen2), C.int(GridPIDGen1)}

	var infos []Info
	seen := make(map[string]bool)

	for i := range vids {
		count := C.gridFindSerialPorts(vids[i], pids[i], &paths[0], 8)
		for j := 0; j < int(count); j++ {
			path := C.GoString(&paths[j][0])
			if seen[path] {
				continue
			}
			seen[path] = true

			infos = append(infos, Info{
				ID:           DeviceID("grid_serial:" + path),
				Name:         "Intech Grid CDC device",
				Type:         TypeGenericHID,
				Connection:   ConnUSB,
				VendorID:     uint16(vids[i]),
				ProductID:    uint16(pids[i]),
				Manufacturer: "Intech Studio",
				Capabilities: Capabilities{HasLEDs: true, Encoders: 16},
				PlatformPath: path,
				ProviderName: "grid_serial",
			})
		}
	}

	return infos, nil
}

func (p *gridSerialProvider) Open(ctx context.Context, id DeviceID) (DeviceConnection, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.conns == nil {
		p.conns = make(map[DeviceID]*gridSerialConnection)
	}
	if existing, ok := p.conns[id]; ok {
		return existing, nil
	}

	// Find the device info by enumerating.
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

	conn, err := openGridSerial(info)
	if err != nil {
		return nil, err
	}
	p.conns[id] = conn
	return conn, nil
}

func (p *gridSerialProvider) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, conn := range p.conns {
		conn.Close()
	}
	p.conns = nil
	return nil
}

// ---------------------------------------------------------------------------
// Serial connection
// ---------------------------------------------------------------------------

type gridSerialConnection struct {
	info     Info
	fd       *os.File
	mu       sync.Mutex
	closed   bool
	events   chan Event
	readBuf  []byte
}

func openGridSerial(info Info) (*gridSerialConnection, error) {
	fd, err := os.OpenFile(info.PlatformPath, os.O_RDWR|unix.O_NOCTTY|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, fmt.Errorf("open serial %s: %w", info.PlatformPath, err)
	}

	// Configure raw serial: 115200 baud, 8N1, no flow control.
	rawFD := int(fd.Fd())
	var termios unix.Termios
	if err := unix.IoctlSetTermios(rawFD, unix.TIOCGETA, &termios); err != nil {
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

	if err := unix.IoctlSetTermios(rawFD, unix.TIOCSETA, &termios); err != nil {
		fd.Close()
		return nil, fmt.Errorf("set termios: %w", err)
	}

	// Clear nonblock now that termios is set.
	if err := unix.SetNonblock(rawFD, false); err != nil {
		fd.Close()
		return nil, fmt.Errorf("clear nonblock: %w", err)
	}

	slog.Info("grid serial opened", "path", info.PlatformPath, "device", info.ID)

	return &gridSerialConnection{
		info:    info,
		fd:      fd,
		events:  make(chan Event), // Always empty — serial is output-only.
		readBuf: make([]byte, 0, 512),
	}, nil
}

func (c *gridSerialConnection) Info() Info { return c.info }

// Start begins reading responses from the serial port (for status/heartbeat).
// The events channel is never written to — serial connections are output-only.
func (c *gridSerialConnection) Start(ctx context.Context) error {
	go c.readLoop(ctx)
	return nil
}

func (c *gridSerialConnection) readLoop(ctx context.Context) {
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
			_ = msgs // Log heartbeats if needed; no events emitted.
		}
	}
}

func (c *gridSerialConnection) Events() <-chan Event { return c.events }

func (c *gridSerialConnection) Feedback() DeviceFeedback {
	return &gridSerialFeedback{conn: c}
}

func (c *gridSerialConnection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	close(c.events)
	return c.fd.Close()
}

func (c *gridSerialConnection) Alive() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return !c.closed
}

// writeFrame sends a Grid Protocol frame over serial.
func (c *gridSerialConnection) writeFrame(frame []byte) error {
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

type gridSerialFeedback struct {
	conn *gridSerialConnection
}

func (f *gridSerialFeedback) SetLED(index int, r, g, b, a uint8) error {
	// Set color on LED layer 1, then set value based on alpha.
	if err := f.conn.writeFrame(GridEncodeLEDColor(index, GridLEDLayer1, r, g, b)); err != nil {
		return err
	}
	return f.conn.writeFrame(GridEncodeLEDValue(index, GridLEDLayer1, a))
}

func (f *gridSerialFeedback) SetRumble(motor int, intensity float64, duration time.Duration) error {
	return ErrNotSupported
}

func (f *gridSerialFeedback) SendMIDI(data []byte) error {
	return ErrNotSupported
}

func (f *gridSerialFeedback) SendRaw(data []byte) error {
	return f.conn.writeFrame(data)
}

// Ensure interface compliance.
var _ DeviceProvider = (*gridSerialProvider)(nil)
var _ DeviceConnection = (*gridSerialConnection)(nil)
var _ DeviceFeedback = (*gridSerialFeedback)(nil)

// Suppress unused import warning for unsafe (used by CGO).
var _ = unsafe.Pointer(nil)
