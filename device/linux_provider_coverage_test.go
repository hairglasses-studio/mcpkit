//go:build linux

package device

import (
	"context"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestLinuxInputProviderEnumerateFromProcFixture(t *testing.T) {
	restoreLinuxSeams(t)

	readProcBusInputDevices = func() ([]byte, error) {
		return []byte(strings.Join([]string{
			`I: Bus=0003 Vendor=045e Product=028e Version=0110`,
			`N: Name="Xbox Wireless Controller"`,
			`H: Handlers=kbd event5 js0`,
			`B: KEY=ffff`,
			`B: ABS=30027`,
			`B: FF=1`,
			``,
			`I: Bus=0003 Vendor=0000 Product=0000 Version=0000`,
			`N: Name="virtual ignored device"`,
			`H: Handlers=event9`,
			`B: KEY=ffff`,
			``,
			`I: Bus=0003 Vendor=9999 Product=9999 Version=0000`,
			`N: Name="Unknown keys only"`,
			`H: Handlers=event10`,
			`B: KEY=ffff`,
			``,
		}, "\n")), nil
	}

	devices, err := (&linuxInputProvider{}).Enumerate(context.Background())
	if err != nil {
		t.Fatalf("Enumerate() error = %v", err)
	}
	if len(devices) != 1 {
		t.Fatalf("Enumerate() returned %d devices, want 1: %+v", len(devices), devices)
	}
	got := devices[0]
	if got.ID != "evdev:/dev/input/event5" {
		t.Fatalf("ID = %q", got.ID)
	}
	if got.Type != TypeGamepad || got.ProviderName != "evdev" || got.PlatformPath != "/dev/input/event5" {
		t.Fatalf("unexpected device info: %+v", got)
	}
	if got.VendorID != 0x045e || got.ProductID != 0x028e {
		t.Fatalf("ids = %04x:%04x", got.VendorID, got.ProductID)
	}
	if got.Capabilities.Buttons != 1 || got.Capabilities.Axes != 1 || !got.Capabilities.HasRumble {
		t.Fatalf("capabilities = %+v", got.Capabilities)
	}

	readProcBusInputDevices = func() ([]byte, error) { return nil, errors.New("no proc") }
	if _, err := (&linuxInputProvider{}).Enumerate(context.Background()); err == nil {
		t.Fatal("Enumerate() succeeded after proc read failure")
	}
}

func TestLinuxEvdevConnectionLifecycleAndFeedback(t *testing.T) {
	restoreLinuxSeams(t)

	evdevIoctlFunc = func(fd uintptr, req uintptr, arg uintptr) error {
		return syscall.ENOTTY
	}

	path := writeInputEvents(t,
		inputEvent{Type: evAbs, Code: 0x00, Val: 100},
		inputEvent{Type: evKey, Code: 28, Val: 1},
	)
	conn := &linuxEvdevConnection{
		deviceInfo: Info{ID: "evdev:test", Capabilities: Capabilities{HasRumble: true}},
		path:       path,
		events:     make(chan Event, 4),
	}

	if err := conn.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if conn.Info().ID != "evdev:test" {
		t.Fatalf("Info() = %+v", conn.Info())
	}
	if conn.Events() == nil {
		t.Fatal("Events() returned nil")
	}
	if conn.Feedback() == nil {
		t.Fatal("Feedback() should be available when HasRumble is true")
	}

	events := drainEvents(t, conn.Events())
	if len(events) != 2 {
		t.Fatalf("readLoop emitted %d events, want 2: %+v", len(events), events)
	}
	if events[0].Type != EventAxis || events[0].Source != "ABS_X" || events[0].Value != 100 {
		t.Fatalf("axis event = %+v", events[0])
	}
	if events[1].Type != EventKey || events[1].Source != "KEY_ENTER" || !events[1].Pressed {
		t.Fatalf("key event = %+v", events[1])
	}
	if err := conn.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	missing := &linuxEvdevConnection{path: filepath.Join(t.TempDir(), "missing"), events: make(chan Event)}
	if err := missing.Start(context.Background()); err == nil {
		t.Fatal("Start() on missing path succeeded")
	}
}

func TestLinuxEvdevConnectionOpenFallbackGrabAndClose(t *testing.T) {
	restoreLinuxSeams(t)

	path := writeInputEvents(t)
	openCalls := 0
	openFile = func(name string, flag int, perm os.FileMode) (*os.File, error) {
		openCalls++
		if flag&os.O_RDWR != 0 {
			return nil, errors.New("read-write denied")
		}
		return os.OpenFile(name, flag, perm)
	}
	conn := &linuxEvdevConnection{path: path, events: make(chan Event)}
	if err := conn.Start(context.Background()); err != nil {
		t.Fatalf("Start() fallback error = %v", err)
	}
	drainEvents(t, conn.Events())
	if openCalls < 2 {
		t.Fatalf("open calls = %d, want read-write fallback to read-only", openCalls)
	}

	fd := mustTempFile(t)
	defer fd.Close()
	evdevIoctlFunc = func(fd uintptr, req uintptr, arg uintptr) error { return nil }
	grabbed := &linuxEvdevConnection{fd: fd}
	if err := grabbed.Grab(); err != nil {
		t.Fatalf("Grab() error = %v", err)
	}
	if !grabbed.grabbed {
		t.Fatal("Grab() did not mark connection grabbed")
	}
	if err := grabbed.ReleaseGrab(); err != nil {
		t.Fatalf("ReleaseGrab() error = %v", err)
	}
	if grabbed.grabbed {
		t.Fatal("ReleaseGrab() did not clear grabbed")
	}
	if err := (&linuxEvdevConnection{}).Grab(); err == nil {
		t.Fatal("Grab() without fd succeeded")
	}
	if err := (&linuxEvdevConnection{}).ReleaseGrab(); err != nil {
		t.Fatalf("ReleaseGrab() without fd = %v", err)
	}

	evdevIoctlFunc = func(fd uintptr, req uintptr, arg uintptr) error { return syscall.ENOTTY }
	grabbed.grabbed = true
	if err := grabbed.ReleaseGrab(); err == nil {
		t.Fatal("ReleaseGrab() with ioctl failure succeeded")
	}
	_ = grabbed.Close()
}

func TestLinuxEvdevFeedbackSetRumble(t *testing.T) {
	restoreLinuxSeams(t)

	fd := mustTempFile(t)
	defer fd.Close()
	var ioctlReqs []uintptr
	evdevIoctlFunc = func(fd uintptr, req uintptr, arg uintptr) error {
		ioctlReqs = append(ioctlReqs, req)
		return nil
	}

	feedback := &linuxEvdevFeedback{fd: fd, effectID: 3}
	if err := feedback.SetRumble(0, 2, time.Nanosecond); err != nil {
		t.Fatalf("SetRumble(strong) error = %v", err)
	}
	feedback.effectID = 3
	if err := feedback.SetRumble(1, -1, 0); err != nil {
		t.Fatalf("SetRumble(weak) error = %v", err)
	}
	if len(ioctlReqs) < 4 {
		t.Fatalf("ioctl requests = %v, want erase/upload for both calls", ioctlReqs)
	}

	evdevIoctlFunc = func(fd uintptr, req uintptr, arg uintptr) error { return syscall.ENOTTY }
	if err := feedback.SetRumble(0, 0.5, time.Millisecond); err == nil {
		t.Fatal("SetRumble() with upload failure succeeded")
	}
}

func TestLinuxMIDIProviderEnumerateAndOpen(t *testing.T) {
	restoreLinuxSeams(t)

	commandOutput = func(name string, args ...string) ([]byte, error) {
		return []byte("Dir Device    Name\nIO  hw:1,0    Main Port\nI   hw:2      Secondary\n"), nil
	}
	devices, err := (&linuxMIDIProvider{}).Enumerate(context.Background())
	if err != nil {
		t.Fatalf("Enumerate() amidi error = %v", err)
	}
	if len(devices) != 2 {
		t.Fatalf("amidi devices = %+v", devices)
	}
	if devices[0].ID != "alsa_midi:hw:1,0" || devices[0].PlatformPath != "/dev/snd/midiC1D0" {
		t.Fatalf("first amidi device = %+v", devices[0])
	}

	commandOutput = func(name string, args ...string) ([]byte, error) { return nil, errors.New("no amidi") }
	midiDeviceGlob = func() ([]string, error) { return []string{"/dev/snd/midiC3D4"}, nil }
	readFile = func(path string) ([]byte, error) {
		if path == "/proc/asound/card3/id" {
			return []byte("GridCard\n"), nil
		}
		return nil, os.ErrNotExist
	}
	devices, err = (&linuxMIDIProvider{}).Enumerate(context.Background())
	if err != nil {
		t.Fatalf("Enumerate() fallback error = %v", err)
	}
	if len(devices) != 1 || devices[0].Name != "GridCard" || devices[0].ID != "alsa_midi:hw:3,4" {
		t.Fatalf("fallback devices = %+v", devices)
	}

	statTarget := mustTempFile(t)
	defer statTarget.Close()
	statFile = func(path string) (os.FileInfo, error) {
		if path != "/dev/snd/midiC3D4" {
			return nil, os.ErrNotExist
		}
		return statTarget.Stat()
	}
	conn, err := (&linuxMIDIProvider{}).Open(context.Background(), "alsa_midi:hw:3,4")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if conn.Info().PlatformPath != "/dev/snd/midiC3D4" || conn.Events() == nil || conn.Feedback() != nil || conn.Alive() {
		t.Fatalf("unexpected unopened MIDI connection state: %+v", conn.Info())
	}

	statFile = func(path string) (os.FileInfo, error) { return nil, os.ErrNotExist }
	if _, err := (&linuxMIDIProvider{}).Open(context.Background(), "alsa_midi:hw:3,4"); err == nil {
		t.Fatal("Open() with missing device succeeded")
	}
	if err := (&linuxMIDIProvider{}).Close(); err != nil {
		t.Fatalf("Close() = %v", err)
	}
}

func TestLinuxMIDIConnectionReadLoopAndFeedback(t *testing.T) {
	restoreLinuxSeams(t)

	path := writeBytes(t, []byte{
		0x40,          // running-status data byte: skipped
		0x90, 60, 127, // note on
		0x80, 60, 0, // note off
		0xB1, 7, 64, // CC
		0xC2, 5, // program change
		0xE3, 0, 64, // pitch bend
		0xF0, 1, 2, 0xF7, // sysex
		0xF8, // ignored system message
	})
	conn := &linuxMIDIConnection{
		deviceInfo: Info{ID: "alsa_midi:hw:1,0", Type: TypeMIDI},
		path:       path,
		events:     make(chan Event, 8),
	}
	if conn.Feedback() != nil {
		t.Fatal("Feedback() before Start should be nil")
	}
	if err := conn.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if conn.Feedback() == nil {
		t.Fatal("Feedback() after Start should be available")
	}

	events := drainEvents(t, conn.Events())
	if conn.Alive() {
		t.Fatal("Alive() after readLoop exit = true")
	}
	wantTypes := []EventType{EventMIDINote, EventMIDINote, EventMIDICC, EventMIDIProgramChange, EventMIDIPitchBend, EventMIDISysEx}
	if len(events) != len(wantTypes) {
		t.Fatalf("events = %+v, want %d events", events, len(wantTypes))
	}
	for i, want := range wantTypes {
		if events[i].Type != want {
			t.Fatalf("event[%d].Type = %v, want %v; events=%+v", i, events[i].Type, want, events)
		}
	}
	if events[0].MIDINote != 60 || !events[0].Pressed || events[2].Controller != 7 || events[5].Source != "midi:sysex" {
		t.Fatalf("unexpected MIDI event details: %+v", events)
	}
	if err := conn.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	fd := mustTempFile(t)
	feedback := &linuxMIDIFeedback{fd: fd}
	if err := feedback.SendMIDI([]byte{0x90, 1, 2}); err != nil {
		t.Fatalf("SendMIDI() error = %v", err)
	}
	_ = fd.Close()
	if err := feedback.SendMIDI([]byte{0x90}); err == nil {
		t.Fatal("SendMIDI() after close succeeded")
	}
}

func TestLinuxHotPlugWatcherRescanAndPollingFallback(t *testing.T) {
	restoreLinuxSeams(t)

	first := Info{ID: "dev1", Name: "Pad 1", Type: TypeGamepad}
	provider := &mockProvider{name: "hotplug", devices: []Info{first}}
	watcher := &LinuxHotPlugWatcher{
		events:    make(chan HotPlugEvent, 4),
		known:     make(map[DeviceID]Info),
		providers: []DeviceProvider{provider},
		netlinkFD: -1,
	}
	watcher.rescan(context.Background())
	assertHotplugEvent(t, watcher.Events(), HotPlugConnect, "dev1")

	provider.devices = nil
	watcher.rescan(context.Background())
	assertHotplugEvent(t, watcher.Events(), HotPlugDisconnect, "dev1")
	if err := watcher.Close(); err != nil {
		t.Fatalf("Close() = %v", err)
	}

	netlinkSocket = func(domain, typ, proto int) (int, error) { return -1, errors.New("no netlink") }
	ctx, cancel := context.WithCancel(context.Background())
	started := NewLinuxHotPlugWatcher()
	started.providers = []DeviceProvider{provider}
	if err := started.Start(ctx); err != nil {
		t.Fatalf("Start() = %v", err)
	}
	cancel()
	if err := started.Close(); err != nil {
		t.Fatalf("Close() after Start = %v", err)
	}
}

func TestLinuxUeventParsing(t *testing.T) {
	relevant := []byte("add@/devices/x\x00ACTION=add\x00SUBSYSTEM=input\x00")
	if !isRelevantUevent(relevant) {
		t.Fatal("expected input add uevent to be relevant")
	}
	if !isRelevantUevent([]byte("remove@/devices/x\x00ACTION=remove\x00SUBSYSTEM=sound")) {
		t.Fatal("expected sound remove uevent without trailing null to be relevant")
	}
	if isRelevantUevent([]byte("change@/devices/x\x00ACTION=change\x00SUBSYSTEM=input\x00")) {
		t.Fatal("change action should not be relevant")
	}
	if isRelevantUevent([]byte("add@/devices/x\x00ACTION=add\x00SUBSYSTEM=block\x00")) {
		t.Fatal("block subsystem should not be relevant")
	}

	parts := splitNullTerminated([]byte("first\x00\x00second\x00third"))
	if strings.Join(parts, ",") != "first,second,third" {
		t.Fatalf("splitNullTerminated() = %#v", parts)
	}
}

func TestGridSerialProviderAndConnection(t *testing.T) {
	restoreLinuxSeams(t)

	classTTY, devPath := makeGridSysfsFixture(t)
	gridSerialTTYGlob = func() ([]string, error) { return []string{classTTY}, nil }
	gridSerialDevPath = func(devName string) string { return devPath }

	provider := &gridSerialProviderLinux{}
	if provider.Name() != "grid_serial" {
		t.Fatalf("Name() = %q", provider.Name())
	}
	if types := provider.DeviceTypes(); len(types) != 1 || types[0] != TypeGenericHID {
		t.Fatalf("DeviceTypes() = %+v", types)
	}
	infos, err := provider.Enumerate(context.Background())
	if err != nil {
		t.Fatalf("Enumerate() error = %v", err)
	}
	if len(infos) != 1 || infos[0].ID != DeviceID("grid_serial:"+devPath) {
		t.Fatalf("grid infos = %+v", infos)
	}
	if vid, pid := readSysfsUSBIDs(classTTY); vid != GridVIDGen2 || pid != GridPIDGen2 {
		t.Fatalf("readSysfsUSBIDs() = %04x:%04x", vid, pid)
	}
	if got := readSysfsFile(filepath.Join(filepath.Dir(classTTY), "missing")); got != "" {
		t.Fatalf("readSysfsFile(missing) = %q", got)
	}
	if vid, pid := readSysfsUSBIDs(filepath.Join(t.TempDir(), "missing")); vid != 0 || pid != 0 {
		t.Fatalf("readSysfsUSBIDs(missing) = %04x:%04x", vid, pid)
	}

	openGridSerial = func(info Info) (*gridSerialConnectionLinux, error) {
		return &gridSerialConnectionLinux{
			info:    info,
			fd:      mustTempFile(t),
			events:  make(chan Event),
			readBuf: make([]byte, 0, 16),
		}, nil
	}
	conn, err := provider.Open(context.Background(), infos[0].ID)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if again, err := provider.Open(context.Background(), infos[0].ID); err != nil || again != conn {
		t.Fatalf("Open() existing = %v, %v; want same connection", again, err)
	}
	if _, err := provider.Open(context.Background(), "grid_serial:/dev/missing"); err == nil {
		t.Fatal("Open() missing device succeeded")
	}
	if err := provider.Close(); err != nil {
		t.Fatalf("provider Close() = %v", err)
	}
}

func TestOpenGridSerialLinuxAndConnectionMethods(t *testing.T) {
	restoreLinuxSeams(t)

	backing := filepath.Join(t.TempDir(), "ttyACM0")
	serialOpenFile = func(path string, flag int, perm os.FileMode) (*os.File, error) {
		return os.OpenFile(backing, os.O_CREATE|os.O_RDWR, 0600)
	}
	ioctlGetTermios = func(fd int, req uint) (*unix.Termios, error) {
		return &unix.Termios{}, nil
	}
	ioctlSetTermios = func(fd int, req uint, termios *unix.Termios) error { return nil }
	setNonblock = func(fd int, nonblocking bool) error { return nil }

	conn, err := openGridSerialLinux(Info{ID: "grid_serial:/dev/ttyACM0", PlatformPath: "/dev/ttyACM0"})
	if err != nil {
		t.Fatalf("openGridSerialLinux() error = %v", err)
	}
	if conn.Info().ID != "grid_serial:/dev/ttyACM0" || conn.Events() == nil || conn.Feedback() == nil || !conn.Alive() {
		t.Fatalf("unexpected connection state: %+v", conn.Info())
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := conn.Start(ctx); err != nil {
		t.Fatalf("Start() = %v", err)
	}
	fb := conn.Feedback()
	if err := fb.SetLED(1, 2, 3, 4, 5); err != nil {
		t.Fatalf("SetLED() = %v", err)
	}
	if err := fb.SendRaw([]byte("raw")); err != nil {
		t.Fatalf("SendRaw() = %v", err)
	}
	if err := fb.SetRumble(0, 1, time.Millisecond); err != ErrNotSupported {
		t.Fatalf("SetRumble() = %v, want ErrNotSupported", err)
	}
	if err := fb.SendMIDI([]byte{0x90}); err != ErrNotSupported {
		t.Fatalf("SendMIDI() = %v, want ErrNotSupported", err)
	}
	if err := conn.Close(); err != nil {
		t.Fatalf("Close() = %v", err)
	}
	if conn.Alive() {
		t.Fatal("Alive() after Close = true")
	}
	if err := conn.writeFrame([]byte("closed")); err != ErrDeviceDisconnected {
		t.Fatalf("writeFrame() after close = %v, want ErrDeviceDisconnected", err)
	}
	if err := conn.Close(); err != nil {
		t.Fatalf("second Close() = %v", err)
	}

	serialOpenFile = func(path string, flag int, perm os.FileMode) (*os.File, error) {
		return nil, errors.New("open failed")
	}
	if _, err := openGridSerialLinux(Info{PlatformPath: "/dev/fail"}); err == nil {
		t.Fatal("openGridSerialLinux() with open failure succeeded")
	}
}

func restoreLinuxSeams(t *testing.T) {
	t.Helper()

	oldReadProc := readProcBusInputDevices
	oldMidiGlob := midiDeviceGlob
	oldReadFile := readFile
	oldStatFile := statFile
	oldCommandOutput := commandOutput
	oldOpenFile := openFile
	oldEvdevIoctl := evdevIoctlFunc
	oldNetlinkSocket := netlinkSocket
	oldNetlinkBind := netlinkBind
	oldNetlinkClose := netlinkClose
	oldNetlinkRecvfrom := netlinkRecvfrom
	oldSetSockoptTimeval := setSockoptTimeval
	oldGridSerialTTYGlob := gridSerialTTYGlob
	oldGridSerialDevPath := gridSerialDevPath
	oldOpenGridSerial := openGridSerial
	oldSerialOpenFile := serialOpenFile
	oldIoctlGetTermios := ioctlGetTermios
	oldIoctlSetTermios := ioctlSetTermios
	oldSetNonblock := setNonblock

	t.Cleanup(func() {
		readProcBusInputDevices = oldReadProc
		midiDeviceGlob = oldMidiGlob
		readFile = oldReadFile
		statFile = oldStatFile
		commandOutput = oldCommandOutput
		openFile = oldOpenFile
		evdevIoctlFunc = oldEvdevIoctl
		netlinkSocket = oldNetlinkSocket
		netlinkBind = oldNetlinkBind
		netlinkClose = oldNetlinkClose
		netlinkRecvfrom = oldNetlinkRecvfrom
		setSockoptTimeval = oldSetSockoptTimeval
		gridSerialTTYGlob = oldGridSerialTTYGlob
		gridSerialDevPath = oldGridSerialDevPath
		openGridSerial = oldOpenGridSerial
		serialOpenFile = oldSerialOpenFile
		ioctlGetTermios = oldIoctlGetTermios
		ioctlSetTermios = oldIoctlSetTermios
		setNonblock = oldSetNonblock
	})
}

func writeInputEvents(t *testing.T, events ...inputEvent) string {
	t.Helper()

	buf := make([]byte, 0, len(events)*inputEventSize)
	for _, ev := range events {
		encoded := make([]byte, inputEventSize)
		binary.LittleEndian.PutUint64(encoded[0:8], uint64(ev.Sec))
		binary.LittleEndian.PutUint64(encoded[8:16], uint64(ev.Usec))
		binary.LittleEndian.PutUint16(encoded[16:18], ev.Type)
		binary.LittleEndian.PutUint16(encoded[18:20], ev.Code)
		binary.LittleEndian.PutUint32(encoded[20:24], uint32(ev.Val))
		buf = append(buf, encoded...)
	}
	return writeBytes(t, buf)
}

func writeBytes(t *testing.T, data []byte) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "device.bin")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

func mustTempFile(t *testing.T) *os.File {
	t.Helper()

	file, err := os.CreateTemp(t.TempDir(), "device-*")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	return file
}

func drainEvents(t *testing.T, ch <-chan Event) []Event {
	t.Helper()

	var events []Event
	timeout := time.After(2 * time.Second)
	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				return events
			}
			events = append(events, ev)
		case <-timeout:
			t.Fatalf("timed out waiting for events; got %+v", events)
		}
	}
}

func assertHotplugEvent(t *testing.T, ch <-chan HotPlugEvent, wantType HotPlugType, wantID DeviceID) {
	t.Helper()

	select {
	case event := <-ch:
		if event.Type != wantType || event.Info.ID != wantID || event.Timestamp.IsZero() {
			t.Fatalf("hotplug event = %+v, want type=%v id=%s", event, wantType, wantID)
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for hotplug event type=%v id=%s", wantType, wantID)
	}
}

func makeGridSysfsFixture(t *testing.T) (classTTY, devPath string) {
	t.Helper()

	root := t.TempDir()
	usbDir := filepath.Join(root, "devices", "pci0000", "1-1")
	target := filepath.Join(usbDir, "tty", "ttyACM0")
	classDir := filepath.Join(root, "class", "tty")
	classTTY = filepath.Join(classDir, "ttyACM0")
	devPath = filepath.Join(root, "dev", "ttyACM0")

	for _, dir := range []string{target, classDir, filepath.Dir(devPath)} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	if err := os.WriteFile(filepath.Join(usbDir, "idVendor"), []byte("303a\n"), 0644); err != nil {
		t.Fatalf("write idVendor: %v", err)
	}
	if err := os.WriteFile(filepath.Join(usbDir, "idProduct"), []byte("8123\n"), 0644); err != nil {
		t.Fatalf("write idProduct: %v", err)
	}
	if err := os.Symlink(target, classTTY); err != nil {
		t.Fatalf("symlink %s -> %s: %v", classTTY, target, err)
	}
	if err := os.WriteFile(devPath, nil, 0600); err != nil {
		t.Fatalf("write dev path: %v", err)
	}
	return classTTY, devPath
}
