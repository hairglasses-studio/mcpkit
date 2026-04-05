//go:build darwin

package device

/*
#cgo LDFLAGS: -framework CoreMIDI -framework CoreFoundation

#include <CoreMIDI/CoreMIDI.h>
#include <CoreFoundation/CoreFoundation.h>

// Shared global client — created once.
static MIDIClientRef sharedClient = 0;
static int clientCreated = 0;

static MIDIClientRef getSharedClient(void) {
    if (!clientCreated) {
        CFStringRef name = CFStringCreateWithCString(NULL, "mapitall", kCFStringEncodingUTF8);
        MIDIClientCreate(name, NULL, NULL, &sharedClient);
        CFRelease(name);
        clientCreated = 1;
    }
    return sharedClient;
}

// Pipe-based C→Go bridge: the read proc writes raw MIDI bytes to a pipe fd.
static void midiReadProc(const MIDIPacketList *pktList, void *readProcRefCon, void *srcConnRefCon) {
    int fd = (int)(intptr_t)readProcRefCon;
    const MIDIPacket *pkt = &pktList->packet[0];
    for (UInt32 i = 0; i < pktList->numPackets; i++) {
        write(fd, pkt->data, pkt->length);
        pkt = MIDIPacketNext(pkt);
    }
}

static int createInputPort(MIDIClientRef client, MIDIPortRef *outPort, int pipeFD) {
    CFStringRef name = CFStringCreateWithCString(NULL, "mapitall-in", kCFStringEncodingUTF8);
    OSStatus err = MIDIInputPortCreate(client, name, midiReadProc, (void *)(intptr_t)pipeFD, outPort);
    CFRelease(name);
    return (int)err;
}

static int getMIDISourceCount(void) {
    return (int)MIDIGetNumberOfSources();
}

static MIDIEndpointRef getMIDISource(int index) {
    return MIDIGetSource(index);
}

static int connectPortToSource(MIDIPortRef port, MIDIEndpointRef source) {
    return (int)MIDIPortConnectSource(port, source, NULL);
}

static int disconnectPortFromSource(MIDIPortRef port, MIDIEndpointRef source) {
    return (int)MIDIPortDisconnectSource(port, source);
}

static int getMIDIEndpointName(MIDIEndpointRef endpoint, char *buf, int bufLen) {
    CFStringRef name = NULL;
    MIDIObjectGetStringProperty(endpoint, kMIDIPropertyName, &name);
    if (!name) return 0;
    int ok = CFStringGetCString(name, buf, bufLen, kCFStringEncodingUTF8);
    CFRelease(name);
    return ok;
}
*/
import "C"

import (
	"context"
	"fmt"
	"os"
)

func init() {
	RegisterProvider(func() DeviceProvider { return &darwinMIDIProvider{} })
}

// ---------------------------------------------------------------------------
// CoreMIDI input provider
// ---------------------------------------------------------------------------

type darwinMIDIProvider struct{}

func (p *darwinMIDIProvider) Name() string            { return "coremidi" }
func (p *darwinMIDIProvider) DeviceTypes() []DeviceType { return []DeviceType{TypeMIDI} }

func (p *darwinMIDIProvider) Enumerate(_ context.Context) ([]Info, error) {
	count := int(C.getMIDISourceCount())
	if count == 0 {
		return nil, nil
	}

	var devices []Info
	var nameBuf [256]C.char
	for i := 0; i < count; i++ {
		endpoint := C.getMIDISource(C.int(i))
		name := fmt.Sprintf("MIDI Source %d", i)
		if C.getMIDIEndpointName(endpoint, &nameBuf[0], 256) != 0 {
			name = C.GoString(&nameBuf[0])
		}

		id := DeviceID(fmt.Sprintf("coremidi:%d", i))
		devices = append(devices, Info{
			ID:           id,
			Name:         name,
			Type:         TypeMIDI,
			Connection:   ConnUSB,
			Capabilities: Capabilities{MIDIPorts: 1},
			PlatformPath: fmt.Sprintf("coremidi:%d", i),
			ProviderName: "coremidi",
		})
	}
	return devices, nil
}

func (p *darwinMIDIProvider) Open(_ context.Context, id DeviceID) (DeviceConnection, error) {
	var index int
	if _, err := fmt.Sscanf(string(id), "coremidi:%d", &index); err != nil {
		return nil, fmt.Errorf("%w: invalid CoreMIDI ID: %s", ErrDeviceNotFound, id)
	}

	count := int(C.getMIDISourceCount())
	if index >= count {
		return nil, fmt.Errorf("%w: CoreMIDI source %d not found (have %d)", ErrDeviceNotFound, index, count)
	}

	var nameBuf [256]C.char
	endpoint := C.getMIDISource(C.int(index))
	name := fmt.Sprintf("MIDI Source %d", index)
	if C.getMIDIEndpointName(endpoint, &nameBuf[0], 256) != 0 {
		name = C.GoString(&nameBuf[0])
	}

	return &darwinMIDIConnection{
		deviceInfo: Info{
			ID:           id,
			Name:         name,
			Type:         TypeMIDI,
			ProviderName: "coremidi",
			PlatformPath: string(id),
		},
		sourceIndex: index,
		events:      make(chan Event, 64),
	}, nil
}

func (p *darwinMIDIProvider) Close() error { return nil }

// ---------------------------------------------------------------------------
// CoreMIDI connection — reads via pipe from C callback
// ---------------------------------------------------------------------------

type darwinMIDIConnection struct {
	deviceInfo  Info
	sourceIndex int
	port        C.MIDIPortRef
	endpoint    C.MIDIEndpointRef
	pipeR       *os.File
	pipeW       *os.File
	events      chan Event
	cancel      context.CancelFunc
	alive       bool
}

func (c *darwinMIDIConnection) Info() Info               { return c.deviceInfo }
func (c *darwinMIDIConnection) Events() <-chan Event     { return c.events }
func (c *darwinMIDIConnection) Feedback() DeviceFeedback { return nil }
func (c *darwinMIDIConnection) Alive() bool              { return c.alive }

func (c *darwinMIDIConnection) Start(ctx context.Context) error {
	client := C.getSharedClient()
	if client == 0 {
		return fmt.Errorf("failed to create CoreMIDI client")
	}

	// Create pipe for C callback → Go bridge.
	r, w, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("pipe: %w", err)
	}
	c.pipeR = r
	c.pipeW = w

	// Create input port with the write-end fd as context.
	var port C.MIDIPortRef
	ret := C.createInputPort(client, &port, C.int(w.Fd()))
	if ret != 0 {
		r.Close()
		w.Close()
		return fmt.Errorf("MIDIInputPortCreate failed: OSStatus %d", ret)
	}
	c.port = port

	// Connect to source.
	c.endpoint = C.getMIDISource(C.int(c.sourceIndex))
	ret = C.connectPortToSource(port, c.endpoint)
	if ret != 0 {
		r.Close()
		w.Close()
		return fmt.Errorf("MIDIPortConnectSource failed: OSStatus %d", ret)
	}

	ctx, c.cancel = context.WithCancel(ctx)
	c.alive = true
	go c.readLoop(ctx)
	return nil
}

func (c *darwinMIDIConnection) readLoop(ctx context.Context) {
	defer func() {
		c.alive = false
		close(c.events)
	}()

	buf := make([]byte, 256)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		n, err := c.pipeR.Read(buf)
		if err != nil {
			return
		}

		for _, ev := range parseMIDIBytes(c.deviceInfo.ID, buf, n) {
			c.emit(ctx, ev)
		}
	}
}

func (c *darwinMIDIConnection) emit(ctx context.Context, ev Event) {
	select {
	case c.events <- ev:
	case <-ctx.Done():
	}
}

func (c *darwinMIDIConnection) Close() error {
	if c.cancel != nil {
		c.cancel()
	}
	if c.port != 0 && c.endpoint != 0 {
		C.disconnectPortFromSource(c.port, c.endpoint)
	}
	if c.pipeW != nil {
		c.pipeW.Close()
	}
	if c.pipeR != nil {
		c.pipeR.Close()
	}
	return nil
}
