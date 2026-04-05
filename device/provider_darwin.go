//go:build darwin

package device

/*
#cgo LDFLAGS: -framework IOKit -framework CoreFoundation

#include <IOKit/hid/IOHIDManager.h>
#include <CoreFoundation/CoreFoundation.h>

// createCFArray wraps CFArrayCreate with standard callbacks.
static CFArrayRef createCFArray(const void **values, CFIndex count) {
    return CFArrayCreate(kCFAllocatorDefault, values, count, &kCFTypeArrayCallBacks);
}

// createMatchDict creates a matching dictionary for HID usage page + usage.
static CFMutableDictionaryRef createHIDMatchDict(int page, int usage) {
    CFMutableDictionaryRef dict = CFDictionaryCreateMutable(
        kCFAllocatorDefault, 2,
        &kCFTypeDictionaryKeyCallBacks,
        &kCFTypeDictionaryValueCallBacks);
    CFNumberRef pageNum = CFNumberCreate(kCFAllocatorDefault, kCFNumberIntType, &page);
    CFNumberRef usageNum = CFNumberCreate(kCFAllocatorDefault, kCFNumberIntType, &usage);
    CFDictionarySetValue(dict, CFSTR(kIOHIDDeviceUsagePageKey), pageNum);
    CFDictionarySetValue(dict, CFSTR(kIOHIDDeviceUsageKey), usageNum);
    CFRelease(pageNum);
    CFRelease(usageNum);
    return dict;
}

// createGamepadMatchArray builds a matching array for joysticks, gamepads, and multi-axis.
static CFArrayRef createGamepadMatchArray(void) {
    CFMutableDictionaryRef d0 = createHIDMatchDict(0x01, 0x04); // Joystick
    CFMutableDictionaryRef d1 = createHIDMatchDict(0x01, 0x05); // Gamepad
    CFMutableDictionaryRef d2 = createHIDMatchDict(0x01, 0x08); // Multi-axis
    const void *dicts[3] = { d0, d1, d2 };
    CFArrayRef arr = CFArrayCreate(kCFAllocatorDefault, dicts, 3, &kCFTypeArrayCallBacks);
    CFRelease(d0);
    CFRelease(d1);
    CFRelease(d2);
    return arr;
}

// getIntProp extracts an integer property from a HID device using a C string key.
static int64_t getIntProp(IOHIDDeviceRef dev, const char *key) {
    CFStringRef cfkey = CFStringCreateWithCString(NULL, key, kCFStringEncodingUTF8);
    CFTypeRef ref = IOHIDDeviceGetProperty(dev, cfkey);
    CFRelease(cfkey);
    if (!ref || CFGetTypeID(ref) != CFNumberGetTypeID()) return 0;
    int64_t val = 0;
    CFNumberGetValue((CFNumberRef)ref, kCFNumberSInt64Type, &val);
    return val;
}

// getStrProp extracts a string property into a C buffer using a C string key.
static int getStrProp(IOHIDDeviceRef dev, const char *key, char *buf, int bufLen) {
    CFStringRef cfkey = CFStringCreateWithCString(NULL, key, kCFStringEncodingUTF8);
    CFTypeRef ref = IOHIDDeviceGetProperty(dev, cfkey);
    CFRelease(cfkey);
    if (!ref || CFGetTypeID(ref) != CFStringGetTypeID()) return 0;
    return CFStringGetCString((CFStringRef)ref, buf, bufLen, kCFStringEncodingUTF8);
}

// Convenience wrappers for common properties.
static int64_t getVendorID(IOHIDDeviceRef dev)  { return getIntProp(dev, "VendorID"); }
static int64_t getProductID(IOHIDDeviceRef dev) { return getIntProp(dev, "ProductID"); }
static int64_t getLocationID(IOHIDDeviceRef dev) { return getIntProp(dev, "LocationID"); }
static int getProductName(IOHIDDeviceRef dev, char *buf, int len) { return getStrProp(dev, "Product", buf, len); }
static int getManufacturer(IOHIDDeviceRef dev, char *buf, int len) { return getStrProp(dev, "Manufacturer", buf, len); }

// pollElementValue reads current value of a HID element.
static int32_t pollElementValue(IOHIDDeviceRef dev, IOHIDElementRef elem) {
    IOHIDValueRef val = NULL;
    if (IOHIDDeviceGetValue(dev, elem, &val) != kIOReturnSuccess || !val) return 0;
    return (int32_t)IOHIDValueGetIntegerValue(val);
}

// hidSetReport sends a HID report (feature or output) to a device.
// reportType: kIOHIDReportTypeFeature (2) or kIOHIDReportTypeOutput (1).
static int hidSetReport(IOHIDDeviceRef dev, int reportType, int reportID,
                        const uint8_t *data, int dataLen) {
    IOReturn ret = IOHIDDeviceSetReport(dev, (IOHIDReportType)reportType,
                                        (CFIndex)reportID, data, (CFIndex)dataLen);
    return (int)ret;
}
*/
import "C"

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"strings"
	"sync"
	"time"
	"unsafe"
)

func init() {
	RegisterProvider(func() DeviceProvider { return &darwinIOKitProvider{} })
}

// Compile-time interface checks.
var _ Grabbable = (*darwinIOKitConnection)(nil)

// ---------------------------------------------------------------------------
// IOKit HID provider — gamepads, generic HID via IOKit HID Manager
// ---------------------------------------------------------------------------

// HID usage page / usage constants.
const (
	hidPageGenericDesktop = 0x01
	hidPageButton         = 0x09
	hidUsageJoystick      = 0x04
	hidUsageGamepad       = 0x05
	hidUsageMultiAxis     = 0x08
	hidUsageX             = 0x30
	hidUsageY             = 0x31
	hidUsageZ             = 0x32
	hidUsageRX            = 0x33
	hidUsageRY            = 0x34
	hidUsageRZ            = 0x35
	hidUsageHatSwitch     = 0x39
)

// Axis usage → canonical source name.
var hidAxisSourceName = map[uint16]string{
	hidUsageX:  "ABS_X",
	hidUsageY:  "ABS_Y",
	hidUsageZ:  "ABS_Z",
	hidUsageRX: "ABS_RX",
	hidUsageRY: "ABS_RY",
	hidUsageRZ: "ABS_RZ",
}

// Button HID usage → canonical source name (usage page 0x09).
var hidButtonSourceName = map[uint16]string{
	1:  "BTN_SOUTH",
	2:  "BTN_EAST",
	3:  "BTN_WEST",
	4:  "BTN_NORTH",
	5:  "BTN_TL",
	6:  "BTN_TR",
	7:  "BTN_TL2",
	8:  "BTN_TR2",
	9:  "BTN_SELECT",
	10: "BTN_START",
	11: "BTN_THUMBL",
	12: "BTN_THUMBR",
	13: "BTN_MODE",
}

type darwinIOKitProvider struct{}

func (p *darwinIOKitProvider) Name() string { return "iokit_hid" }

func (p *darwinIOKitProvider) DeviceTypes() []DeviceType {
	return []DeviceType{TypeGamepad, TypeGenericHID, TypeHOTAS, TypeRacingWheel}
}

func (p *darwinIOKitProvider) Enumerate(_ context.Context) ([]Info, error) {
	manager := C.IOHIDManagerCreate(C.kCFAllocatorDefault, C.kIOHIDOptionsTypeNone)
	if manager == 0 {
		return nil, fmt.Errorf("IOHIDManagerCreate failed")
	}
	defer C.CFRelease(C.CFTypeRef(manager))

	// Match joysticks, gamepads, and multi-axis controllers.
	matchDicts := createMatchingDicts()
	C.IOHIDManagerSetDeviceMatchingMultiple(manager, matchDicts)
	C.CFRelease(C.CFTypeRef(matchDicts))

	C.IOHIDManagerOpen(manager, C.kIOHIDOptionsTypeNone)
	defer C.IOHIDManagerClose(manager, C.kIOHIDOptionsTypeNone)

	deviceSet := C.IOHIDManagerCopyDevices(manager)
	if deviceSet == 0 {
		return nil, nil
	}
	defer C.CFRelease(C.CFTypeRef(deviceSet))

	count := C.CFSetGetCount(deviceSet)
	if count == 0 {
		return nil, nil
	}

	ptrs := make([]unsafe.Pointer, count)
	C.CFSetGetValues(deviceSet, (*unsafe.Pointer)(unsafe.Pointer(&ptrs[0])))

	var devices []Info
	var nameBuf [256]C.char
	for _, ptr := range ptrs {
		dev := C.IOHIDDeviceRef(ptr)

		vendorID := uint16(C.getVendorID(dev))
		productID := uint16(C.getProductID(dev))

		name := "Unknown HID Device"
		if C.getProductName(dev, &nameBuf[0], 256) != 0 {
			name = C.GoString(&nameBuf[0])
		}

		devType := ClassifyDevice(vendorID, productID, name)
		if devType == TypeUnknown {
			devType = TypeGamepad // IOKit matching already filters to gamepad-like
		}

		// Build a unique ID from vendor:product:location.
		locationID := int64(C.getLocationID(dev))
		id := DeviceID(fmt.Sprintf("iokit_hid:%04x:%04x:%x", vendorID, productID, locationID))

		manufacturer := ""
		if C.getManufacturer(dev, &nameBuf[0], 256) != 0 {
			manufacturer = C.GoString(&nameBuf[0])
		}

		caps := Capabilities{
			Buttons: 14,
			Axes:    6,
			Hats:    1,
		}
		// Elgato Stream Deck devices support LED output via HID feature reports.
		if vendorID == elgatoVendorID {
			if sd, ok := streamDeckModels[productID]; ok {
				caps.HasLEDs = true
				caps.Buttons = sd.keys
			}
		}

		devices = append(devices, Info{
			ID:           id,
			Name:         name,
			Type:         devType,
			Connection:   ConnUSB,
			VendorID:     vendorID,
			ProductID:    productID,
			Manufacturer: manufacturer,
			Capabilities: caps,
			PlatformPath: fmt.Sprintf("iokit:%x", locationID),
			ProviderName: "iokit_hid",
		})
	}

	return devices, nil
}

func createMatchingDicts() C.CFArrayRef {
	return C.createGamepadMatchArray()
}

func (p *darwinIOKitProvider) Open(_ context.Context, id DeviceID) (DeviceConnection, error) {
	// Re-enumerate and find the matching device.
	manager := C.IOHIDManagerCreate(C.kCFAllocatorDefault, C.kIOHIDOptionsTypeNone)
	if manager == 0 {
		return nil, fmt.Errorf("IOHIDManagerCreate failed")
	}

	matchDicts := createMatchingDicts()
	C.IOHIDManagerSetDeviceMatchingMultiple(manager, matchDicts)
	C.CFRelease(C.CFTypeRef(matchDicts))
	C.IOHIDManagerOpen(manager, C.kIOHIDOptionsTypeNone)

	deviceSet := C.IOHIDManagerCopyDevices(manager)
	if deviceSet == 0 {
		C.IOHIDManagerClose(manager, C.kIOHIDOptionsTypeNone)
		C.CFRelease(C.CFTypeRef(manager))
		return nil, fmt.Errorf("%w: no HID devices found", ErrDeviceNotFound)
	}

	count := C.CFSetGetCount(deviceSet)
	ptrs := make([]unsafe.Pointer, count)
	C.CFSetGetValues(deviceSet, (*unsafe.Pointer)(unsafe.Pointer(&ptrs[0])))

	var nameBuf [256]C.char
	for _, ptr := range ptrs {
		dev := C.IOHIDDeviceRef(ptr)
		vendorID := uint16(C.getVendorID(dev))
		productID := uint16(C.getProductID(dev))
		locationID := int64(C.getLocationID(dev))
		candidateID := DeviceID(fmt.Sprintf("iokit_hid:%04x:%04x:%x", vendorID, productID, locationID))

		if candidateID != id {
			continue
		}

		name := "Unknown HID Device"
		if C.getProductName(dev, &nameBuf[0], 256) != 0 {
			name = C.GoString(&nameBuf[0])
		}

		C.CFRelease(C.CFTypeRef(deviceSet))
		// manager kept alive — owned by connection

		conn := &darwinIOKitConnection{
			deviceInfo: Info{
				ID:           id,
				Name:         name,
				Type:         ClassifyDevice(vendorID, productID, name),
				ProviderName: "iokit_hid",
			},
			manager:   manager,
			deviceRef: dev,
			events:    make(chan Event, 64),
		}

		// Create LED feedback for Stream Deck devices.
		if vendorID == elgatoVendorID {
			if sd, ok := streamDeckModels[productID]; ok {
				conn.feedback = &darwinIOKitFeedback{
					deviceRef: dev,
					model:     sd,
				}
			}
		}

		return conn, nil
	}

	C.CFRelease(C.CFTypeRef(deviceSet))
	C.IOHIDManagerClose(manager, C.kIOHIDOptionsTypeNone)
	C.CFRelease(C.CFTypeRef(manager))
	return nil, fmt.Errorf("%w: device %s not found", ErrDeviceNotFound, id)
}

func (p *darwinIOKitProvider) Close() error { return nil }

// ---------------------------------------------------------------------------
// IOKit HID connection — polls element values at 200Hz
// ---------------------------------------------------------------------------

type elementMeta struct {
	ref     C.IOHIDElementRef
	page    uint16
	usage   uint16
	logMin  int32
	logMax  int32
	lastVal int32
}

type darwinIOKitConnection struct {
	deviceInfo Info
	manager    C.IOHIDManagerRef
	deviceRef  C.IOHIDDeviceRef
	elements   []elementMeta
	events     chan Event
	cancel     context.CancelFunc
	alive      bool
	grabbed    bool
	feedback   *darwinIOKitFeedback
}

func (c *darwinIOKitConnection) Info() Info           { return c.deviceInfo }
func (c *darwinIOKitConnection) Events() <-chan Event { return c.events }
func (c *darwinIOKitConnection) Alive() bool          { return c.alive }

// Feedback returns a DeviceFeedback for IOKit HID devices that support LED
// output (e.g., Elgato Stream Deck), or nil for devices without LED support.
func (c *darwinIOKitConnection) Feedback() DeviceFeedback {
	if c.feedback == nil {
		return nil
	}
	return c.feedback
}

func (c *darwinIOKitConnection) Start(ctx context.Context) error {
	ret := C.IOHIDDeviceOpen(c.deviceRef, C.kIOHIDOptionsTypeNone)
	if ret != C.kIOReturnSuccess {
		return fmt.Errorf("IOHIDDeviceOpen failed: 0x%x", ret)
	}

	// Cache all elements with their metadata.
	cfElements := C.IOHIDDeviceCopyMatchingElements(c.deviceRef, 0, C.kIOHIDOptionsTypeNone)
	if cfElements != 0 {
		count := C.CFArrayGetCount(cfElements)
		for i := C.CFIndex(0); i < count; i++ {
			elem := C.IOHIDElementRef(C.CFArrayGetValueAtIndex(cfElements, i))
			page := uint16(C.IOHIDElementGetUsagePage(elem))
			usage := uint16(C.IOHIDElementGetUsage(elem))

			// Only track buttons and generic desktop axes.
			if page != hidPageButton && page != hidPageGenericDesktop {
				continue
			}
			if page == hidPageGenericDesktop && usage < hidUsageX && usage != hidUsageHatSwitch {
				continue
			}

			c.elements = append(c.elements, elementMeta{
				ref:    elem,
				page:   page,
				usage:  usage,
				logMin: int32(C.IOHIDElementGetLogicalMin(elem)),
				logMax: int32(C.IOHIDElementGetLogicalMax(elem)),
			})
		}
		C.CFRelease(C.CFTypeRef(cfElements))
	}

	ctx, c.cancel = context.WithCancel(ctx)
	c.alive = true
	go c.readLoop(ctx)
	return nil
}

func (c *darwinIOKitConnection) readLoop(ctx context.Context) {
	defer func() {
		c.alive = false
		close(c.events)
	}()

	ticker := time.NewTicker(5 * time.Millisecond) // 200Hz
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		now := time.Now()
		for i := range c.elements {
			elem := &c.elements[i]
			val := int32(C.pollElementValue(c.deviceRef, elem.ref))
			if val == elem.lastVal {
				continue
			}
			oldVal := elem.lastVal
			elem.lastVal = val
			_ = oldVal

			ev := c.convertElement(elem, val, now)
			if ev != nil {
				select {
				case c.events <- *ev:
				case <-ctx.Done():
					return
				}
			}
		}
	}
}

func (c *darwinIOKitConnection) convertElement(elem *elementMeta, val int32, now time.Time) *Event {
	switch elem.page {
	case hidPageButton:
		source, ok := hidButtonSourceName[elem.usage]
		if !ok {
			source = fmt.Sprintf("BTN_%d", elem.usage)
		}
		pressed := val != 0
		v := 0.0
		if pressed {
			v = 1.0
		}
		return &Event{
			DeviceID:  c.deviceInfo.ID,
			Type:      EventButton,
			Timestamp: now,
			Source:    source,
			Pressed:   pressed,
			Value:     v,
			RawValue:  val,
		}

	case hidPageGenericDesktop:
		if elem.usage == hidUsageHatSwitch {
			return c.convertHat(elem, val, now)
		}
		source, ok := hidAxisSourceName[elem.usage]
		if !ok {
			return nil
		}
		normalized := normalizeHIDAxis(val, elem.logMin, elem.logMax)
		return &Event{
			DeviceID:  c.deviceInfo.ID,
			Type:      EventAxis,
			Timestamp: now,
			Source:    source,
			Value:     normalized,
			RawValue:  val,
		}
	}
	return nil
}

func (c *darwinIOKitConnection) convertHat(elem *elementMeta, val int32, now time.Time) *Event {
	// Standard 8-position hat: 0=N, 1=NE, 2=E, ..., 7=NW, 8+=centered
	var hatX, hatY int8
	switch {
	case val == 0: // N
		hatY = -1
	case val == 1: // NE
		hatX, hatY = 1, -1
	case val == 2: // E
		hatX = 1
	case val == 3: // SE
		hatX, hatY = 1, 1
	case val == 4: // S
		hatY = 1
	case val == 5: // SW
		hatX, hatY = -1, 1
	case val == 6: // W
		hatX = -1
	case val == 7: // NW
		hatX, hatY = -1, -1
	default: // centered
	}

	return &Event{
		DeviceID:  c.deviceInfo.ID,
		Type:      EventHat,
		Timestamp: now,
		Source:    "ABS_HAT0X",
		HatX:     hatX,
		HatY:     hatY,
		RawValue:  val,
	}
}

func normalizeHIDAxis(val, logMin, logMax int32) float64 {
	if logMax == logMin {
		return float64(val)
	}
	min := float64(logMin)
	max := float64(logMax)
	v := float64(val)
	if logMin >= 0 {
		// Unsigned (trigger): 0.0 to 1.0
		return (v - min) / (max - min)
	}
	// Signed (stick): -1.0 to 1.0
	mid := (max + min) / 2
	halfRange := (max - min) / 2
	return (v - mid) / halfRange
}

func (c *darwinIOKitConnection) Close() error {
	if c.cancel != nil {
		c.cancel()
	}
	if c.grabbed {
		c.ReleaseGrab()
	}
	if c.deviceRef != 0 {
		C.IOHIDDeviceClose(c.deviceRef, C.kIOHIDOptionsTypeNone)
	}
	if c.manager != 0 {
		C.IOHIDManagerClose(c.manager, C.kIOHIDOptionsTypeNone)
		C.CFRelease(C.CFTypeRef(c.manager))
	}
	return nil
}

// Grab claims exclusive access to this HID device via IOHIDDeviceOpen with
// kIOHIDOptionsTypeSeizeDevice. While grabbed, the kernel stops forwarding
// events from this device to other clients (e.g., the window server).
func (c *darwinIOKitConnection) Grab() error {
	if c.deviceRef == 0 {
		return fmt.Errorf("device not open")
	}
	if c.grabbed {
		return nil
	}

	// Close the device handle, then reopen with the seize flag.
	C.IOHIDDeviceClose(c.deviceRef, C.kIOHIDOptionsTypeNone)

	const kIOHIDOptionsTypeSeizeDevice = C.IOOptionBits(0x01)
	ret := C.IOHIDDeviceOpen(c.deviceRef, kIOHIDOptionsTypeSeizeDevice)
	if ret != C.kIOReturnSuccess {
		// Attempt to reopen without seize so the connection remains usable.
		C.IOHIDDeviceOpen(c.deviceRef, C.kIOHIDOptionsTypeNone)
		return fmt.Errorf("IOHIDDeviceOpen(seize) failed: 0x%x", ret)
	}
	c.grabbed = true
	return nil
}

// ReleaseGrab releases exclusive access by reopening the device with
// kIOHIDOptionsTypeNone, allowing other clients to receive events again.
func (c *darwinIOKitConnection) ReleaseGrab() error {
	if c.deviceRef == 0 || !c.grabbed {
		return nil
	}

	C.IOHIDDeviceClose(c.deviceRef, C.kIOHIDOptionsTypeNone)

	ret := C.IOHIDDeviceOpen(c.deviceRef, C.kIOHIDOptionsTypeNone)
	if ret != C.kIOReturnSuccess {
		c.grabbed = false
		return fmt.Errorf("IOHIDDeviceOpen(release) failed: 0x%x", ret)
	}
	c.grabbed = false
	return nil
}

// ---------------------------------------------------------------------------
// Elgato Stream Deck constants and model database
// ---------------------------------------------------------------------------

const elgatoVendorID = 0x0fd9

// streamDeckModel describes a Stream Deck variant's protocol parameters.
type streamDeckModel struct {
	keys      int // Number of keys
	cols      int // Key grid columns
	rows      int // Key grid rows
	iconW     int // Key icon width in pixels
	iconH     int // Key icon height in pixels
	imgReport int // Report ID for image data
	pageSize  int // Max payload per HID report page
	headerLen int // Header length within each page
}

// streamDeckModels maps product IDs to Stream Deck protocol parameters.
// All v2+ models use report ID 0x02 for images with JPEG data.
var streamDeckModels = map[uint16]streamDeckModel{
	0x0060: {keys: 15, cols: 5, rows: 3, iconW: 72, iconH: 72, imgReport: 0x02, pageSize: 1024, headerLen: 8},     // Original v1 (JPEG on v2 firmware)
	0x006d: {keys: 6, cols: 3, rows: 2, iconW: 80, iconH: 80, imgReport: 0x02, pageSize: 1024, headerLen: 8},      // Mini
	0x0063: {keys: 32, cols: 8, rows: 4, iconW: 96, iconH: 96, imgReport: 0x02, pageSize: 1024, headerLen: 8},     // XL
	0x0080: {keys: 15, cols: 5, rows: 3, iconW: 72, iconH: 72, imgReport: 0x02, pageSize: 1024, headerLen: 8},     // MK.2
	0x0084: {keys: 3, cols: 3, rows: 1, iconW: 0, iconH: 0, imgReport: 0x02, pageSize: 1024, headerLen: 8},        // Pedal (no display)
	0x0086: {keys: 8, cols: 4, rows: 2, iconW: 120, iconH: 120, imgReport: 0x02, pageSize: 1024, headerLen: 8},    // Plus
	0x008f: {keys: 8, cols: 4, rows: 2, iconW: 72, iconH: 72, imgReport: 0x02, pageSize: 1024, headerLen: 8},      // Neo
}

// ---------------------------------------------------------------------------
// IOKit HID feedback — LED control for Stream Deck via IOHIDDeviceSetReport
// ---------------------------------------------------------------------------

// darwinIOKitFeedback implements DeviceFeedback for macOS IOKit HID devices.
// It supports SetLED for Elgato Stream Deck key image updates and SendRaw
// for pass-through HID report writes.
type darwinIOKitFeedback struct {
	deviceRef C.IOHIDDeviceRef
	model     streamDeckModel
	mu        sync.Mutex
}

// SetLED sets a Stream Deck key to a solid RGB color by generating a JPEG
// image and sending it via IOHIDDeviceSetReport. The index parameter selects
// the key (0-based). The alpha channel is ignored (Stream Deck keys are opaque).
func (f *darwinIOKitFeedback) SetLED(index int, r, g, b, a uint8) error {
	if index < 0 || index >= f.model.keys {
		return fmt.Errorf("%w: key index %d out of range (0-%d)", ErrNotSupported, index, f.model.keys-1)
	}

	// Stream Deck Pedal has no display — only physical buttons.
	if f.model.iconW == 0 || f.model.iconH == 0 {
		return fmt.Errorf("%w: device has no display keys", ErrNotSupported)
	}

	// Generate a solid-color JPEG at the key's native resolution.
	jpegData, err := solidColorJPEG(f.model.iconW, f.model.iconH, r, g, b)
	if err != nil {
		return fmt.Errorf("generate key image: %w", err)
	}

	return f.sendKeyImage(index, jpegData)
}

// sendKeyImage sends JPEG data for a single key using the Stream Deck v2
// paged image protocol. Data is split across multiple HID feature reports,
// each with a header containing page number, key index, and final-page flag.
func (f *darwinIOKitFeedback) sendKeyImage(keyIndex int, jpegData []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	payloadSize := f.model.pageSize - f.model.headerLen
	if payloadSize <= 0 {
		return fmt.Errorf("invalid Stream Deck model config: pageSize %d <= headerLen %d", f.model.pageSize, f.model.headerLen)
	}

	remaining := len(jpegData)
	offset := 0
	page := 0

	for remaining > 0 || page == 0 {
		chunk := payloadSize
		if chunk > remaining {
			chunk = remaining
		}
		isLast := remaining <= payloadSize

		report := make([]byte, f.model.pageSize)
		// Stream Deck v2 image report header (8 bytes):
		//   [0] = report ID (0x02)
		//   [1] = 0x07 (set key image command)
		//   [2] = key index
		//   [3] = is_last (1 if final page, 0 otherwise)
		//   [4-5] = payload length (little-endian)
		//   [6-7] = page number (little-endian)
		report[0] = byte(f.model.imgReport)
		report[1] = 0x07
		report[2] = byte(keyIndex)
		if isLast {
			report[3] = 1
		}
		report[4] = byte(chunk & 0xFF)
		report[5] = byte((chunk >> 8) & 0xFF)
		report[6] = byte(page & 0xFF)
		report[7] = byte((page >> 8) & 0xFF)

		if chunk > 0 {
			copy(report[f.model.headerLen:], jpegData[offset:offset+chunk])
		}

		ret := C.hidSetReport(
			f.deviceRef,
			C.int(2), // kIOHIDReportTypeFeature
			C.int(f.model.imgReport),
			(*C.uint8_t)(unsafe.Pointer(&report[0])),
			C.int(len(report)),
		)
		if ret != 0 {
			return fmt.Errorf("IOHIDDeviceSetReport failed: IOReturn 0x%x (page %d, key %d)", ret, page, keyIndex)
		}

		offset += chunk
		remaining -= chunk
		page++
	}

	return nil
}

// SetRumble is not supported on Stream Deck devices.
func (f *darwinIOKitFeedback) SetRumble(int, float64, time.Duration) error {
	return ErrNotSupported
}

// SendMIDI is not supported on Stream Deck devices.
func (f *darwinIOKitFeedback) SendMIDI([]byte) error {
	return ErrNotSupported
}

// SendRaw sends a raw HID feature report to the device. The caller is
// responsible for constructing the correct report format.
func (f *darwinIOKitFeedback) SendRaw(data []byte) error {
	if len(data) == 0 {
		return nil
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	reportID := int(data[0])
	ret := C.hidSetReport(
		f.deviceRef,
		C.int(2), // kIOHIDReportTypeFeature
		C.int(reportID),
		(*C.uint8_t)(unsafe.Pointer(&data[0])),
		C.int(len(data)),
	)
	if ret != 0 {
		return fmt.Errorf("IOHIDDeviceSetReport failed: IOReturn 0x%x", ret)
	}
	return nil
}

// solidColorJPEG generates a JPEG image of the given dimensions filled with
// a solid RGB color. It returns the compressed JPEG bytes.
func solidColorJPEG(w, h int, r, g, b uint8) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	c := color.RGBA{R: r, G: g, B: b, A: 255}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// containsAnyLower checks if any substring is present in the lowered string.
// The device package already has containsAny in classify.go which is used here.
func isVirtualDevice(name string) bool {
	lower := strings.ToLower(name)
	return containsAny(lower, "virtual", "ydotool")
}
