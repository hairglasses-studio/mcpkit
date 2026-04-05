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
*/
import "C"

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unsafe"
)

func init() {
	RegisterProvider(func() DeviceProvider { return &darwinIOKitProvider{} })
}

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

		devices = append(devices, Info{
			ID:           id,
			Name:         name,
			Type:         devType,
			Connection:   ConnUSB,
			VendorID:     vendorID,
			ProductID:    productID,
			Manufacturer: manufacturer,
			Capabilities: Capabilities{
				Buttons: 14,
				Axes:    6,
				Hats:    1,
			},
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

		return &darwinIOKitConnection{
			deviceInfo: Info{
				ID:           id,
				Name:         name,
				Type:         ClassifyDevice(vendorID, productID, name),
				ProviderName: "iokit_hid",
			},
			manager:   manager,
			deviceRef: dev,
			events:    make(chan Event, 64),
		}, nil
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
}

func (c *darwinIOKitConnection) Info() Info               { return c.deviceInfo }
func (c *darwinIOKitConnection) Events() <-chan Event     { return c.events }
func (c *darwinIOKitConnection) Feedback() DeviceFeedback { return nil }
func (c *darwinIOKitConnection) Alive() bool              { return c.alive }

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
	if c.deviceRef != 0 {
		C.IOHIDDeviceClose(c.deviceRef, C.kIOHIDOptionsTypeNone)
	}
	if c.manager != 0 {
		C.IOHIDManagerClose(c.manager, C.kIOHIDOptionsTypeNone)
		C.CFRelease(C.CFTypeRef(c.manager))
	}
	return nil
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
